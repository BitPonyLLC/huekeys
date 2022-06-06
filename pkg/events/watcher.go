package events

// Event is an interface to allow any kind of message to be produced to
// Watchers.
type Event interface{}

// Watcher is the type used to process Events that have been emitted on the Ch
// channel.
type Watcher struct {
	Ch chan Event

	stop func(*Watcher)
}

// Stop will unregister a Watcher and close its Ch channel.
func (w *Watcher) Stop() {
	w.stop(w)
}
