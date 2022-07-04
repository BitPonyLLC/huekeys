package util

import "time"

// Woke is a utility to help know when a process was suspended for some amount of time.
type Woke struct {
	delay   time.Duration
	diffMin time.Duration
	onWake  WokeFunc
	stop    chan bool
}

// WokeFunc is the callback invoked when a time lapse is detected.
type WokeFunc func(diff time.Duration)

// StartWokeWatch begins watching for conditions indicating when a time lapse
// has occurred. It will invoke onWake when the time difference detected is
// larger than diffMin, checking for lapses once every delay.
func StartWokeWatch(delay time.Duration, diffMin time.Duration, onWake WokeFunc) *Woke {
	w := &Woke{
		delay:   delay,
		diffMin: diffMin,
		onWake:  onWake,
		stop:    make(chan bool, 1),
	}

	go w.start()

	return w
}

// Stop will terminate the watcher (if running).
func (w *Woke) Stop() {
	w.stop <- true
}

//--------------------------------------------------------------------------------
// private

func (w *Woke) start() {
	for {
		timer := time.NewTimer(w.delay)
		start := time.Now()
		select {
		case <-w.stop:
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
			elapsed := time.Since(start)
			diff := elapsed - w.delay
			if diff > w.diffMin {
				w.onWake(diff)
			}
		}
	}
}
