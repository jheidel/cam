package source

import (
	"fmt"
	"image"
	"time"

	"cam/util"

	log "github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
)

const (
	// retryDelay defines the time to wait between reconnection attempts.
	retryDelay = 2 * time.Second

	// disconnectDelay defines the threshold where a reconnect should be attempted.
	disconnectDelay = 5 * time.Second
)

type VideoCapture struct {
	URI string

	// This should be set high for realtime sources. For files it should be set
	// to the actual framerate.
	FPS int

	init *util.Event

	close chan chan bool

	cap       *gocv.VideoCapture
	sz        image.Point
	lastFetch time.Time
}

// TODO better FPS logic.

// NewVideoCapture opens a capture source from the provided URI. It supports
// any format compatible with OpenCV. Assuming FFmpeg is compiled correctly,
// this includes support for MJPEG and RSTP IP cameras.
func NewVideoCapture(uri string, fps int) *VideoCapture {
	return &VideoCapture{
		URI:   uri,
		FPS:   fps,
		close: make(chan chan bool, 1),
		init:  util.NewEvent(),
	}
}

func (v *VideoCapture) connect() error {
	cap, err := gocv.VideoCaptureFile(v.URI)
	if err != nil {
		return err
	}

	m := gocv.NewMat()
	defer m.Close()

	if ok := cap.Read(&m); !ok {
		return fmt.Errorf("Failed to grab test image")
	}

	v.lastFetch = time.Now()
	v.sz = image.Point{X: m.Cols(), Y: m.Rows()}
	v.cap = cap

	v.init.Notify()
	return nil
}

func (v *VideoCapture) Get() <-chan Image {
	c := make(chan Image)
	go func() {
		// Init a stopped timer.
		t := time.NewTimer(time.Second)
		if !t.Stop() {
			<-t.C
		}

		// TODO clean up this somewhat horrifying state machine.

		for {
			start := time.Now()
			d := time.Second / time.Duration(v.FPS)

			if v.Connected() {
				i := NewImage()
				if ok := v.cap.Read(&i.Mat); ok {
					c <- i

					v.lastFetch = i.Time
				} else {
					// TODO remove
					log.Warning("Failed read from capture source")
					i.Close()
				}

				if time.Now().After(v.lastFetch.Add(disconnectDelay)) {
					v.cap.Close()
					v.cap = nil
					log.Errorf("Closed capture source %s due to no frame for %.2f seconds", v.URI, disconnectDelay.Seconds())
				}
			} else {
				log.Infof("Attempting connection to capture source %s", v.URI)
				if err := v.connect(); err != nil {
					log.Errorf("Connection to %s failed: %v", v.URI, err)
					d = retryDelay
				} else {
					log.Infof("Connected to %s, resolution %dx%d", v.URI, v.sz.X, v.sz.Y)
				}
			}

			d -= time.Now().Sub(start)
			if d > 0 {
				t.Reset(d)
			} else {
				continue
			}

			select {
			case <-t.C:
				continue
			case c := <-v.close:
				if v.cap != nil {
					v.cap.Close()
				}
				c <- true
				return
			}
		}
	}()
	return c
}

// Size gets the size of the capture source. This will block until the capture source is initially opened.
func (v *VideoCapture) Size() image.Point {
	v.init.Wait()
	return v.sz
}

func (v *VideoCapture) Connected() bool {
	return v.cap != nil
}

// TODO this can deadlock if the caller stops reading from the Get stream first.
func (v *VideoCapture) Close() {
	c := make(chan bool)
	v.close <- c
	<-c
}
