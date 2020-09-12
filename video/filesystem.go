package video

import (
	"bufio"
	"bytes"
	"cam/video/process"
	"encoding/gob"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	"github.com/pillash/mp4util"
	log "github.com/sirupsen/logrus"
)

const (
	// ExtVideo is the extension for video files.
	ExtVideo = "_video.mp4"
	// ExtThumb is the extension for thumbnail files.
	ExtThumb = "_thumb.jpg"
	// ExtVThumb is the extension for video thumbnail files.
	ExtVThumb = "_vthumb.mp4"

	// FileTimeLayout defines the format of filenames.
	// See https://golang.org/src/time/format.go.
	FileTimeLayout = "20060102-150405Z0700"

	// DatabaseFile is the path to the sqlite3 database.
	DatabaseFile = "cam.db"
)

var (
	// FilesystemRefreshInterval controls the frequency of periodic disk consistency
	// scans.
	FilesystemRefreshInterval = 10 * time.Minute
	// GarbageCollectionInterval controls the frequency of garbage collection.
	GarbageCollectionInterval = 30 * time.Minute
)

type FilesystemListener interface {
	FilesystemUpdated()
}

type Classification struct {
	Detections []process.Detection
}

type VideoRecord struct {
	gorm.Model

	TriggeredAt time.Time
	Identifier  string `gorm:"type:varchar(100);unique_index"`

	HaveVideo  bool
	HaveThumb  bool
	HaveVThumb bool

	// Length of the video file.
	VideoDurationSec int

	// Combined size of this record on disk.
	Size int64

	HaveClassification  bool
	Classification      *Classification
	ClassificationBytes []byte

	// Reference to parent.
	fs *Filesystem
	l  sync.Mutex
}

func (r *VideoRecord) BeforeCreate() error {
	return r.BeforeSave()
}
func (r *VideoRecord) BeforeUpdate() error {
	return r.BeforeSave()
}
func (r *VideoRecord) BeforeSave() error {
	if r.Classification == nil {
		return nil
	}
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	e := gob.NewEncoder(w)
	if err := e.Encode(r.Classification); err != nil {
		return err
	}
	if err := w.Flush(); err != nil {
		return err
	}
	r.ClassificationBytes = b.Bytes()
	return nil
}

func (r *VideoRecord) AfterFind() error {
	if len(r.ClassificationBytes) == 0 {
		return nil
	}
	br := bytes.NewReader(r.ClassificationBytes)
	d := gob.NewDecoder(br)
	c := &Classification{}
	if err := d.Decode(c); err != nil {
		log.Errorf("Failed to decode classification %v", err)
		return err
	}
	r.Classification = c
	return nil
}

// VideoRecordPaths defines the absolute paths where new files should be created.
type VideoRecordPaths struct {
	VideoPath  string
	ThumbPath  string
	VThumbPath string
}

// Paths provides locations for where new files should be created.
func (r *VideoRecord) Paths() *VideoRecordPaths {
	return &VideoRecordPaths{
		VideoPath:  filepath.Join(r.fs.options.BasePath, r.Identifier+ExtVideo),
		ThumbPath:  filepath.Join(r.fs.options.BasePath, r.Identifier+ExtThumb),
		VThumbPath: filepath.Join(r.fs.options.BasePath, r.Identifier+ExtVThumb),
	}
}

func (r *VideoRecord) UpdateVideo(detections []process.Detection) {
	p := r.Paths().VideoPath
	fi, err := os.Stat(p)
	if err != nil {
		log.Errorf("Failed to stat %v: %v", p, err)
		return
	}
	ds, err := mp4util.Duration(p)
	if err != nil {
		log.Errorf("Failed to get video duration %v: %v", p, err)
		return
	}
	r.l.Lock()
	defer r.l.Unlock()
	r.HaveVideo = true
	r.Size += fi.Size()
	r.VideoDurationSec = ds
	if len(detections) > 0 {
		r.HaveClassification = true
		r.Classification = &Classification{
			Detections: detections,
		}
	}
	r.fs.db.Save(r)
	r.fs.notifyListeners()
}

func (r *VideoRecord) UpdateThumb() {
	p := r.Paths().ThumbPath
	fi, err := os.Stat(p)
	if err != nil {
		log.Errorf("Failed to stat %v: %v", p, err)
		return
	}
	r.l.Lock()
	defer r.l.Unlock()
	r.HaveThumb = true
	r.Size += fi.Size()
	r.fs.db.Save(r)
	r.fs.notifyListeners()
}

func (r *VideoRecord) UpdateVThumb() {
	p := r.Paths().VThumbPath
	fi, err := os.Stat(p)
	if err != nil {
		log.Errorf("Failed to stat %v: %v", p, err)
		return
	}
	r.l.Lock()
	defer r.l.Unlock()
	r.HaveVThumb = true
	r.Size += fi.Size()
	r.fs.db.Save(r)
	r.fs.notifyListeners()
}

type FilesystemOptions struct {
	// Root directory for the filesystem. All videos will be stored here, along
	// with the sqlite database file.
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
	db               *gorm.DB
	options          FilesystemOptions
	listeners        []FilesystemListener
	listenersDisable bool
	l                sync.Mutex
}

func NewFilesystem(opts FilesystemOptions) (*Filesystem, error) {
	if err := os.MkdirAll(opts.BasePath, 0755); err != nil {
		return nil, err
	}

	path := filepath.Join(opts.BasePath, DatabaseFile)
	db, err := gorm.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	db.AutoMigrate(&VideoRecord{})

	f := &Filesystem{
		db:      db,
		options: opts,
	}
	go func() {
		log.Infof("Starting initial filesystem refresh (%v). This could take a while.", opts.BasePath)
		// Initial filesystem scan.
		if err := f.doRefresh(); err != nil {
			log.Errorf("Initial filesystem refresh failed: %v", err)
		}
		log.Infof("Initial filesystem scan completed")
		rt := time.NewTicker(FilesystemRefreshInterval)
		gt := time.NewTicker(GarbageCollectionInterval)
		for {
			select {
			case <-gt.C:
				f.doGarbageCollect()
			case <-rt.C:
				if err := f.doRefresh(); err != nil {
					log.Errorf("Periodic ilesystem refresh failed: %v", err)
				}
			}
		}
	}()
	return f, nil
}

func (f *Filesystem) NewRecord(t time.Time) *VideoRecord {
	id := t.Format(FileTimeLayout)
	vr := &VideoRecord{
		TriggeredAt: t,
		Identifier:  id,
		fs:          f,
	}
	f.db.Create(vr)
	return vr
}

func (f *Filesystem) scanFilesystem() (map[string]*VideoRecord, error) {
	start := time.Now()
	defer func() {
		et := time.Since(start)
		if et < time.Second {
			log.Debugf("Filesystem scan completed in %v", et)
		} else {
			log.Infof("Filesystem scan (slow) completed in %v", et)
		}
	}()

	m := make(map[string]*VideoRecord)

	files, err := ioutil.ReadDir(f.options.BasePath)
	if err != nil {
		return nil, err
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
				TriggeredAt: t,
				Identifier:  id,
				fs:          f,
			}
			m[id] = v
		}

		switch {
		case strings.HasSuffix(b, ExtVideo):
			v.HaveVideo = true
		case strings.HasSuffix(b, ExtThumb):
			v.HaveThumb = true
		case strings.HasSuffix(b, ExtVThumb):
			v.HaveVThumb = true
		default:
			continue
		}
	}

	return m, nil
}

func (f *Filesystem) doRefresh() error {
	fsm, err := f.scanFilesystem()
	if err != nil {
		return err
	}
	log.Infof("Found %d records in filesystem", len(fsm))
	if len(fsm) == 0 {
		return fmt.Errorf("Failed to look up records from filesystem, found zero")
	}

	// Determine the set of identifiers present in the database.
	dbm := make(map[string]bool)
	var found []string
	if err := f.db.Model(&VideoRecord{}).Pluck("identifier", &found).Error; err != nil {
		return fmt.Errorf("failed to look up list of db identifiers: %v", err)
	}
	for _, id := range found {
		dbm[id] = true
	}
	log.Infof("Found %d records in database", len(dbm))
	if len(dbm) == 0 {
		return fmt.Errorf("Failed to look up records from db, found zero")
	}

	// `dbm` becomes the records that are not present on the filesystem.
	for k := range fsm {
		delete(dbm, k)
	}
	// `fsm` becomes the records missing in the database.
	for _, k := range found {
		delete(fsm, k)
	}

	if len(dbm) == 0 && len(fsm) == 0 {
		// Everything in sync.
		return nil
	}

	var deleted []string
	if err := f.db.Unscoped().Model(&VideoRecord{}).Where("deleted_at IS NOT NULL").Pluck("identifier", &deleted).Error; err != nil {
		return fmt.Errorf("failed to look up list of deleted db identifiers: %v", err)
	}
	if len(deleted) == 0 {
		return fmt.Errorf("Failed to look up deleted records from filesystem, found zero")
	}
	log.Infof("Found %d deleted records in database", len(deleted))
	delm := make(map[string]*VideoRecord)
	for _, k := range deleted {
		if vr, ok := fsm[k]; ok {
			delete(fsm, k)
			delm[k] = vr
		}
	}

	log.Infof("%d records missing in database, %d records extra in database. %d deleted records in filesystem. Starting sync.", len(fsm), len(dbm), len(delm))
	start := time.Now()
	defer func() {
		et := time.Since(start)
		if et < time.Second {
			log.Debugf("Filesystem sync completed in %v", et)
		} else {
			log.Infof("Filesystem sync (slow) completed in %v", et)
		}
	}()

	// Suppress filesystem listeners while the update happens and notify all at once at the end.
	ne := f.notifyListenersInBatch()
	defer func() {
		ne <- true
	}()

	// Delete extra records.
	for id := range dbm {
		vr := f.GetRecordByID(id)
		vr.Delete()
	}

	// Remove deleted records from filesystem
	for _, vr := range delm {
		vr.Delete()
	}

	// Insert missing records.
	for _, vr := range fsm {
		f.db.Create(vr)
		if vr.HaveThumb {
			vr.UpdateThumb()
		}
		if vr.HaveVThumb {
			vr.UpdateVThumb()
		}
		if vr.HaveVideo {
			vr.UpdateVideo(nil)
		}
	}

	return nil
}

func (f *Filesystem) notifyListenersInBatch() chan<- bool {
	f.l.Lock()
	f.listenersDisable = true
	f.l.Unlock()
	c := make(chan bool)
	go func() {
		<-c
		f.l.Lock()
		f.listenersDisable = false
		f.l.Unlock()
		f.notifyListeners()
	}()
	return c
}

func (f *Filesystem) notifyListeners() {
	f.l.Lock()
	defer f.l.Unlock()
	if f.listenersDisable {
		return
	}
	for _, listener := range f.listeners {
		go listener.FilesystemUpdated()
	}
}

func (f *Filesystem) AddListener(l FilesystemListener) {
	f.l.Lock()
	defer f.l.Unlock()
	f.listeners = append(f.listeners, l)
}

func (f *Filesystem) doGarbageCollect() {
	gcStart := time.Now()
	var toDelete []*VideoRecord
	var total int64
	for _, r := range f.GetRecords(&RecordsFilter{}) {
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
			return r.TriggeredAt.Before(gcStart.Add(-f.options.MaxAge))
		}

		if overSize() || overAge() {
			toDelete = append(toDelete, r)
		}
	}
	if len(toDelete) == 0 {
		return
	}

	ne := f.notifyListenersInBatch()
	defer func() {
		ne <- true
	}()

	for _, r := range toDelete {
		r.Delete()
	}
	log.Infof("Garbage collection removed %d records in %v", len(toDelete), time.Since(gcStart))
}

func (r *VideoRecord) Delete() {
	remove := func(p string) {
		if p == "" {
			return
		}
		if err := os.Remove(p); err != nil {
			log.Errorf("Garbage collection failed for %v: %v", p, err)
		}
	}
	paths := r.Paths()
	if r.HaveVideo {
		remove(paths.VideoPath)
	}
	if r.HaveThumb {
		remove(paths.ThumbPath)
	}
	if r.HaveVThumb {
		remove(paths.VThumbPath)
	}
	r.fs.db.Delete(r)
	log.Infof("Deleted event %v (id=%v)", r.Identifier, r.ID)

	r.fs.notifyListeners()
}

type RecordsFilter struct {
	HaveClassification bool
}

// GetRecords provides the current filesystem. Output be sorted by most recent first.
func (f *Filesystem) GetRecords(filter *RecordsFilter) []*VideoRecord {
	var records []*VideoRecord
	q := f.db.Order("triggered_at DESC")
	if filter.HaveClassification {
		q = q.Where("have_classification = true")
	}
	if err := q.Find(&records).Error; err != nil {
		log.Errorf("Record lookup failed: %v", err)
		return []*VideoRecord{}
	}
	for _, r := range records {
		r.fs = f
	}
	return records
}

func (f *Filesystem) GetRecordByID(ID string) *VideoRecord {
	record := &VideoRecord{}
	if f.db.Where("identifier = ?", ID).First(record).RecordNotFound() {
		return nil
	}
	record.fs = f
	return record
}
