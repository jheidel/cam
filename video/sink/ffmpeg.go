package sink

import (
	"cam/video/source"
	"fmt"
	"log"
	"os"
	"os/exec"
)

type FFMpegSink struct {
	b     chan []byte
	close chan chan bool
}

func NewFFMpegSink(path string, fps, width, height int) *FFMpegSink {
	f := &FFMpegSink{
		b:     make(chan []byte),
		close: make(chan chan bool),
	}
	go func() {
		c := exec.Command(
			"/usr/local/bin/ffmpeg",
			// Configure ffmpeg to read from the opencv pipe.
			"-f", "rawvideo",
			"-pixel_format", "bgr24",
			"-video_size", fmt.Sprintf("%dx%d", width, height),
			"-framerate", fmt.Sprintf("%d", fps),
			"-i", "-", // Read from stdin.
			// Use h264 encoding with reasonable quality and speed. Note that
			// "preset" can be adjusted if the system is too slow to handle encoding.
			"-c:v", "libx264",
			"-preset", "superfast",
			"-crf", "30",
			// Enable fast-start so videos can be displayed in the browser without
			// full download.
			"-movflags", "+faststart",
			path,
		)

		var err error

		// Allows for debugging ffmpeg in shell. TODO maybe remove.
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		pipe, err := c.StdinPipe()
		if err != nil {
			log.Fatalf("Error getting stdin %v", err)
		}

		if err := c.Start(); err != nil {
			log.Fatalf("Error starting ffmpeg %v", err)
		}

		var closer chan bool
	loop:
		for {
			select {
			case closer = <-f.close:
				pipe.Close()
				break loop
			case b := <-f.b:
				if _, err := pipe.Write(b); err != nil {
					// TODO better error handling.
					log.Fatalf("Error writing to pipe!")
				}

			}
		}

		log.Printf("Waiting for FFMPEG shutdown.")
		err = c.Wait()
		log.Printf("FFMPEG exit with status %v", err)
		closer <- true // Signal close is completed.
	}()
	return f
}

func (f *FFMpegSink) Close() {
	c := make(chan bool)
	f.close <- c
	<-c
}

func (f *FFMpegSink) Put(input source.Image) {
	f.b <- input.Mat.ToBytes()
}
