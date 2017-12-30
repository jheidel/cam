package sink

import (
	"cam/video/source"
	"gocv.io/x/gocv"
)

type Window struct {
	window  *gocv.Window
	sizeSet bool
}

func NewWindow(name string) *Window {
	return &Window{
		window: gocv.NewWindow(name),
	}
}

func (w *Window) Put(input source.Image) {
	if !w.sizeSet {
		w.window.ResizeWindow(input.Mat.Cols(), input.Mat.Rows())
		w.sizeSet = true
	}
	w.window.IMShow(input.Mat)
	w.window.WaitKey(1)
}

func (w *Window) Close() {
	w.window.Close()
}
