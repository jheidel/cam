package video

import (
	"cam/video/process"
	"cam/video/sink"
	"cam/video/source"
	"log"
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
		err := process.WriteThumb(r.ThumbPath, trigger)
		if err != nil {
			log.Printf("failed to generate thubmnail: %v", err)
		} else {
			log.Printf("thumbnail written to %v", r.ThumbPath)
		}
		p.Filesystem.Refresh()
	}()

	var s sink.Sink
	s = sink.NewFFmpegSink(r.VideoPath, p.FFmpegOptions)
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

	// TODO thumbnail for first image.
}

func (w *sinkWrap) Close() {
	w.sink.Close()
	w.p.Filesystem.Refresh()

	// Create video thumbnail.
	c := w.p.VThumbProducer.Process(w.vr.VideoPath, w.vr.VThumbPath)
	go func() {
		if c != nil {
			<-c
			w.p.Filesystem.Refresh()
		}
	}()
}
