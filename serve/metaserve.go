package serve

import (
	"cam/video"
	"encoding/json"
	"net/http"
)

type MetaEntry struct {
	ID        string
	Timestamp int64

	HaveVideo  bool
	HaveThumb  bool
	HaveVThumb bool

	DurationSec int
}

type MetaResponse struct {
	Items []*MetaEntry

	ItemsTotalSize  int64
	ItemsCount      int
	OldestTimestamp int64
}

func toMetaEntry(r *video.VideoRecord) *MetaEntry {
	return &MetaEntry{
		ID:          r.Identifier,
		Timestamp:   r.TriggeredAt.Unix(),
		HaveVideo:   r.HaveVideo,
		HaveThumb:   r.HaveThumb,
		HaveVThumb:  r.HaveVThumb,
		DurationSec: r.VideoDurationSec,
	}
}

type MetaServer struct {
	FS *video.Filesystem
}

func (s *MetaServer) BuildResponse() *MetaResponse {
	records := s.FS.GetRecords()

	resp := &MetaResponse{}
	var sz int64
	for _, r := range records {
		resp.Items = append(resp.Items, toMetaEntry(r))
		sz += r.Size
		resp.OldestTimestamp = r.TriggeredAt.Unix()
	}
	resp.ItemsTotalSize = sz
	resp.ItemsCount = len(records)
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
