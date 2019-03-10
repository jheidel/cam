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

	detections []process.Detection
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
		sink:   s,
		Record: r,
		p:      p,
	}
}

func (w *VideoSink) Put(i source.Image) {
	w.sink.Put(i)
}

func (w *VideoSink) SetDetections(detection process.Detections) {
	if len(detection) > 0 {
		log.Infof("Final detections for video: %v", detection.DebugString())
	}
	w.detections = detection.SortedDetections()
}

func (w *VideoSink) Close() {
	w.sink.Close()
	w.Record.UpdateVideo(w.detections)

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
