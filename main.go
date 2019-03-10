package main

import (
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"cam/notify"
	"cam/serve"
	"cam/util"
	"cam/video"
	"cam/video/process"
	"cam/video/sink"
	"cam/video/source"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	log "github.com/sirupsen/logrus"
)

var (
	port     = flag.Int("port", 8443, "Port to host http web frontend.")
	rootPath = flag.String("root", "/home/jeff/db/", "Root path for storing videos")
	certPath = flag.String("cert", "/home/jeff/devkeys/cert.pem", "Path to cert.pem file")
	keyPath  = flag.String("key", "/home/jeff/devkeys/privkey.pem", "Path to key.pem file")
)

func main() {
	flag.Parse()

	// TODO migrate to flags once you have a config file.
	if len(os.Args) < 2 {
		fmt.Println("How to run:\n\t./main [camera URI]")
		os.Exit(1)
		return
	}

	// Configure logging.
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)

	// parse args
	uri := os.Args[1]

	ffmpegp, err := util.LocateFFmpeg()
	if err != nil {
		log.Errorf("Unable to locate ffmpeg binary: %v", err)
		fmt.Println("FFmpeg is required for saving video files.")
		fmt.Println("Either ensure the ffmpeg binary is in $PATH,")
		fmt.Println("or set the FFMPEG environment variable.")
		os.Exit(1)
		return
	} else {
		log.Infof("Located ffmpeg binary, %v", ffmpegp)
	}

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
		BasePath: *rootPath,
		// DO NOT SUBMIT
		MaxSize: 5 << 30, // 100 GiB
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

	prototxt, err := Asset("models/MobileNetSSD_deploy.prototxt")
	if err != nil {
		log.Fatalf("Failed to load model prototxt: %v", err)
	}

	caffeModel, err := Asset("models/MobileNetSSD_deploy.caffemodel")
	if err != nil {
		log.Fatalf("Failed to load caffemodel: %v", err)
	}

	classifier := process.NewClassifier(prototxt, caffeModel)

	rec := video.NewRecorder(vp, &video.RecorderOptions{BufferTime: buftime, RecordTime: rectime, MaxRecordTime: maxtime})
	defer rec.Close()

	// Enable / disable the classifier when recording.
	rec.Listeners = append(rec.Listeners, &video.ClassifierRecordTrigger{
		Classifier: classifier,
	})

	motion := process.NewMotion(mjpegServer, classifier, cap.Size())
	// Trigger recorder on motion.
	motion.Triggers = append(motion.Triggers, rec)

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	meta := &serve.MetaServer{
		FS: fs,
	}

	delete := &serve.DeleteServer{
		FS: fs,
	}

	metaws := serve.NewMetaUpdater()
	fs.AddListener(metaws) // Receive filesystem updates

	push, err := notify.NewWebPush(*rootPath)
	if err != nil {
		log.Fatalf("Failed to set up web push: %v", err)
	}

	notifier := &notify.Notifier{
		Listeners: []notify.NotifyListener{push},
	}
	motion.Triggers = append(motion.Triggers, notifier)
	rec.Listeners = append(rec.Listeners, notifier)

	go func() {
		log.Infof("Hosting web frontend on port %d", *port)
		http.Handle("/mjpeg", mjpegServer)
		http.Handle("/trigger", rec)
		http.Handle("/events", meta)
		http.Handle("/eventsws", metaws)
		http.Handle("/delete", delete)
		http.Handle("/video", serve.NewVideoServer(fs))
		http.Handle("/thumb", serve.NewThumbServer(fs))
		http.Handle("/vthumb", serve.NewVThumbServer(fs))
		http.Handle("/",
			http.FileServer(
				&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo, Prefix: "web/build/default"}))
		push.RegisterHandlers(http.DefaultServeMux)

		ps := fmt.Sprintf(":%d", *port)
		err := http.ListenAndServeTLS(ps, *certPath, *keyPath, nil)
		log.Infof("HTTP server exited with status %v", err)
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
			log.Warningf("Caught signal %v", sig)
			return
		}
	}
}
