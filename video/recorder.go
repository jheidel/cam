package video

import (
	"fmt"
	"net/http"
	"time"

	"cam/video/sink"
	"cam/video/source"
)

type RecorderOptions struct {
	BufferTime, RecordTime time.Duration
}

type Recorder struct {
	producer sink.SinkProducer
	opts     *RecorderOptions
	buf      *Buffer

	input    chan source.Image
	inputack chan bool
	trigger  chan bool
	close    chan chan bool
}

func NewRecorder(p sink.SinkProducer, o *RecorderOptions) *Recorder {
	r := &Recorder{
		producer: p,
		opts:     o,
		buf:      NewBuffer(o.BufferTime),

		input:    make(chan source.Image),
		inputack: make(chan bool),
		trigger:  make(chan bool),
		close:    make(chan chan bool),
	}
	go func() {
		recording := false
		var out sink.Sink
		var stop <-chan time.Time

		for {
			select {
			case img := <-r.input:
				if recording {
					out.Put(img)
				}
				r.buf.Put(img)
				r.inputack <- true

			case <-r.trigger:
				if !recording {
					out = r.producer.New()
					r.buf.FlushToSink(out)
					recording = true
				}
				stop = time.NewTimer(r.opts.RecordTime).C

			case <-stop:
				if !recording {
					panic("expected to be in state recording")
				}
				out.Close()
				recording = false

			case c := <-r.close:
				if recording {
					out.Close()
				}
				r.buf.Close()
				c <- true
				return
			}
		}
	}()
	return r
}

func (r *Recorder) Put(input source.Image) {
	r.input <- input
	<-r.inputack
}

func (r *Recorder) Close() {
	c := make(chan bool)
	r.close <- c
	<-c
}

// Trigger will start recording to the SinkProducer, including `BufferTime` of
// history and lasting for `RecordTime`. Subsequent triggers will reset
// `RecordTime`.
func (r *Recorder) Trigger() {
	r.trigger <- true
}

// ServeHTTP implements http.Handler interface for manual triggering.
// TODO maybe move this to camera level?
func (r *Recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.Trigger()

	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "ok")
}
