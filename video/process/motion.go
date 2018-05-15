package process

import (
	"cam/video/sink"
	log "github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
	"image"
	"image/color"
	"time"
)

type Triggerable interface {
	// Indicates that motion has been triggered.
	Trigger()
}

var (
	// Be blind to motion for this amount of time to avoid detections when
	// starting.
	StartupTimeout = 10 * time.Second
)

// TODO: CPU usage is too high, FPS limiting would help (FINISH)
// TODO: option to disable shadow elimination?

type Motion struct {
	// If set, will be triggered when motion is above the threshold.
	Trigger Triggerable

	BlendRatio float64

	AnalysisFPS int

	lastFrame time.Time

	// Channel for incoming images.
	c     chan gocv.Mat
	mjpeg *sink.MJPEGServer

	// Channel for double buffering.
	a chan gocv.Mat

	d gocv.BackgroundSubtractorMOG2

	blend, blendin, draw, m0, m1, m2, m3, st1, sts, stl, mask gocv.Mat

	crop image.Rectangle
}

func NewMotion(ms *sink.MJPEGServer, sz image.Point) *Motion {
	mask := gocv.NewMatWithSize(sz.Y, sz.X, gocv.MatTypeCV8UC3)
	gocv.Rectangle(&mask, image.Rectangle{Min: image.Point{}, Max: sz}, color.RGBA{0, 0, 0, 0}, -1)

	bounds := []image.Point{{600, 550}, {1300, 550}, {1900, 1050}, {120, 1050}}
	gocv.FillPoly(&mask, [][]image.Point{bounds}, color.RGBA{255, 255, 255, 255})

	// TODO fill the mask.

	m := &Motion{
		c:     make(chan gocv.Mat),
		mjpeg: ms,

		a: make(chan gocv.Mat, 2),

		BlendRatio: 0.38,

		// Slow down analysis to limit CPU usage.
		AnalysisFPS: 1,

		// history=500, threshold=16
		// TODO: make history based on analysis FPS.
		d: gocv.NewBackgroundSubtractorMOG2(60, 16),

		blend:   gocv.NewMat(),
		blendin: gocv.NewMat(),
		draw:    gocv.NewMat(),
		m0:      gocv.NewMat(),
		m1:      gocv.NewMat(),
		m2:      gocv.NewMat(),
		m3:      gocv.NewMat(),

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

	motionEnabled := false
	time.AfterFunc(StartupTimeout, func() {
		log.Infof("Now watching for motion.")
		motionEnabled = true
	})

	first := true

	for input := range m.c {
		s := time.Now()

		if first {
			input.CopyTo(m.blend)
			first = false
		} else {
			gocv.AddWeighted(input, m.BlendRatio, m.blend, 1-m.BlendRatio, 0.0, &m.blend)
		}

		// TODO draw on source image, or expose bounding rects.
		m.blend.CopyTo(m.blendin)

		// TODO: limit FPS here.

		if s.Sub(m.lastFrame) < time.Second/time.Duration(m.AnalysisFPS) {
			// Return frame to available pool.
			// TODO refactor...
			m.a <- input
			// Skip frame processing
			continue
		}
		m.lastFrame = s

		debug.Put("blended", m.blendin)

		m.blendin.CopyTo(m.draw)

		debug.Put("mask", m.mask)

		gocv.BitwiseAnd(m.blendin, m.mask, &m.blendin)
		debug.Put("masked", m.blendin)

		inputcrop := m.blendin.Region(m.crop)
		debug.Put("cropped", inputcrop)

		//gocv.MedianBlur(input, m.m1, 10)
		//gocv.Blur(inputcrop, m.m1, image.Point{X: 10, Y: 10})
		//debug.Put("blurred", m.m1)

		//m.d.Apply(m.m1, m.m2)
		m.d.Apply(inputcrop, &m.m2)
		debug.Put("motion", m.m2)

		// was 128
		// day: 128
		// night: 1
		// Maybe be smart and turn this on only at night?
		gocv.Threshold(m.m2, &m.m3, 128, 255, gocv.ThresholdBinary)
		debug.Put("motionthresh", m.m3)

		// TODO separate
		gocv.Erode(m.m3, &m.m3, m.sts)
		debug.Put("erode", m.m3)
		gocv.Dilate(m.m3, &m.m3, m.stl)
		debug.Put("dilate", m.m3)

		contours := gocv.FindContours(m.m3, gocv.RetrievalList, gocv.ChainApproxSimple)
		for _, contour := range contours {
			bounds := gocv.BoundingRect(contour)
			gocv.Rectangle(&m.draw, bounds.Add(m.crop.Min), color.RGBA{255, 0, 0, 255}, 2)
		}

		if motionEnabled && len(contours) > 0 {
			// TODO make this a metrics stream.
			log.Debugf("Detected motion, %d contours", len(contours))
			if m.Trigger != nil {
				m.Trigger.Trigger()
			}
		}

		debug.Put("motiondraw", m.draw)

		// TODO: export this as a streaming stat.
		// log.Printf("Elapsed: %v", time.Now().Sub(s))

		// Return image to the available pool.
		m.a <- input
	}
}

// TODO make this take Image so it's time aware.
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
