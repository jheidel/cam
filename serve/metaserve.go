package serve

import (
	"cam/video"
	"encoding/json"
	"net/http"
	"sort"
)

type MetaEntry struct {
	ID        string
	Timestamp int64

	HaveVideo  bool
	HaveThumb  bool
	HaveVThumb bool
}

type MetaResponse struct {
	Items []*MetaEntry
}

func toMetaEntry(r *video.VideoRecord) *MetaEntry {
	return &MetaEntry{
		ID:         r.ID,
		Timestamp:  r.Time.Unix(),
		HaveVideo:  len(r.VideoPath) > 0,
		HaveThumb:  len(r.ThumbPath) > 0,
		HaveVThumb: len(r.VThumbPath) > 0,
	}
}

type MetaServer struct {
	FS *video.Filesystem
}

func (s *MetaServer) BuildResponse() *MetaResponse {
	resp := &MetaResponse{}

	records := s.FS.GetRecords()
	for _, r := range records {
		resp.Items = append(resp.Items, toMetaEntry(r))
	}
	sort.Slice(resp.Items, func(i, j int) bool {
		return resp.Items[i].Timestamp < resp.Items[j].Timestamp
	})
	return resp
}

func (s *MetaServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	js, err := json.Marshal(s.BuildResponse())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
