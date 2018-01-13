package video

import (
	"time"

	"cam/video/sink"
	"cam/video/source"
)

type Buffer struct {
	MaxAge time.Duration

	// buffer contains image history, oldest first.
	buffer []source.Image
	pool   *source.MatPool

	input    chan source.Image
	close    chan chan bool
	flush    chan sink.Sink
	flushack chan bool
}

func NewBuffer(maxAge time.Duration) *Buffer {
	b := &Buffer{
		MaxAge: maxAge,
		pool:   source.NewMatPool(),

		input:    make(chan source.Image),
		close:    make(chan chan bool),
		flush:    make(chan sink.Sink),
		flushack: make(chan bool),
	}
	go func() {
		for {
			select {
			case in := <-b.input:
				// Add to buffer tail.
				b.buffer = append(b.buffer, in)
				// Clear out old images from head.
				for i, img := range b.buffer {
					if in.Time.Sub(img.Time) >= b.MaxAge {
						b.pool.ReleaseMat(img.Mat)
					} else {
						b.buffer = b.buffer[i:]
						break
					}
				}
			case sink := <-b.flush:
				for _, img := range b.buffer {
					sink.Put(img)
				}
				b.flushack <- true
			case c := <-b.close:
				for _, img := range b.buffer {
					b.pool.ReleaseMat(img.Mat)
				}
				c <- true
				return
			}
		}
	}()
	return b
}

func (b *Buffer) Put(input source.Image) {
	m := b.pool.NewMat()
	input.Mat.CopyTo(m)
	i := source.Image{
		Mat:  m,
		Time: input.Time,
	}
	b.input <- i
}

func (b *Buffer) FlushToSink(sink sink.Sink) {
	b.flush <- sink
	<-b.flushack
}

func (b *Buffer) Close() {
	c := make(chan bool)
	b.close <- c
	<-c
}
