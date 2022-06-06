// Package events provides a simple abstraction layer for producing and
// consuming in-process events.
package events

import (
	"sync"
)

// Manager is the type used to allow registration of watchers interested in
// certain kinds of events.
type Manager struct {
	watchers sync.Map
}

// Watch will create a new Watcher that can be used to wait for Events to be
// emitted.
func (m *Manager) Watch() *Watcher {
	watcher := &Watcher{
		Ch:   make(chan Event, 1),
		stop: m.Stop,
	}

	m.watchers.Store(watcher, watcher)
	return watcher
}

// Emit will send an Event out to all Watchers registered to listen for Events.
func (m *Manager) Emit(event Event) {
	m.watchers.Range(func(_, value any) bool {
		watcher := value.(*Watcher)
		watcher.Ch <- event
		return true
	})
}

// Stop will unregister a Watcher and close its Ch channel.
func (m *Manager) Stop(watcher *Watcher) {
	m.watchers.Delete(watcher)
	close(watcher.Ch)
}
