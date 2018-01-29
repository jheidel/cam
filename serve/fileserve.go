package serve

import (
	"cam/video"
	"fmt"
	"io"
	"net/http"
	"os"
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
			return r.VideoPath
		},
		ContentType: "video/mp4",
	}
}

func NewThumbServer(fs *video.Filesystem) *FileServer {
	return &FileServer{
		FS: fs,
		PathFunc: func(r *video.VideoRecord) string {
			return r.ThumbPath
		},
		ContentType: "image/jpeg",
	}
}

func NewVThumbServer(fs *video.Filesystem) *FileServer {
	return &FileServer{
		FS: fs,
		PathFunc: func(r *video.VideoRecord) string {
			return r.VThumbPath
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

	f, err := os.Open(s.PathFunc(vr))
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer f.Close()

	w.Header().Add("Content-Type", s.ContentType)
	io.Copy(w, f)
}
