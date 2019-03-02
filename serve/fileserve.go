package serve

import (
	"cam/video"
	"fmt"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

// TODO limit read parallelism to avoid disk thrashing?

type FileServer struct {
	FS          *video.Filesystem
	PathFunc    func(r *video.VideoRecord) string
	ContentType string
}

func NewVideoServer(fs *video.Filesystem) *FileServer {
	return &FileServer{
		FS: fs,
		PathFunc: func(r *video.VideoRecord) string {
			return r.Paths().VideoPath
		},
		ContentType: "video/mp4",
	}
}

func NewThumbServer(fs *video.Filesystem) *FileServer {
	return &FileServer{
		FS: fs,
		PathFunc: func(r *video.VideoRecord) string {
			return r.Paths().ThumbPath
		},
		ContentType: "image/jpeg",
	}
}

func NewVThumbServer(fs *video.Filesystem) *FileServer {
	return &FileServer{
		FS: fs,
		PathFunc: func(r *video.VideoRecord) string {
			return r.Paths().VThumbPath
		},
		ContentType: "video/mp4",
	}
}

func (s *FileServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id := r.Form.Get("id")
	vr := s.FS.GetRecordByID(id)
	if vr == nil {
		http.Error(w, fmt.Sprintf("No record found for id %v", id), http.StatusNotFound)
		return
	}

	var err error
	var dl bool
	if r.Form.Get("download") != "" {
		dl, err = strconv.ParseBool(r.Form.Get("download"))
		if err != nil {
			http.Error(w, fmt.Sprintf("Invalid value for download: %v", err), http.StatusBadRequest)
			return
		}
	}

	p := s.PathFunc(vr)

	f, err := os.Open(p)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()

	if dl {
		fi, err := f.Stat()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Length", strconv.FormatInt(fi.Size(), 10))
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", path.Base(p)))
	}

	w.Header().Add("Content-Type", s.ContentType)
	http.ServeContent(w, r, p, time.Time{}, f)
}
