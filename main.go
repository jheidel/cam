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

	"net/http"
)

import _ "net/http/pprof"

func main() {
	if len(os.Args) < 3 {
		fmt.Println("How to run:\n\tcapwindow [camera URI] [output file]")
		return
	}

	// parse args
	uri := os.Args[1]
	filename := os.Args[2]

	cap := source.NewVideoCapture(uri)
	window := sink.NewWindow("Output")
	defer window.Close()

	c := cap.Get()

	// TODO push this into sink.
	sample := <-c

	var video sink.Sink

	fps := 15

	video = sink.NewFFMpegSink(filename, fps, sample.Mat.Cols(), sample.Mat.Rows())
	video = sink.NewFPSNormalize(video, fps)
	defer video.Close()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	for {
		select {
		case i := <-c:
			i = process.DrawTimestamp("Gate", i)
			//window.Put(i)
			video.Put(i)
			i.Release()
		case sig := <-sigs:
			fmt.Println(sig)
			return
		}
	}
}
