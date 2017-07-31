package usync

import "time"

//
// A ticker that takes an initial time and a period
//
// Just like time.Ticker but with an initial time
//
type Ticker struct {
	initial time.Duration
	period  time.Duration
	timeC   chan time.Time

	C <-chan time.Time // user gets time on this (just like time.Ticker)
}

//
// create a ticker that will emit a time after an initial time, and then
// periodically after that
//
func NewTicker(initial, period time.Duration) (rv *Ticker) {
	rv = &Ticker{
		initial: initial,
		period:  period,
		timeC:   make(chan time.Time),
	}
	rv.C = rv.timeC
	go rv.run()
	return
}

//
// make sure to Stop the ticker when done!
//
func (this *Ticker) Stop() {
	defer func() { recover() }()
	close(this.timeC)
}

func (this *Ticker) run() {
	//
	// on Stop, we'll panic, so recover the panic and exit the goroutine
	//
	defer func() { recover() }()

	time.Sleep(this.initial)
	for {
		select {
		case this.timeC <- time.Now().UTC():
		default:
		}
		time.Sleep(this.period)
	}
}
