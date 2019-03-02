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

type sinkWrap struct {
	sink sink.Sink
	vr   *VideoRecord

	p *VideoSinkProducer
}

func (p *VideoSinkProducer) New(trigger source.Image) sink.Sink {
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

	return &sinkWrap{
		sink: s,
		vr:   r,
		p:    p,
	}
}

func (w *sinkWrap) Put(i source.Image) {
	w.sink.Put(i)
}

func (w *sinkWrap) Close() {
	w.sink.Close()
	w.vr.UpdateVideo()

	// Create video thumbnail.
	paths := w.vr.Paths()
	c := w.p.VThumbProducer.Process(paths.VideoPath, paths.VThumbPath)
	go func() {
		if c != nil {
			<-c
			w.vr.UpdateVThumb()
		}
	}()
}
