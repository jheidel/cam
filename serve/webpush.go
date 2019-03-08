package serve

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/SherClockHolmes/webpush-go"
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

	Peer string

	SubscriptionID       string `gorm:"unique_index"`
	PushSubscriptionJSON string

	LastSuccess        *time.Time
	LastFailure        *time.Time
	LastFailureMessage string
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
	mux.HandleFunc("/push_get_subscriptions", p.handleGetSubscriptions)
	mux.HandleFunc("/push_subscribe", p.handleSubscribe)
	mux.HandleFunc("/push_unsubscribe", p.handleUnsubscribe)

	// Manually test web push notifications by triggering a fake event.
	mux.HandleFunc("/push_test", p.handleTest)
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
	jb, _ := json.Marshal(sub)
	pc := &PushConfig{
		Peer:                 r.RemoteAddr,
		SubscriptionID:       sub.Endpoint,
		PushSubscriptionJSON: string(jb),
	}
	if err := p.db.Create(pc).Error; err != nil {
		log.Errorf("Failed to create push subscription: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Infof("Added push subscription for peer %v", pc.Peer)
}

func (p *WebPush) handleUnsubscribe(w http.ResponseWriter, r *http.Request) {
	sub := p.extractSub(w, r)
	if sub == nil {
		return
	}
	pc := &PushConfig{}
	if p.db.Where("subscription_id = ?", sub.Endpoint).First(pc).RecordNotFound() {
		http.Error(w, "subscription not found", http.StatusNotFound)
		return
	}
	if err := p.db.Delete(pc).Error; err != nil {
		log.Errorf("Failed to delete record %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	log.Infof("Removed push subscription for peer %v (created at %v)", pc.Peer, pc.CreatedAt)
}

func (p *WebPush) handleGetSubscriptions(w http.ResponseWriter, r *http.Request) {
	var subs []*PushConfig
	if err := p.db.Find(&subs).Error; err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	for _, s := range subs {
		// Don't write back key material.
		s.PushSubscriptionJSON = "REDACTED"
	}
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(subs); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (p *WebPush) handleTest(w http.ResponseWriter, r *http.Request) {
	p.NotifyAll()
}

func (p *WebPush) notifyOne(pc *PushConfig, payload []byte) error {
	var ps webpush.Subscription
	if err := json.NewDecoder(strings.NewReader(pc.PushSubscriptionJSON)).Decode(&ps); err != nil {
		return err
	}

	resp, err := webpush.SendNotification(payload, &ps, &webpush.Options{
		// TODO better suited address
		Subscriber:      "jheidel@gmail.com",
		VAPIDPublicKey:  p.Key.Public,
		VAPIDPrivateKey: p.Key.Private,
		TTL:             120,
		Urgency:         webpush.UrgencyHigh,
		Topic:           "cam_event",
	})
	if resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone) {
		log.Infof("Push service reports status %v, deleting from database.", resp.Status)
		if err := p.db.Delete(pc).Error; err != nil {
			log.Errorf("Failed to remove record from db: %v", err)
			return err
		}
		return nil
	}

	// Update the push record with the results of this push.
	now := time.Now()
	if err != nil {
		log.Warnf("Web push to client failed: %v", err)
		pc.LastFailure = &now
		pc.LastFailureMessage = err.Error()
	} else {
		pc.LastSuccess = &now
	}

	if err := p.db.Save(pc).Error; err != nil {
		return err
	}
	return nil
}

type Notification struct {
	First  string
	Second string
}

func (p *WebPush) NotifyAll() error {
	var subs []*PushConfig
	if err := p.db.Find(&subs).Error; err != nil {
		return err
	}

	n := &Notification{
		First:  "Hello",
		Second: "World!",
	}

	payload, err := json.Marshal(n)
	if err != nil {
		return err
	}

	log.Infof("Sending web push notification to %d subscribers", len(subs))
	var wg sync.WaitGroup
	for _, s := range subs {
		wg.Add(1)
		go func(pc *PushConfig) {
			if err := p.notifyOne(pc, payload); err != nil {
				log.Errorf("Web push notify failed: %v", err)
			}
			wg.Done()
		}(s)
	}
	wg.Wait()
	log.Infof("Web push completed")
	return nil
}
