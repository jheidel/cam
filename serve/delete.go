package serve

import (
	"cam/video"
	"fmt"
	"net/http"
)

type DeleteServer struct {
	FS *video.Filesystem
}

func (s *DeleteServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return
	}

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

	vr.Delete()
}
