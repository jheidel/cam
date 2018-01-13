package source

import (
	"gocv.io/x/gocv"
	"image"
	"time"
)

// Image is
// TODO
type Image struct {
	Mat  gocv.Mat
	Time time.Time

	// pool is the MatPool that created this image. It can be used in order to allocate new Mats.
	pool *MatPool
}

func (i *Image) Release() {
	i.pool.ReleaseMat(i.Mat)
}

// Don't think this is used ?
func (i *Image) NewImage() Image {
	return Image{
		Mat:  i.pool.NewMat(),
		Time: i.Time,
	}
}

// TODO something that signifies whether the source is offline.

// Source defines a stream of images, such as a camera.
type Source interface {
	// Get generates a channel for receiving OpenCV images. The caller is free to
	// manipulate the provided Mat. Each Mat is guarenteed to be available until
	// the caller waits for the next image (the caller should not store pointers).
	Get() <-chan Image

	// Size returns the size of the capture source.
	Size() image.Point

	// Connected returns whether the capture source is considered "live".
	Connected() bool

	// Close disconnects from the capture source and frees up all resources.
	Close()
}
