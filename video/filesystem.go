package video

import (
	"encoding/json"
	"github.com/pillash/mp4util"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	ExtVideo  = "_video.mp4"
	ExtThumb  = "_thumb.jpg"
	ExtVThumb = "_vthumb.mp4"

	// FileTimeLayout defines the format of filenames.
	// See https://golang.org/src/time/format.go.
	FileTimeLayout = "20060102-150405Z0700"

	// CheckpointFile is the name of the serialized filesystem representation containing record metadata.
	CheckpointFile = "metadata.json"
)

var (
	FilesystemRefreshInterval = time.Minute
	GarbageCollectionInterval = 5 * time.Minute
)

type FilesystemListener interface {
	FilesystemUpdated()
}

type VideoRecord struct {
	Time time.Time
	ID   string

	// TODO: make these relative?
	VideoPath  string
	ThumbPath  string
	VThumbPath string

	// Length of the video file.
	VideoDuration time.Duration

	// Combined size of this record on disk.
	Size int64

	// Reference to parent.
	fs *Filesystem
}

// TODO json version for frontend.

type FilesystemOptions struct {
	BasePath string

	// MaxSize defines the total size threshold for garbage collection of old
	// events on disk. Note that this amount may be exceeded slightly for a short
	// time since garbage collection is periodic. Default value disables GC on
	// size.
	MaxSize int64

	// MaxAge defines the age threshold for garbage collection of old events on
	// disk. Default value disables GC on size.
	MaxAge time.Duration
}

type Filesystem struct {
	Records map[string]*VideoRecord

	options FilesystemOptions

	videoDurationCache map[string]time.Duration

	listeners []FilesystemListener

	refreshc chan bool
	l        sync.Mutex
}

func NewFilesystem(opts FilesystemOptions) (*Filesystem, error) {
	if err := os.MkdirAll(opts.BasePath, 0755); err != nil {
		return nil, err
	}
	f := &Filesystem{
		options:            opts,
		refreshc:           make(chan bool, 1),
		videoDurationCache: make(map[string]time.Duration),
	}
	go func() {
		log.Infof("Starting initial filesystem refresh. This could take a while.")
		f.doRefresh() // Initial filesystem scan.
		rt := time.NewTicker(FilesystemRefreshInterval)
		gt := time.NewTicker(GarbageCollectionInterval)
		for {
			select {
			case <-rt.C:
				f.Refresh()
			case <-gt.C:
				f.doGarbageCollect()
			case <-f.refreshc:
				f.doRefresh()
			}
		}
	}()
	return f, nil
}

func (f *Filesystem) NewRecord(t time.Time) *VideoRecord {
	id := t.Format(FileTimeLayout)
	base := filepath.Join(f.options.BasePath, id)
	return &VideoRecord{
		Time: t,
		ID:   id,

		VideoPath:  base + ExtVideo,
		ThumbPath:  base + ExtThumb,
		VThumbPath: base + ExtVThumb,

		fs: f,
	}
}

func (f *Filesystem) saveCheckpoint() error {
	p := filepath.Join(f.options.BasePath, CheckpointFile)
	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	if err := ioutil.WriteFile(p, b, 0660); err != nil {
		return err
	}
	return nil
}

// TODO implement checkpoint restore.

func (f *Filesystem) doRefresh() error {
	refreshStart := time.Now()
	m := make(map[string]*VideoRecord)

	files, err := ioutil.ReadDir(f.options.BasePath)
	if err != nil {
		return err
	}

	for _, file := range files {
		b := file.Name()
		if len(b) < len(FileTimeLayout) {
			continue
		}
		id := b[:len(FileTimeLayout)]
		t, err := time.Parse(FileTimeLayout, id)
		if err != nil {
			continue
		}

		v := m[id]
		if v == nil {
			v = &VideoRecord{
				Time: t,
				ID:   id,

				fs: f,
			}
		}

		p := filepath.Join(f.options.BasePath, b)
		switch {
		case strings.HasSuffix(b, ExtVideo):
			v.VideoPath = p
			d, err := f.lookupVideoDuration(p)
			if err != nil {
				log.Errorf("Failed to get duration for %v: %v", p, err)
			} else {
				v.VideoDuration = d
			}
		case strings.HasSuffix(b, ExtThumb):
			v.ThumbPath = p
		case strings.HasSuffix(b, ExtVThumb):
			v.VThumbPath = p
		default:
			continue
		}

		v.Size += file.Size()

		m[id] = v
	}

	et := time.Since(refreshStart)
	if et < time.Second {
		log.Debugf("Filesystem refresh completed in %v", et)
	} else {
		log.Infof("Filesystem refresh (slow) completed in %v", et)
	}

	f.l.Lock()
	defer f.l.Unlock()

	// TODO: equal might need to change if additional metadata is stored in records...

	equal := reflect.DeepEqual(f.Records, m)
	f.Records = m

	if !equal {
		go func() {
			for _, listener := range f.listeners {
				listener.FilesystemUpdated()
			}
		}()

		if err := f.saveCheckpoint(); err != nil {
			log.Errorf("Failed to save filesystem checkpoint: %v", err)
		}
	}
	return nil
}

// lookupVideoDuration implements cached lookup of the duration of the mp4 file at `path`.
func (f *Filesystem) lookupVideoDuration(path string) (time.Duration, error) {
	if d, ok := f.videoDurationCache[path]; ok {
		return d, nil
	}
	ds, err := mp4util.Duration(path)
	if err != nil {
		return time.Duration(0), err
	}
	d := time.Second * time.Duration(ds)
	f.videoDurationCache[path] = d
	return d, nil
}

func (f *Filesystem) AddListener(l FilesystemListener) {
	f.l.Lock()
	defer f.l.Unlock()
	f.listeners = append(f.listeners, l)
}

func (f *Filesystem) doGarbageCollect() {
	gcStart := time.Now()

	var total int64
	var cleaned int
	for _, r := range f.GetRecords() {
		total += r.Size

		overSize := func() bool {
			if f.options.MaxSize == 0 {
				return false // Disabled
			}
			return total > f.options.MaxSize
		}

		overAge := func() bool {
			if f.options.MaxAge == time.Duration(0) {
				return false // Disabled
			}
			return r.Time.Before(gcStart.Add(-f.options.MaxAge))
		}

		if overSize() || overAge() {
			r.Delete()
			cleaned += 1
		}
	}
	if cleaned == 0 {
		return
	}

	log.Infof("Garbage collection removed %d records in %v", cleaned, time.Since(gcStart))

	// Filesystem was changed and a refresh is needed.
	f.Refresh()
}

func (r *VideoRecord) Delete() {
	if r.VideoPath != "" {
		delete(r.fs.videoDurationCache, r.VideoPath)
	}

	remove := func(p string) {
		if p == "" {
			return
		}
		if err := os.Remove(p); err != nil {
			log.Errorf("Garbage collection failed for %v: %v", p, err)
		}
	}
	remove(r.VideoPath)
	remove(r.ThumbPath)
	remove(r.VThumbPath)

	log.Infof("Deleted event %v", r.ID)
}

// Refresh triggers a manual refresh of the filesystem records.
func (f *Filesystem) Refresh() {
	select {
	case f.refreshc <- true:
	default:
	}
}

// GetRecords provides the current filesystem. Output be sorted by most recent first.
func (f *Filesystem) GetRecords() []*VideoRecord {
	f.l.Lock()
	var rs []*VideoRecord
	for _, r := range f.Records {
		rs = append(rs, r)
	}
	f.l.Unlock()

	sort.Slice(rs, func(i, j int) bool {
		return rs[j].Time.Before(rs[i].Time) // descending sort
	})
	return rs
}

func (f *Filesystem) GetRecordByID(ID string) *VideoRecord {
	f.l.Lock()
	defer f.l.Unlock()
	return f.Records[ID]
}
