package video

import (
	"fmt"
	"net/http"
	"time"

	"cam/video/process"
	"cam/video/source"
)

type RecorderOptions struct {
	BufferTime, RecordTime, MaxRecordTime time.Duration
}

type Recorder struct {
	producer *VideoSinkProducer
	opts     *RecorderOptions
	buf      *Buffer

	input     chan source.Image
	inputack  chan bool
	trigger   chan bool
	detection chan process.Detections
	close     chan chan bool
}

func NewRecorder(p *VideoSinkProducer, o *RecorderOptions) *Recorder {
	r := &Recorder{
		producer: p,
		opts:     o,
		buf:      NewBuffer(o.BufferTime),

		input:     make(chan source.Image),
		inputack:  make(chan bool),
		trigger:   make(chan bool),
		detection: make(chan process.Detections),
		close:     make(chan chan bool),
	}
	go func() {
		recording := false
		var out *VideoSink
		var detection process.Detections
		var stop <-chan time.Time
		var stopLong <-chan time.Time

		stopFunc := func() {
			if !recording {
				panic("expected to be in state recording")
			}
			out.SetDetections(detection)
			go out.Close()
			recording = false
			stop = nil
			stopLong = nil
		}

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
					out = r.producer.New(r.buf.GetLast())
					detection = make(process.Detections)
					r.buf.FlushToSink(out)
					recording = true
					stopLong = time.NewTimer(r.opts.MaxRecordTime).C
				}
				stop = time.NewTimer(r.opts.RecordTime).C

			case d := <-r.detection:
				if detection != nil {
					detection.Merge(d)
				}

			case <-stop:
				stopFunc()
			case <-stopLong:
				stopFunc()

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

func (r *Recorder) Detection(d process.Detections) {
	r.detection <- d
}

// ServeHTTP implements http.Handler interface for manual triggering.
// TODO maybe move this to camera level?
func (r *Recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.Trigger()

	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "ok")
}
