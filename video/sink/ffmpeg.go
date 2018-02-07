package sink

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"image"
	"os"
	"os/exec"
	"time"

	"cam/util"
	"cam/video/source"
)

// TODO:
// - get ffmpeg from path or env variable
// - docs
// - error handling (remove fatals)
// - race where subprocess receives signal before the main program can react
// (falls under error handling.)
// - configuration options
// - limit number of skipped frames allowed

const (
	ExtTemp = ".temp"
)

type FFmpegOptions struct {
	// Size is the dimensions of the source image.
	Size image.Point

	// FPS is the target frames per second of the file.
	FPS int

	// BufferTime is the amount of expected historical state to write.
	BufferTime time.Duration
}

type FFmpegSink struct {
	Path  string
	b     chan []byte
	close chan chan bool
}

// TODO ffmpeg producer.

func NewFFmpegSink(path string, opts FFmpegOptions) *FFmpegSink {
	// Ensure we can store a reasonable buffer in memory without waiting for
	// ffmpeg. This is needed since the rolling buffer will be dumped to FFmpeg
	// upon recording start and we don't want to hold up the newer frames.
	bufc := opts.FPS * (int(opts.BufferTime.Seconds()) + 20)

	f := &FFmpegSink{
		Path:  path,
		b:     make(chan []byte, bufc),
		close: make(chan chan bool),
	}
	go func() {

		c := exec.Command(
			util.LocateFFmpegOrDie(),
			// Configure ffmpeg to read from the opencv pipe.
			"-f", "rawvideo",
			"-pixel_format", "bgr24",
			"-video_size", fmt.Sprintf("%dx%d", opts.Size.X, opts.Size.Y),
			"-framerate", fmt.Sprintf("%d", opts.FPS),
			"-i", "-", // Read from stdin.
			// Use h264 encoding with reasonable quality and speed. Note that
			// "preset" can be adjusted if the system is too slow to handle encoding.
			"-c:v", "libx264",
			//"-preset", "superfast",
			"-preset", "ultrafast",
			"-crf", "30",
			// Enable fast-start so videos can be displayed in the browser without
			// full download.
			"-movflags", "+faststart",
			// Allow playback on a wider range of devices.
			"-pix_fmt", "yuv420p",
			"-profile:v", "baseline",
			"-level", "3.0",
			// Explicit format since our active output file will have a special extension.
			"-f", "mp4",
			path+ExtTemp,
		)

		var err error

		// TODO convert some of this to structured logging.

		// Allows for debugging ffmpeg in shell.
		// TODO
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr

		pipe, err := c.StdinPipe()
		if err != nil {
			log.Fatalf("Error getting stdin %v", err)
		}

		log.Infof("Start writing to '%s' using FFmpeg", path)
		if err := c.Start(); err != nil {
			log.Fatalf("Error starting ffmpeg %v", err)
		}

		var closec chan bool
	loop:
		for {
			select {
			case closec = <-f.close:
				log.Printf("Closing FFMPEG.")
				pipe.Close()
				break loop
			case b := <-f.b:
				if _, err := pipe.Write(b); err != nil {
					// TODO error handling.
					log.Fatalf("Error writing to pipe! (packet length %d)", len(b))
				}

			}
		}

		log.Printf("Waiting for FFmpeg shutdown...")
		err = c.Wait()
		log.Printf("Finished writing %s (error code %v)", path, err)

		if err := os.Rename(f.Path+ExtTemp, f.Path); err != nil {
			log.Errorf("Error moving file to its final destination")
		}
		closec <- true
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
	select {
	case f.b <- input.Mat.ToBytes():
	default:
		log.Warningf("WARN: video output frame skip. Insufficient buffer?")
	}
}
