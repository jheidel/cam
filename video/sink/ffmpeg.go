package sink

import (
	"cam/video/source"
	"fmt"
	"image"
	"log"
	"os"
	"os/exec"
	"time"
)

// TODO:
// - get ffmpeg from path or env variable
// - docs
// - error handling (remove fatals)
// - configuration options
// - limit number of skipped frames allowed

type FFmpegSink struct {
	b     chan []byte
	close chan chan bool
}

func NewFFmpegSink(path string, fps int, size image.Point, writeBuffer time.Duration) *FFmpegSink {
	// Ensure we can store a reasonable buffer in memory without waiting for
	// ffmpeg. This is needed since the rolling buffer will be dumped to FFmpeg
	// upon recording start and we don't want to hold up the newer frames.
	bufc := fps * int(writeBuffer.Seconds()) * 5 / 4

	f := &FFmpegSink{
		b:     make(chan []byte, bufc),
		close: make(chan chan bool),
	}
	go func() {

		c := exec.Command(
			"/usr/local/bin/ffmpeg",
			// Configure ffmpeg to read from the opencv pipe.
			"-f", "rawvideo",
			"-pixel_format", "bgr24",
			"-video_size", fmt.Sprintf("%dx%d", size.X, size.Y),
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

		// Allows for debugging ffmpeg in shell.
		// TODO
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		pipe, err := c.StdinPipe()
		if err != nil {
			log.Fatalf("Error getting stdin %v", err)
		}

		log.Printf("Start writing to '%s' using FFmpeg", path)
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
					log.Fatalf("Error writing to pipe!")
				}

			}
		}

		log.Printf("Waiting for FFmpeg shutdown...")
		err = c.Wait()
		log.Printf("Finished writing %s (error code %v)", path, err)
		closer <- true // Signal close is completed.
	}()
	return f
}

func (f *FFmpegSink) Close() {
	c := make(chan bool)
	f.close <- c
	<-c
}

func (f *FFmpegSink) Put(input source.Image) {
	// TODO ensure Mat is actually bgr24? Bindings don't appear to exist though.
	f.b <- input.Mat.ToBytes()
}
