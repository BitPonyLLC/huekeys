package util

import "time"

type Woke struct {
	delay   time.Duration
	diffMin time.Duration
	onWake  WokeFunc
	stop    chan bool
}

type WokeFunc func(diff time.Duration)

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

func (w *Woke) Stop() {
	w.stop <- true
}

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
