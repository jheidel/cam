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

type RecorderListener interface {
	// Invoked when recording starts.
	StartRecording(vr *VideoRecord)

	// Invoked when recording stops.
	StopRecording(vr *VideoRecord)
}

type Recorder struct {
	Listeners []RecorderListener

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
		var stop <-chan time.Time
		var stopLong <-chan time.Time

		stopFunc := func() {
			if !recording {
				panic("expected to be in state recording")
			}
			go out.Close()
			for _, l := range r.Listeners {
				l.StopRecording(out.Record)
			}
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
					r.buf.FlushToSink(out)
					recording = true
					stopLong = time.NewTimer(r.opts.MaxRecordTime).C
					for _, l := range r.Listeners {
						l.StartRecording(out.Record)
					}
				}
				stop = time.NewTimer(r.opts.RecordTime).C

			case d := <-r.detection:
				if recording {
					out.AddDetections(d)
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

// MotionDetected will start recording to the SinkProducer, including `BufferTime` of
// history and lasting for `RecordTime`. Subsequent triggers will reset
// `RecordTime`.
func (r *Recorder) MotionDetected() {
	r.trigger <- true
}

func (r *Recorder) MotionClassified(d process.Detections) {
	r.detection <- d
}

// ServeHTTP implements http.Handler interface for manual triggering.
// TODO maybe move this to camera level?
func (r *Recorder) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.MotionDetected()

	w.Header().Add("Content-Type", "text/plain; charset=utf-8")
	fmt.Fprintln(w, "ok")
}

type ClassifierRecordTrigger struct {
	Classifier *process.Classifier
}

func (t *ClassifierRecordTrigger) StartRecording(vr *VideoRecord) {
	t.Classifier.Enable()
}

func (t *ClassifierRecordTrigger) StopRecording(vr *VideoRecord) {
	t.Classifier.Disable()
}
