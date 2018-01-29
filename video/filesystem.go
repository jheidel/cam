package video

import (
	"io/ioutil"
	"os"
	"path/filepath"
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
)

var (
	FilesystemRefresh = time.Minute
)

type VideoRecord struct {
	Time time.Time
	ID   string

	VideoPath  string
	ThumbPath  string
	VThumbPath string
}

// TODO json version for frontend.

type Filesystem struct {
	BasePath string

	Records map[string]*VideoRecord

	refreshc chan bool
	l        sync.Mutex
}

func NewFilesystem(path string) (*Filesystem, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}
	f := &Filesystem{
		BasePath: path,
		refreshc: make(chan bool, 1),
	}
	go func() {
		f.refresh()
		t := time.NewTicker(FilesystemRefresh)
		for {
			select {
			case <-t.C:
				f.refresh()
			case <-f.refreshc:
				f.refresh()
			}
		}
	}()
	return f, nil
}

func (f *Filesystem) NewRecord(t time.Time) *VideoRecord {
	id := t.Format(FileTimeLayout)
	base := filepath.Join(f.BasePath, id)
	return &VideoRecord{
		Time: t,
		ID:   id,

		VideoPath:  base + ExtVideo,
		ThumbPath:  base + ExtThumb,
		VThumbPath: base + ExtVThumb,
	}
}

func (f *Filesystem) refresh() error {
	m := make(map[string]*VideoRecord)

	files, err := ioutil.ReadDir(f.BasePath)
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
			}
		}

		p := filepath.Join(f.BasePath, b)
		switch {
		case strings.HasSuffix(b, ExtVideo):
			v.VideoPath = p
		case strings.HasSuffix(b, ExtThumb):
			v.ThumbPath = p
		case strings.HasSuffix(b, ExtVThumb):
			v.VThumbPath = p
		default:
			continue
		}

		m[id] = v
	}

	f.l.Lock()
	defer f.l.Unlock()
	f.Records = m

	return nil
}

// Refresh triggers a manual refresh of the filesystem records.
func (f *Filesystem) Refresh() {
	select {
	case f.refreshc <- true:
	default:
	}
}

func (f *Filesystem) GetRecords() []*VideoRecord {
	f.l.Lock()
	defer f.l.Unlock()

	var rs []*VideoRecord
	for _, r := range f.Records {
		rs = append(rs, r)
	}
	return rs
}

func (f *Filesystem) GetRecordByID(ID string) *VideoRecord {
	f.l.Lock()
	defer f.l.Unlock()
	return f.Records[ID]
}

// TODO garbage collecting old records.
