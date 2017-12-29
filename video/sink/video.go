package sink

import (
	"cam/video/source"
	"gocv.io/x/gocv"
	"time"
)

// Video writes a time-aligned video. Frames will be duplicated or skipped
// in order to maintain the timestamps provided in the stream.
type Video struct {
	frameDur time.Duration

	writer *gocv.VideoWriter

	last     gocv.Mat
	curFrame time.Time
}

func NewVideo(path string, fps int, width, height int) (*Video, error) {
	// TODO assert path ends in mkv?
	w, err := gocv.VideoWriterFile(path, "X264", float64(fps), width, height)
	if err != nil {
		return nil, err
	}
	return &Video{
		frameDur: time.Second / time.Duration(fps),
		writer:   w,
		last:     gocv.NewMat(),
	}, nil
}

func (v *Video) Close() {
	if err := v.writer.Close(); err != nil {
		// TODO
	}
	if err := v.last.Close(); err != nil {
		// TODO
	}
}

func (v *Video) Put(input source.Image) {
	if v.curFrame.IsZero() {
		input.Mat.CopyTo(v.last)
		v.writer.Write(input.Mat)
		v.curFrame = input.Time
		return
	}

	nextFrame := v.curFrame.Add(v.frameDur)
	if input.Time.Before(nextFrame) {
		// Don't need a new frame yet. Ignore.
		return
	}

	for {
		v.curFrame = nextFrame
		nextFrame = v.curFrame.Add(v.frameDur)
		if input.Time.Before(nextFrame) {
			v.writer.Write(input.Mat)
			input.Mat.CopyTo(v.last)
			return
		} else {
			// Missed a frame. Rewrite last frame.
			v.writer.Write(v.last)
		}
	}
}
