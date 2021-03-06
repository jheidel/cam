package video

import (
	"cam/video/process"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/pillash/mp4util"
	log "github.com/sirupsen/logrus"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
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
)

var (
	// GarbageCollectionInterval controls the frequency of garbage collection.
	GarbageCollectionInterval = 30 * time.Minute
)

type FilesystemListener interface {
	FilesystemUpdated()
}

type Classification struct {
	Detections []process.Detection
}

func (c *Classification) Scan(value interface{}) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal Classification JSON value:", value))
	}
	return json.Unmarshal(bytes, c)
}

func (c *Classification) Value() (driver.Value, error) {
	if c == nil {
		return nil, nil
	}
	return json.Marshal(c)
}

// I'm adding this just because I'm running into a strange table corruption
// issue with the gorm schema for VideoRecord. By inserting another table
// first, maybe that will force different behavior to shed some light on the
// issue?
type DummyModel struct {
	gorm.Model
	DummyField bool
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

	HaveClassification bool
	Classification     *Classification

	// Reference to parent.
	fs *Filesystem
	l  sync.Mutex
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

func (r *VideoRecord) SetDetections(detections []process.Detection) {
	defer r.fs.notifyListeners()
	r.l.Lock()
	defer r.l.Unlock()
	r.setDetections(detections)
	if err := r.fs.db.Debug().Save(r).Error; err != nil {
		log.Fatalf("SetDetections.Save %v for %v", err, spew.Sdump(r))
	}
}

func (r *VideoRecord) setDetections(detections []process.Detection) {
	if len(detections) == 0 {
		return
	}
	r.HaveClassification = true
	r.Classification = &Classification{
		Detections: detections,
	}
}

func (r *VideoRecord) UpdateVideo(detections []process.Detection) {
	defer r.fs.notifyListeners()
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
	r.setDetections(detections)
	if err = r.fs.db.Debug().Save(r).Error; err != nil {
		log.Fatalf("UpdateVideo.Save %v for %v", err, spew.Sdump(r))
	}
}

func (r *VideoRecord) UpdateThumb() {
	defer r.fs.notifyListeners()

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
	if err = r.fs.db.Debug().Save(r).Error; err != nil {
		log.Fatalf("UpdateThumb.Save %v for %v", err, spew.Sdump(r))
	}
}

func (r *VideoRecord) UpdateVThumb() {
	defer r.fs.notifyListeners()

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
	if err = r.fs.db.Debug().Save(r).Error; err != nil {
		log.Fatalf("UpdateVThumb.Save %v for %v", err, spew.Sdump(r))
	}
}

func (r *VideoRecord) Delete() {
	defer r.fs.notifyListeners()

	r.l.Lock()
	defer r.l.Unlock()

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
	// Hard delete from database.
	// TODO soft delete is probably fine
	if err := r.fs.db.Unscoped().Delete(r).Error; err != nil {
		log.Fatalf("Delete %v for %v", err, spew.Sdump(r))
	}
	log.Infof("Deleted event %v (id=%v)", r.Identifier, r.ID)
}

type FilesystemOptions struct {
	// URI for connecting to the database.
	DatabaseURI string

	// Root directory for the filesystem. All videos will be stored here.
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

func (f *Filesystem) DB() *gorm.DB {
	return f.db
}

type dbConnector struct {
	MysqlURI string
}

func (c *dbConnector) Connect() (*gorm.DB, error) {
	if c.MysqlURI == "" {
		return nil, nil
	}
	db, err := gorm.Open(mysql.Open(c.MysqlURI+"?charset=utf8mb4&parseTime=True&loc=Local"), &gorm.Config{
		// TODO customize logger to integrate with logrus instead of dumping to
		// stdout using a different format
		Logger: logger.Default.LogMode(logger.Info),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	db.AutoMigrate(&DummyModel{})
	db.AutoMigrate(&VideoRecord{})
	log.Infof("Connected to mysql database")
	return db, nil
}

func NewFilesystem(opts FilesystemOptions) (*Filesystem, error) {
	if err := os.MkdirAll(opts.BasePath, 0755); err != nil {
		return nil, err
	}
	connector := &dbConnector{
		MysqlURI: opts.DatabaseURI,
	}
	db, err := connector.Connect()
	if err != nil {
		return nil, err
	}
	f := &Filesystem{
		db:      db,
		options: opts,
	}

	go func() {
		gt := time.NewTicker(GarbageCollectionInterval)
		for {
			select {
			case <-gt.C:
				f.doGarbageCollect()
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
	if err := f.db.Debug().Create(vr).Error; err != nil {
		log.Fatalf("Failed to create new record: %v", err)
	}
	return vr
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

type RecordsFilter struct {
	HaveClassification bool
}

// GetRecords provides the current filesystem. Output be sorted by most recent first.
func (f *Filesystem) GetRecords(filter *RecordsFilter) []*VideoRecord {
	var records []*VideoRecord
	q := f.db.Debug().Order("triggered_at DESC")
	if filter.HaveClassification {
		q = q.Where("have_classification = true")
	}
	if err := q.Find(&records).Error; err != nil {
		log.Fatalf("Record lookup failed: %v for filter %v", err, spew.Sdump(filter))
		return []*VideoRecord{}
	}
	for _, r := range records {
		r.fs = f
	}
	return records
}

func (f *Filesystem) GetRecordByID(ID string) *VideoRecord {
	record := &VideoRecord{}
	if err := f.db.Where("identifier = ?", ID).First(record).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		log.Fatalf("GetRecordById %v over ID %v", err, ID)
	}
	record.fs = f
	return record
}
