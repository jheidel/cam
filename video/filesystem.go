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
	FileTimeLayout = "20060102-150405-Z0700"
)

type VideoRecord struct {
	Time time.Time

	VideoPath  string
	ThumbPath  string
	VThumbPath string
}

// TODO json version for frontend.
// TODO auto refresh periodic.
// TODO refresh when things change.

type Filesystem struct {
	BasePath string

	Records []*VideoRecord

	l sync.Mutex
}

func NewFilesystem(path string) (*Filesystem, error) {
	if err := os.MkdirAll(path, 0755); err != nil {
		return nil, err
	}
	return &Filesystem{
		BasePath: path,
	}, nil
}

func (f *Filesystem) NewRecord(t time.Time) *VideoRecord {
	base := filepath.Join(f.BasePath, t.Format(FileTimeLayout))
	return &VideoRecord{
		Time:       t,
		VideoPath:  base + ExtVideo,
		ThumbPath:  base + ExtThumb,
		VThumbPath: base + ExtVThumb,
	}
}

func (f *Filesystem) refresh() error {
	m := make(map[time.Time]*VideoRecord)

	files, err := ioutil.ReadDir(f.BasePath)
	if err != nil {
		return err
	}

	for _, file := range files {
		b := file.Name()
		if len(b) < len(FileTimeLayout) {
			continue
		}
		t, err := time.Parse(FileTimeLayout, b[:len(FileTimeLayout)])
		if err != nil {
			continue
		}

		v := m[t]
		if v == nil {
			v = &VideoRecord{
				Time: t,
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

		m[t] = v
	}

	records := make([]*VideoRecord, 0, len(m))
	for _, v := range m {
		records = append(records, v)
	}

	f.l.Lock()
	defer f.l.Unlock()
	f.Records = records

	return nil
}

func (f *Filesystem) GetRecords() []*VideoRecord {
	f.l.Lock()
	defer f.l.Unlock()
	return f.Records[:]
}

// TODO garbage collecting.
