package sink

import (
	"cam/video/source"
	"gocv.io/x/gocv"
)

// Video provides a sink that wraps opencv's VideoWriter.  This is unused
// because the uncompressed files produced are too large, or the default
// encoding options for compressed codecs are too slow / CPU intensive. Use
// FFmpeg sink instead.
type Video struct {
	writer *gocv.VideoWriter
}

func NewVideo(path string, fps int, width, height int) (*Video, error) {
	w, err := gocv.VideoWriterFile(path, "HFYU", float64(fps), width, height, true)
	if err != nil {
		return nil, err
	}
	return &Video{
		writer: w,
	}, nil
}

func (v *Video) Close() {
	v.writer.Close()
}

func (v *Video) Put(input source.Image) {
	v.writer.Write(input.Mat)
}
