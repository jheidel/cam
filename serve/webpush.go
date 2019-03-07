package serve

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/davecgh/go-spew/spew"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
	log "github.com/sirupsen/logrus"
)

const DatabaseFile = "webpush.db"

type VAPIDKey struct {
	Public  string
	Private string
}

type WebPush struct {
	// Key is the VAPID key for the web push. It will be generated from startup and
	// persisted in the database.
	Key *VAPIDKey

	db *gorm.DB
}

type PushConfig struct {
	gorm.Model
}

func NewWebPush(root string) (*WebPush, error) {
	path := filepath.Join(root, DatabaseFile)
	db, err := gorm.Open("sqlite3", path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %v", err)
	}
	db.AutoMigrate(&VAPIDKey{})
	db.AutoMigrate(&PushConfig{})
	p := &WebPush{
		Key: &VAPIDKey{},
		db:  db,
	}
	// Load VAPID key from database, otherwise create.
	if db.First(p.Key).RecordNotFound() {
		priv, pub, err := webpush.GenerateVAPIDKeys()
		if err != nil {
			return nil, err
		}
		p.Key.Private = priv
		p.Key.Public = pub
		if err := db.Create(p.Key).Error; err != nil {
			return nil, err
		}
		log.Infof("Web push VAPID keys generated")
	} else {
		log.Infof("Web push VAPID keys loaded from database")
	}
	return p, nil
}

func (p *WebPush) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/push_get_pubkey", p.handleGetPubkey)
	mux.HandleFunc("/push_subscribe", p.handleSubscribe)
	mux.HandleFunc("/push_unsubscribe", p.handleUnsubscribe)
}

func (p *WebPush) handleGetPubkey(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	io.WriteString(w, p.Key.Public)
}

func (p *WebPush) extractSub(w http.ResponseWriter, r *http.Request) *webpush.Subscription {
	if r.Method != "POST" {
		http.Error(w, "Invalid request method", http.StatusMethodNotAllowed)
		return nil
	}
	sub := &webpush.Subscription{}
	if err := json.NewDecoder(r.Body).Decode(sub); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return nil
	}
	return sub
}

func (p *WebPush) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	sub := p.extractSub(w, r)
	if sub == nil {
		return
	}

	log.Infof("Got sub request %v", spew.Sdump(sub))

	go func(p *WebPush, sub *webpush.Subscription) {
		time.Sleep(10 * time.Second)
		log.Infof("Sending notification")

		webpush.SendNotification([]byte("TEST"), sub, &webpush.Options{
			Subscriber:      "jheidel@gmail.com",
			VAPIDPrivateKey: p.Key.Private,
			VAPIDPublicKey:  p.Key.Public,
			TTL:             30,
		})
	}(p, sub)

	// TODO continue.
}

func (p *WebPush) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	sub := p.extractSub(w, r)
	if sub == nil {
		return
	}
	log.Infof("Got unsub request %v", spew.Sdump(sub))
}
