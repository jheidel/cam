package source

import (
	"fmt"
	"gocv.io/x/gocv"
	"image"
	"log"
	"time"
)

const (
	// maxFPS is an upper bound on the frames rate to receive to avoid spinning in
	// a tight loop.
	maxFPS = 100

	// retryDelay defines the time to wait between reconnection attempts.
	retryDelay = 2 * time.Second

	// disconnectDelay defines the threshold where a reconnect should be attempted.
	disconnectDelay = 5 * time.Second
)

type VideoCapture struct {
	URI string

	initC chan bool
	init  bool
	close chan chan bool

	cap       *gocv.VideoCapture
	sz        image.Point
	lastFetch time.Time

	pool *MatPool
}

// NewVideoCapture opens a capture source from the provided URI. It supports
// any format compatible with OpenCV. Assuming FFmpeg is compiled correctly,
// this includes support for MJPEG and RSTP IP cameras.
func NewVideoCapture(uri string) *VideoCapture {
	return &VideoCapture{
		URI:   uri,
		initC: make(chan bool, 1),
		close: make(chan chan bool, 1),
		pool:  NewMatPool(),
	}
}

func (v *VideoCapture) waitInit() {
	if !v.init {
		<-v.initC
		v.initC <- true
	}
}

func (v *VideoCapture) setInit() {
	if !v.init {
		v.init = true
		v.initC <- true // Unblock anyone waiting on init.
	}
}

func (v *VideoCapture) connect() error {
	cap, err := gocv.VideoCaptureFile(v.URI)
	if err != nil {
		return err
	}

	m := gocv.NewMat()
	defer m.Close()

	if ok := cap.Read(m); !ok {
		return fmt.Errorf("Failed to grab test image")
	}

	v.lastFetch = time.Now()
	v.sz = image.Point{X: m.Cols(), Y: m.Rows()}
	v.cap = cap

	v.setInit()
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

		m := v.pool.NewMat()
		for {
			start := time.Now()
			d := time.Second / time.Duration(maxFPS)

			if v.Connected() {
				i := Image{
					Mat:  m,
					Time: time.Now(),
					pool: v.pool,
				}
				if ok := v.cap.Read(i.Mat); ok {
					c <- i

					v.lastFetch = i.Time
					m = v.pool.NewMat()
				}

				if v.lastFetch.After(time.Now().Add(disconnectDelay)) {
					v.cap.Close()
					v.cap = nil
					log.Printf("Closed capture source %s due to no frame for %d seconds", v.URI, disconnectDelay.Seconds())
				}
			} else {
				log.Printf("Attempting connection to capture source %s", v.URI)
				if err := v.connect(); err != nil {
					log.Printf("Connection to %s failed: %v", v.URI, err)
					d = retryDelay
				} else {
					log.Printf("Connected to %s, resolution %dx%d", v.URI, v.sz.X, v.sz.Y)
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
				v.pool.ReleaseMat(m)
				c <- true
				return
			}
		}
	}()
	return c
}

// Size gets the size of the capture source. This will block until the capture source is initially opened.
func (v *VideoCapture) Size() image.Point {
	v.waitInit()
	return v.sz
}

func (v *VideoCapture) Connected() bool {
	return v.cap != nil
}

func (v *VideoCapture) Close() {
	c := make(chan bool)
	v.close <- c
	<-c
}
