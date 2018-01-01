package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cam/video/process"
	"cam/video/sink"
	"cam/video/source"
)

var (
	port = flag.Int("port", 8080, "Port to host web frontend.")
)

func main() {
	flag.Parse()

	// TODO migrate to flags once you have a config file.
	if len(os.Args) < 3 {
		fmt.Println("How to run:\n\tcapwindow [camera URI] [output file]")
		return
	}

	// parse args
	uri := os.Args[1]
	filename := os.Args[2]

	cap := source.NewVideoCapture(uri)
	//window := sink.NewWindow("Output")
	//defer window.Close()

	c := cap.Get()

	var video sink.Sink

	fps := 15

	video = sink.NewFFmpegSink(filename, fps, cap.Size(), 5*time.Second)
	video = sink.NewFPSNormalize(video, fps)
	defer video.Close()

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		// TODO link to polymer build directory.
		log.Printf("Hosting web frontend on port %d", *port)
		http.Handle("/", http.FileServer(http.Dir("./web/build/default")))
		log.Println(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
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
