package source

import (
	log "github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
)

type MatPool struct {
	new   chan chan gocv.Mat
	free  chan gocv.Mat
	close chan bool

	allocated int
	available []gocv.Mat
}

func NewMatPool() *MatPool {
	p := &MatPool{
		new:   make(chan chan gocv.Mat),
		free:  make(chan gocv.Mat),
		close: make(chan bool),
	}
	go func() {
		closed := false
		for {
			select {
			case <-p.close:
				closed = true
				for _, m := range p.available {
					m.Close()
					p.allocated -= 1
				}
				p.available = []gocv.Mat{}
			case m := <-p.free:
				if closed {
					m.Close()
					p.allocated -= 1
				} else {
					p.available = append(p.available, m)
				}
			case r := <-p.new:
				var m gocv.Mat
				if len(p.available) > 0 {
					m, p.available = p.available[0], p.available[1:]
				} else {
					m = gocv.NewMat()
					p.allocated += 1
					// TODO clean; tie max size to buffer.
					// TODO start blocking callers instead (supports the file dump case).
					if p.allocated > 500 {
						log.Fatalf("Too many MatPool allocations. Perhaps an Image isn't being released?")
					}
				}
				r <- m
			}
		}
	}()
	return p
}

func (p *MatPool) NewMat() gocv.Mat {
	r := make(chan gocv.Mat)
	p.new <- r
	return <-r
}

func (p *MatPool) ReleaseMat(m gocv.Mat) {
	p.free <- m
}

func (p *MatPool) Close() {
	p.close <- true
}
