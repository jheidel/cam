package sink

import (
	"fmt"
	"gocv.io/x/gocv"
	"log"
	"net/http"
	"sync"
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
		m:      make(map[chan []byte]bool),
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
		http.Error(w, err.Error(), 400)
		return
	}

	id := MJPEGID{
		Name: r.Form.Get("name"),
	}

	if id.Name == "" {
		http.Error(w, "missing name", 400)
		return
	}

	stream := s.getStream(id)
	if stream == nil {
		http.Error(w, "unknown stream ID", 404)
		return
	}

	log.Println("MJPEG stream:", r.RemoteAddr, "connected to", id)
	w.Header().Add("Content-Type", "multipart/x-mixed-replace;boundary="+boundaryWord)

	c := make(chan []byte)
	stream.lock.Lock()
	stream.m[c] = true
	stream.lock.Unlock()

	for {
		b := <-c
		_, err := w.Write(b)
		if err != nil {
			break
		}

		// TODO enforce a maximum FPS?
	}

	stream.lock.Lock()
	delete(stream.m, c)
	stream.lock.Unlock()
	log.Println("MJPEG stream:", r.RemoteAddr, "disconnected from", id)
}

type MJPEGStream struct {
	id    MJPEGID
	m     map[chan []byte]bool
	frame []byte

	parent *MJPEGServer
	lock   sync.Mutex
}

func (s *MJPEGStream) empty() bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	return len(s.m) == 0
}

func (s *MJPEGStream) Put(input gocv.Mat) {
	if s.empty() {
		// Nobody is listening; don't bother encoding.
		return
	}

	jpeg, err := gocv.IMEncode(".jpg", input)
	if err != nil {
		log.Printf("Error encoding to JPG for MJPEG stream %v: %v", s.id, err)
		return
	}

	header := fmt.Sprintf(headerf, len(jpeg))
	if len(s.frame) < len(jpeg)+len(header) {
		s.frame = make([]byte, (len(jpeg)+len(header))*2)
	}

	copy(s.frame, header)
	copy(s.frame[len(header):], jpeg)

	s.lock.Lock()
	defer s.lock.Unlock()
	for c := range s.m {
		select {
		case c <- s.frame[:(len(header) + len(jpeg))]:
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
