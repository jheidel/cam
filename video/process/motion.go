package process

import (
	"cam/video/sink"
	"gocv.io/x/gocv"
	"image"
	"image/color"
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

	draw, m0, m1, m2, m3, st1, sts, stl, mask gocv.Mat

	crop image.Rectangle
}

func NewMotion(ms *sink.MJPEGServer, sz image.Point) *Motion {
	mask := gocv.NewMatWithSize(sz.Y, sz.X, gocv.MatTypeCV8UC3)
	gocv.Rectangle(mask, image.Rectangle{Min: image.Point{}, Max: sz}, color.RGBA{0, 0, 0, 0}, -1)

	bounds := []image.Point{{600, 550}, {1300, 550}, {1900, 1050}, {120, 1050}}
	gocv.FillPoly(mask, bounds, color.RGBA{255, 255, 255, 255})

	// TODO fill the mask.

	m := &Motion{
		c:     make(chan gocv.Mat),
		mjpeg: ms,

		a: make(chan gocv.Mat, 2),

		d: gocv.NewBackgroundSubtractorMOG2(),

		draw: gocv.NewMat(),
		m0:   gocv.NewMat(),
		m1:   gocv.NewMat(),
		m2:   gocv.NewMat(),
		m3:   gocv.NewMat(),

		mask: mask,
		crop: gocv.BoundingRect(bounds),

		// TODO allow reconfiguring structring element.
		sts: gocv.GetStructuringElement(gocv.MorphCross, image.Point{X: 10, Y: 10}),
		stl: gocv.GetStructuringElement(gocv.MorphEllipse, image.Point{X: 30, Y: 30}),
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

		input.CopyTo(m.draw)

		debug.Put("mask", m.mask)

		gocv.BitwiseAnd(input, m.mask, input)
		debug.Put("masked", input)

		inputcrop := input.Region(m.crop)
		debug.Put("cropped", inputcrop)

		//gocv.MedianBlur(input, m.m1, 10)
		//gocv.Blur(inputcrop, m.m1, image.Point{X: 10, Y: 10})
		//debug.Put("blurred", m.m1)

		//m.d.Apply(m.m1, m.m2)
		m.d.Apply(inputcrop, m.m2)
		debug.Put("motion", m.m2)

		gocv.Threshold(m.m2, m.m3, 128, 255, gocv.ThresholdBinary)
		debug.Put("motionthresh", m.m3)

		// TODO separate
		gocv.Erode(m.m3, m.m3, m.sts)
		debug.Put("erode", m.m3)
		gocv.Dilate(m.m3, m.m3, m.stl)
		debug.Put("dilate", m.m3)

		contours := gocv.FindContours(m.m3, gocv.RetrievalList, gocv.ChainApproxSimple)
		for _, contour := range contours {
			bounds := gocv.BoundingRect(contour)
			gocv.Rectangle(m.draw, bounds.Add(m.crop.Min), color.RGBA{255, 0, 0, 255}, 2)
		}

		debug.Put("motiondraw", m.draw)

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
