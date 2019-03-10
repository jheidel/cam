package notify

import (
	"cam/video"
	"cam/video/process"
	"sync"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
)

const (
	ConfidenceThreshold = 0.9

	// TODO: move quiet hours to server configuration
	NotificationHoursStart = 6
	NotificationHoursEnd   = 20
)

// Notification is sent to all NotifyListeners registered with Notifier.
type Notification struct {
	TimeString string
	Identifier string
	Detection  process.Detection
}

type NotifyListener interface {
	Notify(n *Notification) error
}

type Notifier struct {
	Listeners []NotifyListener

	vr       *video.VideoRecord
	notified bool

	l sync.Mutex
}

// MotionDetected is invoked when motion is detected by the camera.
func (n *Notifier) MotionDetected() {
	// Does nothing based on motion alone.
}

// MotionDetected is invoked when motion is detected by the camera.
func (n *Notifier) MotionClassified(detection process.Detections) {
	n.l.Lock()
	defer n.l.Unlock()

	if n.vr == nil || n.notified {
		// Nothing to do.
		return
	}

	detections := detection.SortedDetections()
	if len(detections) == 0 {
		return
	}
	if detections[0].Confidence < ConfidenceThreshold {
		// Not interesting enough for notification.
		return
	}

	ts := n.vr.TriggeredAt

	if ts.Hour() < NotificationHoursStart || ts.Hour() >= NotificationHoursEnd {
		log.Infof("Would send notification, but currently in quiet hours.")
		return
	}

	notification := &Notification{
		TimeString: ts.Format("3:04 PM"),
		Identifier: n.vr.Identifier,
		Detection:  detections[0],
	}
	log.Infof("Sending notification: %v", spew.Sdump(notification))
	for _, l := range n.Listeners {
		go func(l NotifyListener) {
			if err := l.Notify(notification); err != nil {
				log.Errorf("Failed to send notification: %v", err)
			}
		}(l)
	}
	n.notified = true
}

// StartRecording is invoked when the video recorder starts.
func (n *Notifier) StartRecording(vr *video.VideoRecord) {
	n.l.Lock()
	defer n.l.Unlock()
	n.vr = vr
	n.notified = false
}

// StartRecording is invoked when the video recorder completes.
func (n *Notifier) StopRecording(vr *video.VideoRecord) {
	n.l.Lock()
	defer n.l.Unlock()
	n.vr = nil
	n.notified = false
}
