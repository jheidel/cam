package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cam/serve"
	"cam/video"
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
	if len(os.Args) < 2 {
		fmt.Println("How to run:\n\tcapwindow [camera URI]")
		return
	}

	// parse args
	uri := os.Args[1]

	fps := 15

	inputfps := fps
	if !strings.HasSuffix(uri, ".mp4") {
		// Live source, use high FPS ceiling.
		inputfps = 100
	}

	// TODO increase FPS for live sources.
	cap := source.NewVideoCapture(uri, inputfps)
	// defer cap.Close()

	//window := sink.NewWindow("Output")
	//defer window.Close()

	c := cap.Get()

	buftime := 2 * time.Second
	rectime := 20 * time.Second
	// Max time for video clips before interruption.
	maxtime := 5 * time.Minute

	fsOpts := video.FilesystemOptions{
		BasePath: "/tmp/gatecam2/",
		MaxSize:  100 << 30, // 100 GiB
	}
	fs, err := video.NewFilesystem(fsOpts)
	if err != nil {
		log.Fatalf("Failed to create filesystem: %v", err)
	}

	vp := &video.VideoSinkProducer{
		FFmpegOptions: sink.FFmpegOptions{
			Size:       cap.Size(),
			FPS:        fps,
			BufferTime: buftime,
		},
		Filesystem:     fs,
		VThumbProducer: process.NewVThumbProducer(),
	}

	mjpegServer := sink.NewMJPEGServer()

	msraw := mjpegServer.NewStream(sink.MJPEGID{Name: "raw"})
	defer msraw.Close()

	msdefault := mjpegServer.NewStream(sink.MJPEGID{Name: "default"})
	defer msdefault.Close()

	rec := video.NewRecorder(vp, &video.RecorderOptions{BufferTime: buftime, RecordTime: rectime, MaxRecordTime: maxtime})
	defer rec.Close()

	motion := process.NewMotion(mjpegServer, cap.Size())
	// Trigger recorder on motion.
	motion.Trigger = rec

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	meta := &serve.MetaServer{
		FS: fs,
	}

	metaws := serve.NewMetaUpdater()
	fs.Listeners = append(fs.Listeners, metaws) // Receive filesystem updates

	go func() {
		log.Printf("Hosting web frontend on port %d", *port)
		http.Handle("/mjpeg", mjpegServer)
		http.Handle("/trigger", rec)
		http.Handle("/events", meta)
		http.Handle("/eventsws", metaws)
		http.Handle("/video", serve.NewVideoServer(fs))
		http.Handle("/thumb", serve.NewThumbServer(fs))
		http.Handle("/vthumb", serve.NewVThumbServer(fs))
		// TODO link to polymer build directory.
		http.Handle("/", http.FileServer(http.Dir("./web/build/default")))
		log.Println(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
	}()

	for {
		select {
		case i := <-c:
			msraw.Put(i.Mat)

			motion.Process(i.Mat)

			i = process.DrawTimestamp("Gate", i)
			//window.Put(i)

			msdefault.Put(i.Mat)

			//video.Put(i)
			rec.Put(i)
			i.Release()
		case sig := <-sigs:
			log.Println("Caught signal", sig)
			return
		}
	}
}
