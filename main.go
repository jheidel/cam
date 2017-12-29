package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"cam/video/process"
	"cam/video/sink"
	"cam/video/source"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("How to run:\n\tcapwindow [camera URI]")
		return
	}

	// parse args
	uri := os.Args[1]

	cap := source.NewVideoCapture(uri)
	window := sink.NewWindow("Output")
	defer window.Close()

	c := cap.Get()

	sample := <-c

	video, err := sink.NewVideo("/tmp/test.mkv", 20, sample.Mat.Cols(), sample.Mat.Rows())
	if err != nil {
		log.Fatal("Failed to init video")
	}
	defer video.Close()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	for {
		select {
		case i := <-c:
			i = process.DrawTimestamp("Gate", i)
			window.Put(i)
			video.Put(i)
			i.Release()
		case sig := <-sigs:
			fmt.Println(sig)
			return
		}
	}
}
