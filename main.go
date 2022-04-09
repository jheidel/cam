package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"cam/config"
	"cam/notify"
	"cam/serve"
	"cam/util"
	"cam/video"
	"cam/video/process"
	"cam/video/sink"
	"cam/video/source"

	assetfs "github.com/elazarl/go-bindata-assetfs"
	"github.com/gorilla/handlers"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	log "github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
)

var (
	port       = flag.Int("port", 8080, "Port to host http web frontend.")
	portSSL    = flag.Int("port_ssl", 8443, "Port to host https web frontend (requires certificates set)")
	sslCert    = flag.String("ssl_cert", os.Getenv("SSL_CERT"), "SSL certificate for https")
	sslKey     = flag.String("ssl_key", os.Getenv("SSL_KEY"), "SSL private key for https")
	rootPath   = flag.String("root", "/tmp", "Root path for storing videos")
	configFile = flag.String("config", "config.template.json", "Path to the camera configuration file")
	database   = flag.String("database", os.Getenv("DATABASE"), "Mysql database path. Required.")

	BuildTimestamp string
	BuildGitHash   string
)

func topLevelContext() context.Context {
	ctx, cancelf := context.WithCancel(context.Background())
	go func() {
		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigs
		log.Warnf("Caught signal %q, shutting down.", sig)
		cancelf()
	}()
	return ctx
}

func main() {
	flag.Parse()

	ctx := topLevelContext()

	// Configure logging.
	customFormatter := new(log.TextFormatter)
	customFormatter.TimestampFormat = "2006-01-02 15:04:05"
	customFormatter.FullTimestamp = true
	log.SetFormatter(customFormatter)

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

	if err := config.Load(ctx, *configFile); err != nil {
		log.Fatalf("Failed to load initial config: %v", err)
	}

	uri := config.Get().URI
	if uri == "" {
		log.Fatalf("URI is required")
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

	c := cap.Get()

	buftime := 2 * time.Second
	rectime := 20 * time.Second
	// Max time for video clips before interruption.
	maxtime := 5 * time.Minute
	if v := config.Get().MaxRecordTimeSec; v > 0 {
		maxtime = time.Duration(v) * time.Second
	}

	fsOpts := video.FilesystemOptions{
		DatabaseURI: *database,
		BasePath:    *rootPath,
		MaxSize:     config.Get().FilesystemMaxSize,
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

	meta := &serve.MetaServer{
		FS: fs,
	}

	delete := &serve.DeleteServer{
		FS: fs,
	}

	metaws := serve.NewMetaUpdater()
	fs.AddListener(metaws) // Receive filesystem updates

	push, err := notify.NewWebPush(fs.DB())
	if err != nil {
		log.Fatalf("Failed to set up web push: %v", err)
	}

	notifyws := serve.NewMetaUpdater()

	notifier := &notify.Notifier{
		Listeners: []notify.NotifyListener{push, notifyws},
	}
	motion.Triggers = append(motion.Triggers, notifier)
	rec.Listeners = append(rec.Listeners, notifier)

	go func() {
		http.Handle("/mjpeg", mjpegServer)
		http.Handle("/trigger", rec)
		http.Handle("/events", handlers.CompressHandler(meta))
		http.Handle("/eventsws", metaws)
		http.Handle("/delete", delete)
		http.Handle("/video", serve.NewVideoServer(fs))
		http.Handle("/thumb", serve.NewThumbServer(fs))
		http.Handle("/vthumb", serve.NewVThumbServer(fs))
		http.Handle("/notifyws", notifyws)
		http.Handle("/metrics", promhttp.Handler())
		http.HandleFunc("/build", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			ts, err := strconv.Atoi(BuildTimestamp)
			if err != nil {
				log.Fatalf("build timestamp %v not an integer", BuildTimestamp)
			}
			t := time.Unix(int64(ts), 0)
			fmt.Fprintf(w, "ts=%s\n", t.Format("Jan 2, 2006 3:04 PM"))
			fmt.Fprintf(w, "cam=%s\n", BuildGitHash)
			fmt.Fprintf(w, "gocv=%s\n", gocv.Version())
			fmt.Fprintf(w, "opencv=%s\n", gocv.OpenCVVersion())
		})
		http.Handle("/",
			http.FileServer(
				&assetfs.AssetFS{Asset: Asset, AssetDir: AssetDir, AssetInfo: AssetInfo, Prefix: "web/build/default"}))
		push.RegisterHandlers(http.DefaultServeMux)
		var err error

		if cert, key := *sslCert, *sslKey; cert != "" && key != "" {
			go func() {
				// Redirect HTTP traffic to HTTPS endpoint
				log.Infof("Hosting https redirect on port %d", *port)
				err := http.ListenAndServe(fmt.Sprintf(":%d", *port), http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					http.Redirect(w, r, "https://"+r.Host+r.RequestURI, http.StatusMovedPermanently)
				}))
				log.Infof("HTTP redirect server exited with status %v", err)
			}()
			log.Infof("Hosting web frontend on port %d", *portSSL)
			err = http.ListenAndServeTLS(fmt.Sprintf(":%d", *portSSL), cert, key, nil)
		} else {
			// Fallback to serving on HTTP
			log.Infof("Hosting web frontend on port %d", *port)
			err = http.ListenAndServe(fmt.Sprintf(":%d", *port), nil)
		}

		log.Infof("HTTP server exited with status %v", err)
	}()

	// Main loop: continously read from the camera and handle images.
	for ctx.Err() == nil {
		select {
		case i := <-c:
			msraw.Put(i.Mat)

			motion.Process(i.Mat)

			i = process.DrawTimestamp("Gate", i)
			//window.Put(i)

			msdefault.Put(i.Mat)

			//video.Put(i)
			rec.Put(i)

			// All done with this image.
			i.Close()
		case <-ctx.Done():
			return
		}
	}
	log.Warnf("Exit")
}
