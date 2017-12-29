package source

import (
	"gocv.io/x/gocv"
	"log"
	"time"
)

type Image struct {
	Mat  gocv.Mat
	Time time.Time

	// pool is the ImagePool that created this image.
	pool *ImagePool
}

func (i *Image) Release() {
	i.pool.free <- *i
}

func (i *Image) NewImage() Image {
	img := i.pool.New()
	img.Time = i.Time
	return img
}

type ImagePool struct {
	new  chan chan Image
	free chan Image

	allocated int
	available []Image
}

func (p *ImagePool) New() Image {
	r := make(chan Image)
	p.new <- r
	return <-r
}

func NewImagePool() *ImagePool {
	p := &ImagePool{
		new:  make(chan chan Image),
		free: make(chan Image),
	}
	go func() {
		for {
			select {
			case i := <-p.free:
				p.available = append(p.available, i)
			case r := <-p.new:
				var i Image
				if len(p.available) > 0 {
					i, p.available = p.available[0], p.available[1:]
				} else {
					i = Image{
						Mat:  gocv.NewMat(),
						pool: p,
					}
					p.allocated += 1
					// TODO clean; tie size to buffer.
					// TODO start blocking callers instead (supports the file dump case).
					if p.allocated > 500 {
						log.Fatalf("Too many ImagePool allocations. Perhaps an Image isn't being Released?")
					}
				}
				i.Time = time.Time{}
				r <- i
			}
		}
	}()
	return p
}

// TODO something that signifies whether the source is offline.

// Source defines a stream of images, such as a camera.
type Source interface {
	// Get generates a channel for receiving OpenCV images. The caller is free to
	// manipulate the provided Mat. Each Mat is guarenteed to be available until
	// the caller waits for the next image (the caller should not store pointers).
	Get() <-chan Image
}

// TODO
type Stream struct {
}
