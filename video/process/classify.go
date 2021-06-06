package process

import (
	"fmt"
	"image"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"
	"gocv.io/x/gocv"

	"cam/video/sink"
)

// ColorThresh denotes the minimum value for an image to be considered color.
const ColorThresh = 15

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

	diff     gocv.Mat
	diffBlur gocv.Mat

	enabled bool
	l       sync.Mutex
}

func NewClassifier(prototxt, caffeModel []byte) *Classifier {
	net, err := gocv.ReadNetFromCaffeBytes(prototxt, caffeModel)
	if err != nil {
		log.Fatalf("Failed to read caffe model to net: %v", err)
	}
	return &Classifier{
		net:      net,
		small:    gocv.NewMat(),
		diff:     gocv.NewMat(),
		diffBlur: gocv.NewMat(),
	}
}

type Detections map[string]float32

func (d Detections) Merge(other Detections) {
	for k, v := range other {
		if d[k] < v {
			d[k] = v
		}
	}
}

type Detection struct {
	Class      string
	Confidence float32
}

func (d Detections) SortedDetections() []Detection {
	var ss []Detection
	for k, v := range d {
		ss = append(ss, Detection{k, v})
	}
	sort.Slice(ss, func(i, j int) bool {
		return ss[i].Confidence > ss[j].Confidence
	})
	return ss
}

func (d Detections) DebugString() string {
	var ds []string
	for _, kv := range d.SortedDetections() {
		ds = append(ds, fmt.Sprintf("%s: %.2f", kv.Class, kv.Confidence))
	}
	return strings.Join(ds, ", ")
}

func (cl *Classifier) ImageColorValue(input gocv.Mat) float32 {
	channels := gocv.Split(input)
	defer func() {
		for _, v := range channels {
			v.Close()
		}
	}()
	gocv.AbsDiff(channels[1], channels[2], &cl.diff)
	gocv.Blur(cl.diff, &cl.diffBlur, image.Point{X: 10, Y: 10})
	_, maxDiff, _, _ := gocv.MinMaxIdx(cl.diffBlur)
	return maxDiff
}

func (cl *Classifier) Classify(input gocv.Mat, debug *sink.MJPEGStreamPool) Detections {
	if !cl.isEnabled() {
		return nil
	}

	start := time.Now()
	defer func() {
		// TODO export this as a streaming stat.
		log.Debugf("Classifier ran in %v", time.Now().Sub(start).String())
	}()
	output := make(Detections)

	scale := image.Point{X: 300, Y: 300}
	gocv.Resize(input, &cl.small, scale, 0, 0, gocv.InterpolationLinear)

	if diff := cl.ImageColorValue(cl.small); diff < ColorThresh {
		// Refuse to classify grayscale.
		log.Debugf("Refusing to classify grayscale image with color value %f", diff)
		return output
	}

	blob := gocv.BlobFromImage(cl.small, 0.007843, scale, gocv.NewScalar(127.5, 127.5, 127.5, 0), false, false)
	defer blob.Close()

	cl.net.SetInput(blob, "data")

	detBlob := cl.net.Forward("detection_out")
	defer detBlob.Close()

	detections := gocv.GetBlobChannel(detBlob, 0, 0)
	defer detections.Close()

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

		if output[class] < confidence {
			output[class] = confidence
		}
	}
	return output
}

func (cl *Classifier) Enable() {
	cl.l.Lock()
	defer cl.l.Unlock()
	cl.enabled = true
	log.Infof("Classifier enabled")
}

func (cl *Classifier) Disable() {
	cl.l.Lock()
	defer cl.l.Unlock()
	cl.enabled = false
	log.Infof("Classifier disabled")
}

func (cl *Classifier) isEnabled() bool {
	cl.l.Lock()
	defer cl.l.Unlock()
	return cl.enabled
}
