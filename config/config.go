package config

import (
	"image"
)

type Config struct {
	URI               string
	FilesystemMaxSize int64

	NotificationHoursStart int
	NotificationHoursEnd   int

	MotionBounds []image.Point
	MotionThresh float64
	MotionErode  int

	FullchainPath string
	PrivkeyPath   string
}
