package process

import (
	log "github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
	"image"
	"time"
)

// Detection classes for MobileNet SSD
var mobileNetClasses = map[int]string{
	0: "background",
	1: "aeroplane", 2: "bicycle", 3: "bird", 4: "boat",
	5: "bottle", 6: "bus", 7: "car", 8: "cat", 9: "chair",
	10: "cow", 11: "diningtable", 12: "dog", 13: "horse",
	14: "motorbike", 15: "person", 16: "pottedplant",
	17: "sheep", 18: "sofa", 19: "train", 20: "tvmonitor",
}

// Mapping from a class returned by mobilenet to desired output class.
var mobileNetRemap = map[string]string{
	"bicycle":   "person",
	"person":    "person",
	"bus":       "vehicle",
	"car":       "vehicle",
	"motorbike": "vehicle",
	"train":     "vehicle",
	"cat":       "animal",
	"cow":       "animal",
	"dog":       "animal",
	"horse":     "animal",
	"sheep":     "animal",
}

type Classifier struct {
	net gocv.Net

	// Resized 300x300 image for image classification.
	small gocv.Mat
}

func NewClassifier(prototxt, caffeModel []byte) *Classifier {
	return &Classifier{
		net:   gocv.ReadNetFromCaffeBytes(prototxt, caffeModel),
		small: gocv.NewMat(),
	}
}

type Detection struct {
	Class      string
	Confidence float32
}

func (cl *Classifier) Classify(input gocv.Mat) *Detection {
	start := time.Now()
	defer func() {
		// TODO export this as a streaming stat.
		log.Debugf("Classifier ran in %v", time.Now().Sub(start).String())
	}()

	scale := image.Point{X: 300, Y: 300}
	gocv.Resize(input, &cl.small, scale, 0, 0, gocv.InterpolationLinear)

	blob := gocv.BlobFromImage(cl.small, 0.007843, scale, gocv.NewScalar(127.5, 127.5, 127.5, 0), false, false)
	defer blob.Close()

	cl.net.SetInput(blob, "data")

	detBlob := cl.net.Forward("detection_out")
	defer detBlob.Close()

	detections := gocv.GetBlobChannel(detBlob, 0, 0)
	defer detections.Close()

	var detection *Detection

	for r := 0; r < detections.Rows(); r++ {
		classID := int(detections.GetFloatAt(r, 1))
		classMn := mobileNetClasses[classID]
		class := mobileNetRemap[classMn]
		if class == "" {
			continue
		}

		confidence := detections.GetFloatAt(r, 2)
		if confidence < 0.5 {
			continue
		}

		left := int(detections.GetFloatAt(r, 3) * float32(input.Cols()))
		top := int(detections.GetFloatAt(r, 4) * float32(input.Rows()))
		right := int(detections.GetFloatAt(r, 5) * float32(input.Cols()))
		bottom := int(detections.GetFloatAt(r, 6) * float32(input.Rows()))
		log.Debugf("Detection of %s (%s) at (%d, %d, %d, %d), confidence %.2f", class, classMn, left, top, right, bottom, confidence)

		// TODO render a nice debug view!

		if detection == nil || confidence > detection.Confidence {
			detection = &Detection{
				Class:      class,
				Confidence: confidence,
			}
		}
	}
	return detection
}