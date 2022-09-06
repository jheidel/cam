package sink

import (
	"fmt"
	log "github.com/sirupsen/logrus"
	"gocv.io/x/gocv"
	"image"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// MJPEG multi-streaming, based on implementation by saljam:
// https://github.com/saljam/mjpeg/blob/master/stream.go

const boundaryWord = "MJPEGBOUNDARY"
const headerf = "\r\n" +
	"--" + boundaryWord + "\r\n" +
	"Content-Type: image/jpeg\r\n" +
	"Content-Length: %d\r\n" +
	"X-Timestamp: 0.000000\r\n" +
	"\r\n"

type MJPEGID struct {
	// TODO include camera
	Name string
}

type MJPEGClientState struct {
	Width   int
	Height  int
	FPS     float64
	Quality int

	lastSent time.Time
}

func (cl *MJPEGClientState) ready() bool {
	if cl.FPS == 0 {
		return true // No FPS limit
	}
	if cl.lastSent.IsZero() {
		return true // first frame
	}
	d := time.Since(cl.lastSent)
	return float64(d) >= float64(time.Second)/cl.FPS
}

func (cl *MJPEGClientState) key() MJPEGClientState {
	return MJPEGClientState{
		Width:   cl.Width,
		Height:  cl.Height,
		Quality: cl.Quality,
	}
}

type MJPEGServer struct {
	m map[MJPEGID]*MJPEGStream

	lock sync.Mutex
}

func NewMJPEGServer() *MJPEGServer {
	return &MJPEGServer{
		m: make(map[MJPEGID]*MJPEGStream),
	}
}

func (s *MJPEGServer) NewStream(id MJPEGID) *MJPEGStream {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.m[id]; ok {
		log.Panicf("A stream for %v already exists", id)
	}

	ms := &MJPEGStream{
		id:     id,
		m:      make(map[chan []byte]*MJPEGClientState),
		frame:  make([]byte, len(headerf)),
		parent: s,
	}

	s.m[id] = ms
	return ms
}

func (s *MJPEGServer) getStream(id MJPEGID) *MJPEGStream {
	s.lock.Lock()
	defer s.lock.Unlock()
	if ms, ok := s.m[id]; ok {
		return ms
	}
	return nil
}

// ServeHTTP implements http.Handler interface, serving MJPEG.
func (s *MJPEGServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	id := MJPEGID{
		Name: r.Form.Get("name"),
	}

	if id.Name == "" {
		http.Error(w, "missing name", http.StatusBadRequest)
		return
	}

	stream := s.getStream(id)
	if stream == nil {
		http.Error(w, "unknown stream ID", http.StatusNotFound)
		return
	}

	w.Header().Add("Content-Type", "multipart/x-mixed-replace;boundary="+boundaryWord)

	cs := &MJPEGClientState{}

	if s := r.Form.Get("width"); s != "" {
		if v, err := strconv.Atoi(s); err != nil {
			http.Error(w, "bad width", http.StatusBadRequest)
			return
		} else {
			cs.Width = v
		}
	}

	if s := r.Form.Get("height"); s != "" {
		if v, err := strconv.Atoi(s); err != nil {
			http.Error(w, "bad height", http.StatusBadRequest)
			return
		} else {
			cs.Height = v
		}
	}

	if s := r.Form.Get("quality"); s != "" {
		if v, err := strconv.Atoi(s); err != nil {
			http.Error(w, "bad quality", http.StatusBadRequest)
			return
		} else {
			cs.Quality = v
		}
	}

	if s := r.Form.Get("fps"); s != "" {
		if v, err := strconv.ParseFloat(s, 64); err != nil {
			http.Error(w, "bad fps", http.StatusBadRequest)
			return
		} else {
			cs.FPS = v
		}
	}

	log.WithField("addr", r.RemoteAddr).Infof("MJPEG stream connected to %v for %q with config %#v", id, id.Name, cs)

	c := make(chan []byte)
	stream.lock.Lock()
	stream.m[c] = cs
	stream.lock.Unlock()

	for {
		b := <-c
		_, err := w.Write(b)
		if err != nil {
			break
		}
	}

	stream.lock.Lock()
	delete(stream.m, c)
	stream.lock.Unlock()
	log.WithField("addr", r.RemoteAddr).Infof("MJPEG stream disconnected from %v", id)
}

type MJPEGStream struct {
	id    MJPEGID
	m     map[chan []byte]*MJPEGClientState
	frame []byte

	parent *MJPEGServer
	lock   sync.Mutex
}

func (s *MJPEGStream) empty() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	if len(s.m) == 0 {
		return true // no clients
	}

	// check if there is a client available for a new frame
	for _, cl := range s.m {
		if cl.ready() {
			return false
		}
	}
	return true
}

func (s *MJPEGStream) Put(input gocv.Mat) {
	if s.empty() {
		// Nobody is listening; don't bother encoding.
		return
	}

	// Idenitfy all the resolutions we need to encode for connected clients.
	resolutions := make(map[MJPEGClientState][]byte)
	s.lock.Lock()
	for _, cl := range s.m {
		if !cl.ready() {
			continue
		}
		resolutions[cl.key()] = nil
	}
	s.lock.Unlock()

	// Generate all resolutions and qualities we need
	for res, _ := range resolutions {
		var convert gocv.Mat
		convert = input

		small := gocv.NewMat()
		defer small.Close()

		if res.Width != 0 || res.Height != 0 {
			gocv.Resize(input, &small, image.Point{X: res.Width, Y: res.Height}, 0, 0, gocv.InterpolationDefault)
			convert = small
		}

		var jpeg *gocv.NativeByteBuffer
		var err error

		if res.Quality == 0 {
			jpeg, err = gocv.IMEncode(".jpg", convert)
		} else {
			jpeg, err = gocv.IMEncodeWithParams(".jpg", convert, []int{gocv.IMWriteJpegQuality, res.Quality})
		}

		if err != nil {
			log.Errorf("Error encoding to JPG for MJPEG stream %v: %v", s.id, err)
			return
		}

		header := fmt.Sprintf(headerf, jpeg.Len())

		// TODO: optimize to avoid the make+copy (tricky with multiple consumers)
		l := len(header) + jpeg.Len()
		b := make([]byte, l)
		copy(b, header)
		copy(b[len(header):], jpeg.GetBytes())

		// Save final result
		resolutions[res] = b
	}

	// Stream converted image to all clients
	s.lock.Lock()
	defer s.lock.Unlock()
	for c, cl := range s.m {
		if !cl.ready() {
			continue
		}

		b, ok := resolutions[cl.key()]
		if !ok {
			log.Warnf("generated image not available for %#v, maybe recently connected client?", cl.key())
			continue
		}

		select {
		case c <- b:
			cl.lastSent = time.Now()
		default:
			// Skip listeners not ready for next frame.
		}
	}
}

func (s *MJPEGStream) Close() {
	s.parent.lock.Lock()
	defer s.parent.lock.Unlock()
	delete(s.parent.m, s.id)
}

// MJPEGStreamPool is a convenience wrapper that holds a number of streams that
// are created dynamically when referenced.
type MJPEGStreamPool struct {
	server *MJPEGServer
	m      map[MJPEGID]*MJPEGStream
}

func (s *MJPEGServer) NewStreamPool() *MJPEGStreamPool {
	return &MJPEGStreamPool{
		server: s,
		m:      make(map[MJPEGID]*MJPEGStream),
	}
}

func (p *MJPEGStreamPool) Put(name string, img gocv.Mat) {
	id := MJPEGID{
		Name: name,
	}
	var stream *MJPEGStream
	var ok bool
	if stream, ok = p.m[id]; !ok {
		stream = p.server.NewStream(id)
		p.m[id] = stream
	}
	stream.Put(img)
}

func (p *MJPEGStreamPool) Close() {
	for _, s := range p.m {
		s.Close()
	}
	// Clear.
	p.m = make(map[MJPEGID]*MJPEGStream)
}
