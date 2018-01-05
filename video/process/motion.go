package process

import (
	"cam/video/sink"
	"gocv.io/x/gocv"
	"image"
	"log"
	"time"
)

type Motion struct {
	// Channel for incoming images.
	c     chan gocv.Mat
	mjpeg *sink.MJPEGServer

	// Channel for double buffering.
	a chan gocv.Mat

	d gocv.BackgroundSubtractorMOG2

	m1, m2, st gocv.Mat
}

func NewMotion(ms *sink.MJPEGServer) *Motion {
	m := &Motion{
		c:     make(chan gocv.Mat),
		mjpeg: ms,

		a: make(chan gocv.Mat, 2),

		d: gocv.NewBackgroundSubtractorMOG2(),

		m1: gocv.NewMat(),
		m2: gocv.NewMat(),

		// TODO allow reconfiguring structring element.
		st: gocv.GetStructuringElement(gocv.MorphRect, image.Point{X: 5, Y: 5}),
	}

	// Fill mat buffer.
	m.a <- gocv.NewMat()
	m.a <- gocv.NewMat()

	go m.loop()
	return m
}

func (m *Motion) loop() {
	motionview := m.mjpeg.NewStream(sink.MJPEGID{Name: "motion"})
	defer motionview.Close()

	for input := range m.c {

		s := time.Now()
		//gocv.Blur(input.Mat, m.m1, image.Point{X: 5, Y: 5})

		gocv.MedianBlur(input, m.m1, 5)
		m.d.Apply(m.m1, m.m2)

		// TODO separate
		gocv.Erode(m.m2, m.m2, m.st)
		gocv.Dilate(m.m2, m.m2, m.st)

		log.Printf("Elapsed: %v", time.Now().Sub(s))

		motionview.Put(m.m2)

		m.a <- input
	}
}

func (m *Motion) Process(input gocv.Mat) {
	mat := <-m.a
	input.CopyTo(mat)

	select {
	case m.c <- mat:
	default:
		// Allow skipping frames if already processing.
		m.a <- mat
	}
}
