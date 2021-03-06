package process

import (
	"image"
	"image/color"
	"time"

	"cam/config"
	"cam/util"
	"cam/video/sink"

	log "github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
)

type MotionTriggerable interface {
	// Indicates that motion has been triggered.
	MotionDetected()

	MotionClassified(d Detections)
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
	Triggers []MotionTriggerable

	BlendRatio float64

	AnalysisFPS int

	lastFrame time.Time

	// Channel for incoming images.
	c     chan gocv.Mat
	mjpeg *sink.MJPEGServer

	// Channel for double buffering.
	a chan gocv.Mat

	subtractor gocv.BackgroundSubtractorMOG2

	blend, blendin, draw, m0, m1, m2, m3, st1, sts, stl, mask gocv.Mat

	crop image.Rectangle

	classifier *Classifier
}

func NewMotion(ms *sink.MJPEGServer, classifier *Classifier, sz image.Point) *Motion {
	mask := gocv.NewMatWithSize(sz.Y, sz.X, gocv.MatTypeCV8UC3)
	gocv.Rectangle(&mask, image.Rectangle{Min: image.Point{}, Max: sz}, color.RGBA{0, 0, 0, 0}, -1)

	// TODO support live reload of mask
	bounds := config.Get().MotionBounds
	gocv.FillPoly(&mask, gocv.NewPointsVectorFromPoints([][]image.Point{bounds}), color.RGBA{255, 255, 255, 255})

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
		subtractor: gocv.NewBackgroundSubtractorMOG2WithParams(60, config.Get().MotionThresh, false),

		blend:   gocv.NewMat(),
		blendin: gocv.NewMat(),
		draw:    gocv.NewMat(),
		m0:      gocv.NewMat(),
		m1:      gocv.NewMat(),
		m2:      gocv.NewMat(),
		m3:      gocv.NewMat(),

		mask: mask,
		crop: gocv.BoundingRect(gocv.NewPointVectorFromPoints(bounds)),

		// TODO allow reconfiguring structring element.
		sts: gocv.GetStructuringElement(gocv.MorphCross, image.Point{
			X: config.Get().MotionErode,
			Y: config.Get().MotionErode,
		}),
		stl: gocv.GetStructuringElement(gocv.MorphEllipse, image.Point{X: 30, Y: 30}),

		classifier: classifier,
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

	motionEnabled := util.NewEvent()
	time.AfterFunc(StartupTimeout, func() {
		log.Infof("Now watching for motion.")
		motionEnabled.Notify()
	})

	first := true

	for input := range m.c {
		s := time.Now()

		if first {
			input.CopyTo(&m.blend)
			first = false
		} else {
			gocv.AddWeighted(input, m.BlendRatio, m.blend, 1-m.BlendRatio, 0.0, &m.blend)
		}

		// TODO draw on source image, or expose bounding rects.
		m.blend.CopyTo(&m.blendin)

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

		m.blendin.CopyTo(&m.draw)

		debug.Put("mask", m.mask)

		gocv.BitwiseAnd(m.blendin, m.mask, &m.blendin)
		debug.Put("masked", m.blendin)

		inputcrop := m.blendin.Region(m.crop)
		debug.Put("cropped", inputcrop)

		//gocv.MedianBlur(input, m.m1, 10)
		//gocv.Blur(inputcrop, m.m1, image.Point{X: 10, Y: 10})
		//debug.Put("blurred", m.m1)

		//m.d.Apply(m.m1, m.m2)
		m.subtractor.Apply(inputcrop, &m.m2)
		inputcrop.Close()

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
		for i := 0; i < contours.Size(); i++ {
			contour := contours.At(i)
			bounds := gocv.BoundingRect(contour)
			gocv.Rectangle(&m.draw, bounds.Add(m.crop.Min), color.RGBA{255, 0, 0, 255}, 2)
		}

		sz := contours.Size()
		if motionEnabled.HasBeenNotified() && sz > 0 {
			// TODO make this a metrics stream.
			log.Debugf("Detected motion, %d contours", sz)
			for _, t := range m.Triggers {
				t.MotionDetected()
			}
		}

		debug.Put("motiondraw", m.draw)

		// Run classification (note that this will only produce results if the
		// classifier has been enabled)
		if d := m.classifier.Classify(input, debug); len(d) > 0 {
			log.Infof("Classifier had detection results: %v", d.DebugString())
			for _, t := range m.Triggers {
				t.MotionClassified(d)
			}
		}

		// TODO: export this as a streaming stat.
		// log.Printf("Elapsed: %v", time.Now().Sub(s))

		// Return image to the available pool.
		m.a <- input
	}
}

// TODO make this take Image so it's time aware.
func (m *Motion) Process(input gocv.Mat) {
	mat := <-m.a
	input.CopyTo(&mat)

	select {
	case m.c <- mat:
	default:
		// Allow skipping frames if already processing.
		m.a <- mat
	}
}
