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
	New() Sink
}
