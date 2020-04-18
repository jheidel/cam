package config

import (
	"context"
	"encoding/json"
	"os"
	"sync"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/fsnotify/fsnotify"
	log "github.com/sirupsen/logrus"
)

var (
	gLock   sync.RWMutex
	gConfig *Config
)

func configFromFile(path string) (*Config, error) {
	var config Config
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	p := json.NewDecoder(f)
	if err := p.Decode(&config); err != nil {
		return nil, err
	}
	log.Infof("Loaded configuration: %v", spew.Sdump(config))
	return &config, nil
}

func Get() *Config {
	gLock.RLock()
	defer gLock.RUnlock()
	return gConfig
}

func waitForChange(ctx context.Context, path string) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	defer watcher.Close()
	if err := watcher.Add(path); err != nil {
		return err
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-watcher.Events:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Second / 10):
	}
	return ctx.Err()
}

func Load(ctx context.Context, path string) error {
	config, err := configFromFile(path)
	if err != nil {
		return err
	}
	gConfig = config
	go func() {
		for ctx.Err() == nil {
			if err := waitForChange(ctx, path); err != nil {
				log.Errorf("Error waiting for file change: %v", err)
				continue
			}

			config, err := configFromFile(path)
			if err != nil {
				log.Errorf("Failed to load new config: %v", err)
				continue
			}
			gLock.Lock()
			gConfig = config
			gLock.Unlock()
		}
	}()
	return nil
}
