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
	getLast  chan chan source.Image
}

func NewBuffer(maxAge time.Duration) *Buffer {
	b := &Buffer{
		MaxAge: maxAge,
		pool:   source.NewMatPool(),

		input:    make(chan source.Image),
		close:    make(chan chan bool),
		flush:    make(chan sink.Sink),
		flushack: make(chan bool),
		getLast:  make(chan chan source.Image),
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
			case c := <-b.getLast:
				last := b.buffer[len(b.buffer)-1]
				c <- last.Clone()
			}
		}
	}()
	return b
}

func (b *Buffer) Put(input source.Image) {
	b.input <- input.CloneToPool(b.pool)
}

// GetLast returns a copy of the image. The caller must release it.
func (b *Buffer) GetLast() source.Image {
	c := make(chan source.Image)
	b.getLast <- c
	return <-c
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
