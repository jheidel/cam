package source

import (
	"gocv.io/x/gocv"
	"log"
	"time"
)

type VideoCapture struct {
	URI  string
	pool *ImagePool
}

func NewVideoCapture(uri string) *VideoCapture {
	return &VideoCapture{
		URI:  uri,
		pool: NewImagePool(),
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
			// TODO handle better
			return
		}

		i := v.pool.New()
		for {
			// TODO add max FPS?

			i.Time = time.Now()
			if ok := cap.Read(i.Mat); !ok {
				time.Sleep(time.Millisecond)
				// TODO timeout; disconnect detect.
				log.Println("Read failure.")
				continue
			}
			c <- i
			i = v.pool.New()
		}
	}()
	return c
}
