package sink

import (
	"cam/video/source"
)

// Sink defines a destination for a stream of images, such as a video file or monitor.
type Sink interface {
	// Put inserts an image to the sink. The caller *must not* modify this image
	// and it should not hold any references to the underlying Mat.
	Put(input source.Image)

	// Close should be called to finalize the Sink.
	Close()
}

type SinkProducer interface {
	// New produces a new sink from the provided triggering image. In motion
	// detection cases, `trigger` will be the first frame containing the motion,
	// though it may not be the first frame written to the sink if the output is
	// buffered. `trigger` must be released when finished.
	New(trigger source.Image) Sink
}
