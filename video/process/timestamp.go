package process

import (
	"cam/video/source"
	"gocv.io/x/gocv"
	"image"
	"image/color"
)

var (
	colorTime = color.RGBA{R: 255, G: 255, B: 255, A: 255}
	colorBG   = color.RGBA{R: 0, G: 0, B: 0, A: 255}
)

// Timestamp draws the current time on the given image.
func DrawTimestamp(name string, img source.Image) source.Image {

	// TODO use a stftime package instead of this insanity?
	// https://stackoverflow.com/questions/20234104/how-to-format-current-time-using-a-yyyymmddhhmmss-format
	text := name + " - " + img.Time.Format("2006-01-02 15:04:05 MST")

	font := gocv.FontHersheySimplex
	scale := 0.5
	thickness := 1

	sz := gocv.GetTextSize(text, font, scale, thickness)

	pad := 2

	gocv.Rectangle(img.Mat, image.Rectangle{Min: image.Point{X: 0, Y: 0}, Max: image.Point{X: sz.X + pad*2, Y: sz.Y + pad*2}}, colorBG, -1)

	gocv.PutText(img.Mat, text, image.Point{X: pad, Y: sz.Y + pad}, font, scale, colorTime, thickness)

	return img
}
