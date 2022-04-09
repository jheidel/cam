package process

import (
	"cam/video/source"
	"gocv.io/x/gocv"
	"image"
	"io/ioutil"
)

func WriteThumb(path string, input source.Image) error {
	tmat := gocv.NewMat()
	defer tmat.Close()
	gocv.Resize(input.Mat, &tmat, image.Point{X: 320, Y: 180}, 0, 0, gocv.InterpolationArea)

	jpeg, err := gocv.IMEncode(".jpg", tmat)
	if err != nil {
		return err
	}

	if err := ioutil.WriteFile(path, jpeg.GetBytes(), 0644); err != nil {
		return err
	}

	return nil
}
