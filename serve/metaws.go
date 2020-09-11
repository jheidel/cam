package serve

import (
	"cam/notify"

	"github.com/gorilla/websocket"
	log "github.com/sirupsen/logrus"
	"net/http"
	"time"
)

const (
	// Time allowed to write message to the client
	writeWait  = 10 * time.Second
	pingPeriod = 10 * time.Second
)

type MetaUpdater struct {
	upgrader websocket.Upgrader
	cs       map[chan bool]bool
	addc     chan chan bool
	delc     chan chan bool
	notify   chan bool
}

func NewMetaUpdater() *MetaUpdater {
	m := &MetaUpdater{
		upgrader: websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		},
		cs:     make(map[chan bool]bool),
		addc:   make(chan chan bool),
		delc:   make(chan chan bool),
		notify: make(chan bool),
	}
	go func() {
		for {
			select {
			case c := <-m.addc:
				m.cs[c] = true
			case c := <-m.delc:
				delete(m.cs, c)
			case <-m.notify:
				for k, _ := range m.cs {
					k <- true
				}
			}
		}
	}()
	return m
}

func (m *MetaUpdater) FilesystemUpdated() {
	m.notify <- true
}

func (m *MetaUpdater) Notify(n *notify.Notification) error {
	m.notify <- true
	return nil
}

func (m *MetaUpdater) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ws, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		if _, ok := err.(websocket.HandshakeError); !ok {
			log.WithField("addr", r.RemoteAddr).Errorf("Websocket handshake failed for update stream: %v", err)
		}
		return
	}
	go m.serve(ws)
}

func (m *MetaUpdater) serve(ws *websocket.Conn) {
	clog := log.WithField("addr", ws.RemoteAddr())
	clog.Info("connected to events update socket")
	defer func() {
		ws.Close()
		clog.Info("disconnected from events update socket")
	}()
	pingTicker := time.NewTicker(pingPeriod)
	defer pingTicker.Stop()

	notifyc := make(chan bool)
	m.addc <- notifyc
	defer func() { m.delc <- notifyc }()

	// Even though we don't care about incoming messages, we need to read from
	// the socket in order to process control messages.
	go func() {
		for {
			if _, _, err := ws.NextReader(); err != nil {
				ws.Close()
				return
			}
		}
	}()

	for {
		select {
		case <-notifyc:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.TextMessage, []byte("update")); err != nil {
				return
			}
		case <-pingTicker.C:
			ws.SetWriteDeadline(time.Now().Add(writeWait))
			if err := ws.WriteMessage(websocket.PingMessage, []byte{}); err != nil {
				return
			}
		}
	}
}
