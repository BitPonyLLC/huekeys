package events

import (
	"sync"
)

type Manager struct {
	watchers sync.Map
}

func (m *Manager) Watch() *Watcher {
	watcher := &Watcher{
		Ch:   make(chan Event, 1),
		stop: m.Stop,
	}

	m.watchers.Store(watcher, watcher)
	return watcher
}

func (m *Manager) Emit(event Event) {
	m.watchers.Range(func(_, value any) bool {
		watcher := value.(*Watcher)
		watcher.Ch <- event
		return true
	})
}

func (m *Manager) Stop(watcher *Watcher) {
	m.watchers.Delete(watcher)
	close(watcher.Ch)
}
