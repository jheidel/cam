package source

import (
	"gocv.io/x/gocv"
	"image"
	"log"
	"time"
)

type VideoCapture struct {
	URI string

	pool *MatPool
}

// NewVideoCapture opens a capture source from the provided URI. It supports
// any format compatible with OpenCV. Assuming FFmpeg is compiled correctly,
// this includes support for MJPEG and RSTP IP cameras.
func NewVideoCapture(uri string) *VideoCapture {
	return &VideoCapture{
		URI:  uri,
		pool: NewMatPool(),
	}
}

func (v *VideoCapture) Get() <-chan Image {
	c := make(chan Image)
	go func() {

		// TODO fetch a test image first.
		// TODO need to expose size.
		// TODO handle disconnects.

		cap, err := gocv.VideoCaptureFile(v.URI)
		if err != nil {
			log.Fatalf("Failed to open video capture %v", err)
			// TODO handle errors better
			return
		}

		m := v.pool.NewMat()
		for {
			// TODO add max FPS?
			i := Image{
				Mat:  m,
				Time: time.Now(),
				pool: v.pool,
			}
			if ok := cap.Read(i.Mat); !ok {
				time.Sleep(time.Millisecond)
				// TODO timeout; disconnect detect.
				log.Println("Read failure.")
				continue
			}
			c <- i
			m = v.pool.NewMat()
		}
	}()
	return c
}

func (v *VideoCapture) Size() image.Point {
	// TODO
	return image.Point{}
}

func (v *VideoCapture) Connected() bool {
	return true

}
