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

	m1, m2, m3, st1, st3 gocv.Mat
}

func NewMotion(ms *sink.MJPEGServer) *Motion {
	m := &Motion{
		c:     make(chan gocv.Mat),
		mjpeg: ms,

		a: make(chan gocv.Mat, 2),

		d: gocv.NewBackgroundSubtractorMOG2(),

		m1: gocv.NewMat(),
		m2: gocv.NewMat(),
		m3: gocv.NewMat(),

		// TODO allow reconfiguring structring element.
		st1: gocv.GetStructuringElement(gocv.MorphCross, image.Point{X: 1, Y: 1}),
		st3: gocv.GetStructuringElement(gocv.MorphCross, image.Point{X: 3, Y: 3}),
	}

	// Fill mat buffer.
	m.a <- gocv.NewMat()
	m.a <- gocv.NewMat()

	go m.loop()
	return m
}

func (m *Motion) loop() {

	debug := m.mjpeg.NewStreamPool()
	defer debug.Close()

	for input := range m.c {

		s := time.Now()

		//gocv.MedianBlur(input, m.m1, 10)
		gocv.Blur(input, m.m1, image.Point{X: 10, Y: 10})

		m.d.Apply(m.m1, m.m2)
		debug.Put("motion", m.m2)

		gocv.Threshold(m.m2, m.m3, 128, 255, gocv.ThresholdBinary)
		debug.Put("motionthresh", m.m3)

		// TODO separate
		gocv.Erode(m.m3, m.m3, m.st3)
		debug.Put("motionfilter", m.m3)
		//gocv.Dilate(m.m2, m.m2, m.st)

		log.Printf("Elapsed: %v", time.Now().Sub(s))

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
