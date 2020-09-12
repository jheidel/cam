package util

import (
	"sync"
)

type Event struct {
	notified bool
	c        *sync.Cond
}

func NewEvent() *Event {
	return &Event{
		c: sync.NewCond(&sync.Mutex{}),
	}
}

func (e *Event) Notify() {
	e.c.L.Lock()
	defer e.c.L.Unlock()
	if !e.notified {
		e.notified = true
		e.c.Broadcast()
	}
}

func (e *Event) Wait() {
	e.c.L.Lock()
	defer e.c.L.Unlock()
	for !e.notified {
		e.c.Wait()
	}
}

func (e *Event) HasBeenNotified() bool {
	e.c.L.Lock()
	defer e.c.L.Unlock()
	return e.notified
}
