package video

import (
	"cam/video/process"
	"cam/video/sink"
	"cam/video/source"

	log "github.com/sirupsen/logrus"
)

type VideoSinkProducer struct {
	FFmpegOptions  sink.FFmpegOptions
	Filesystem     *Filesystem
	VThumbProducer *process.VThumbProducer
}

type VideoSink struct {
	sink sink.Sink

	Record *VideoRecord

	detections process.Detections
	p          *VideoSinkProducer
}

func (p *VideoSinkProducer) New(trigger source.Image) *VideoSink {
	r := p.Filesystem.NewRecord(trigger.Time)

	go func() {
		defer trigger.Release()
		path := r.Paths().ThumbPath
		err := process.WriteThumb(path, trigger)
		if err != nil {
			log.Errorf("failed to generate thumbnail: %v", err)
		} else {
			log.Infof("thumbnail written to %v", path)
		}
		r.UpdateThumb()
	}()

	var s sink.Sink
	path := r.Paths().VideoPath
	s = sink.NewFFmpegSink(path, p.FFmpegOptions)
	// Ensure video is output with constant FPS.
	s = sink.NewFPSNormalize(s, p.FFmpegOptions.FPS)

	return &VideoSink{
		sink:       s,
		Record:     r,
		detections: make(process.Detections),
		p:          p,
	}
}

func (w *VideoSink) Put(i source.Image) {
	w.sink.Put(i)
}

func (w *VideoSink) AddDetections(detections process.Detections) {
	if detections == nil || len(detections) == 0 {
		return
	}
	if len(w.detections) == 0 {
		// First detection: make an update so it shows up in the UI.
		w.Record.SetDetections(detections.SortedDetections())
	}
	w.detections.Merge(detections)
}

func (w *VideoSink) Close() {
	w.sink.Close()
	w.Record.UpdateVideo(w.detections.SortedDetections())

	// Create video thumbnail.
	paths := w.Record.Paths()
	c := w.p.VThumbProducer.Process(paths.VideoPath, paths.VThumbPath)
	go func() {
		if c != nil {
			<-c
			w.Record.UpdateVThumb()
		}
	}()
}
