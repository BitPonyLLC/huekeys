package events

type Event interface{}

type Watcher struct {
	Ch chan Event

	stop func(*Watcher)
}

func (w *Watcher) Stop() {
	w.stop(w)
}
