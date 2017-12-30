package sink

import (
	"cam/video/source"
	"gocv.io/x/gocv"
	"time"
)

// FPSNormalize wraps another Sink so that an incoming stream of variable-timed
// video is converted to fixed-rate video. This is useful for exporting a
// webcam feed (which may have variable frame rate) to a video file which
// requires fixed frame rate. Frames will be dropped or added in order to
// achieve the target frame rate.
type FPSNormalize struct {
	// sink is the wrapped Sink which will receive a FPS-normalized stream.
	sink Sink

	frameDur time.Duration
	last     gocv.Mat
	curFrame time.Time
}

// NewFPSNormalize creates an FPSNormalize, wrapping the provided sink and
// exporting at the given frame rate.
func NewFPSNormalize(sink Sink, fps int) *FPSNormalize {
	return &FPSNormalize{
		sink:     sink,
		frameDur: time.Second / time.Duration(fps),
		last:     gocv.NewMat(),
	}
}

func (f *FPSNormalize) Close() {
	f.sink.Close()
	f.last.Close()
}

func (f *FPSNormalize) Put(input source.Image) {

	if f.curFrame.IsZero() {
		f.sink.Put(input)
		input.Mat.CopyTo(f.last)
		f.curFrame = input.Time
		return
	}

	nextFrame := f.curFrame.Add(f.frameDur)
	if input.Time.Before(nextFrame) {
		// Don't need a new frame yet. Ignore.
		return
	}

	for {
		f.curFrame = nextFrame
		nextFrame = f.curFrame.Add(f.frameDur)
		if input.Time.Before(nextFrame) {

			// TODO pool?
			i := source.Image{
				Mat:  input.Mat,
				Time: f.curFrame,
				// TODO pool propagate?
			}
			f.sink.Put(i)
			input.Mat.CopyTo(f.last)
			return
		} else {
			// Missed a frame. Rewrite last frame.
			i := source.Image{
				Mat:  f.last,
				Time: f.curFrame,
				// TODO pool propagate?
			}
			f.sink.Put(i)
		}
	}
}
