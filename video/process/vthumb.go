package process

import (
	"log"
	"os"
	"os/exec"

	"cam/util"
)

const (
	ExtTemp = ".temp"
)

type VThumbProducer struct {
	c     chan *workItem
	close chan chan bool
}

type workItem struct {
	src, dst string
	donec    chan bool
}

func NewVThumbProducer() *VThumbProducer {
	f := &VThumbProducer{
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
				util.LocateFFmpegOrDie(),
				// Configure input from source file.
				"-i", w.src,
				// Thumbnails can be choppy to reduce size.
				"-r", "3",
				// Output format as libx264
				"-c:v", "libx264",
				// Speed up video and resize to thumbnail size.
				"-vf", "setpts=0.1*PTS,scale=320:180",
				// Fast, fairly low quality.
				"-preset", "fast",
				"-crf", "28",
				// Keep CPU usage down. Thumbnail conversion doesn't need to be fast.
				"-threads", "1",
				// Limit duration to 5s (trim)
				"-t", "5",
				// Allow playback on a wider range of devices.
				"-pix_fmt", "yuv420p",
				"-profile:v", "baseline",
				"-level", "3.0",
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
				w.donec <- true
			}
		}
	}()
	return f
}

func (f *VThumbProducer) Process(src, dst string) <-chan bool {
	w := &workItem{
		src:   src,
		dst:   dst,
		donec: make(chan bool),
	}
	select {
	case f.c <- w:
	default:
		log.Printf("WARN: thumbnail processing dropped due to backlog")
		return nil
	}
	return w.donec
}

func (f *VThumbProducer) Close() {
	c := make(chan bool)
	f.close <- c
	<-c
}
