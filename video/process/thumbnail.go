package process

import (
	"log"
	"os"
	"os/exec"
)

const (
	ExtTemp = ".temp"
)

type ThumbnailProducer struct {
	c     chan *workItem
	close chan chan bool
}

type workItem struct {
	src, dst string
}

func NewThumbnailProducer() *ThumbnailProducer {
	f := &ThumbnailProducer{
		c:     make(chan *workItem, 100),
		close: make(chan chan bool, 1),
	}
	go func() {
		for {
			var w *workItem
			select {
			case cc := <-f.close:
				cc <- true
				return
			case w = <-f.c:
			}

			c := exec.Command(
				"/usr/local/bin/ffmpeg",
				// Configure input from source file.
				"-i", w.src,
				// Thumbnails can be choppy to reduce size.
				"-r", "3",
				// Output format as libx264
				"-c:v", "libx264",
				// Speed up video and resize to thumbnail size.
				"-vf", "setpts=0.2*PTS,scale=240:135",
				// Fast, fairly low quality.
				"-preset", "ultrafast",
				"-crf", "30",
				// Keep CPU usage down. Thumbnail conversion doesn't need to be fast.
				"-threads", "1",
				// Explicit format.
				"-f", "mp4",
				w.dst+ExtTemp,
			)

			// Allows for debugging ffmpeg in shell.
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr

			if err := c.Start(); err != nil {
				log.Printf("Failed to start thumbnail conversion for %v: %v", w, err)
				continue
			}

			wait := make(chan error)
			go func() {
				wait <- c.Wait()
			}()

			select {
			case cc := <-f.close:
				c.Process.Kill()
				cc <- true
				return
			case err := <-wait:
				// TODO clean this up.
				if err == nil {
					if err := os.Rename(w.dst+ExtTemp, w.dst); err != nil {
						log.Printf("Error moving thumbnail to its final destination")
					} else {
						log.Printf("Thumbnail conversion succeeded for %v", w)
					}
				} else {
					log.Printf("Thumbnail conversion failed for %v: %v", w, err)
				}
			}
		}
	}()
	return f
}

func (f *ThumbnailProducer) Process(src, dst string) {
	w := &workItem{
		src: src,
		dst: dst,
	}
	select {
	case f.c <- w:
	default:
		log.Printf("WARN: thumbnail processing dropped due to backlog")
	}
}

func (f *ThumbnailProducer) Close() {
	c := make(chan bool)
	f.close <- c
	<-c
}
