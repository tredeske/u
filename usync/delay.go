package usync

import (
	"time"

	"github.com/tredeske/u/ulog"
)

// All values added to Delayer are wrapped in one of these so that delay time
// can be properly accounted for.
//
// The Put() methods do this automatically, but you may need to use the Wrap()
// method if using InC directly.
type Delayed struct {
	Deadline time.Time
	Value    any
}

// Delays things
type Delayer struct {
	Name      string          //
	Cap       int             // capacity - max items to hold
	Delay     time.Duration   // amount of time to delay each item
	InC       chan Delayed    // where to put things, or use Put() methods
	shutdownC chan struct{}   //
	OnItem    func(value any) // call for each item after delay
	OnClose   func()          // if set, call after InC closed and drained
}

// wrap value in Delayed (if using InC directly)
func (this Delayer) Wrap(value any) (rv Delayed) {
	return Delayed{
		Deadline: time.Now().Add(this.Delay),
		Value:    value,
	}
}

// Start this up
func (this *Delayer) Open() {
	if 0 == this.Delay {
		panic("no delay set")
	} else if 2 > this.Cap {
		panic("invalid Cap")
	} else if nil == this.OnItem {
		panic("OnItem not set")
	}

	if nil == this.InC {
		this.InC = make(chan Delayed, this.Cap)
	}
	this.shutdownC = make(chan struct{}) // must be non-buffered

	go this.run()
}

// close this down
//
// InC will be drained and all items will be processed with normal delay
func (this Delayer) Close() {
	close(this.InC)
}

// Shutdown this ASAP
//
// All items will be discarded and all delays cancelled
func (this Delayer) Shutdown() {
	defer LogPanic(recover(), "shutting down sync.Delayer")
	close(this.shutdownC)
	close(this.InC)
}

// Cause all items to be discarded
//
// Once drained, use Plug() to ready Delayer for use
func (this Delayer) Drain() {
	this.shutdownC <- struct{}{} // synchronous
}

// Are all items drained?
func (this Delayer) IsDrained() bool {
	return 0 == len(this.InC)
}

// Return Delayer to service
func (this Delayer) Plug() {
	this.shutdownC <- struct{}{} // synchronous
}

// main loop
func (this Delayer) run() {

	if 0 == len(this.Name) {
		this.Name = "Delayer"
	}
	defer func() {
		if it := recover(); nil != it {
			ulog.Warnf("%s: delayer panic: %s", this.Name, it)
		}
	}()

	var timer *time.Timer
	var delayed Delayed
	var ok bool
	var draining bool
	var send bool
	var timerC <-chan time.Time
	inC := this.InC

loop:
	for {

		select {
		case _, ok = <-this.shutdownC:
			if !ok {
				break loop // shutdown - we're done //////////////////////////
			}

			//
			// toggle drain/resume
			//
			draining = !draining
			if nil != timerC {
				timer.Stop()
				timerC = nil
			}
			inC = this.InC // make sure input enabled
			send = false
			continue

		case delayed, ok = <-inC:
			if !ok {
				break loop // closed - we're done //////////////////////////
			} else if draining {
				continue
			}
			inC = nil // disable input

			//
			// compute delay til when we should release the item
			//
			delay := time.Until(delayed.Deadline)
			if 100 < delay {
				if nil == timer {
					timer = time.NewTimer(delay)
				} else {
					timer.Stop()
					timer.Reset(delay)
				}
				timerC = timer.C
			} else {
				send = true
			}

		case <-timerC:
			timerC = nil // disable timer chan select
			send = true
		}

		if send {
			this.OnItem(delayed.Value)
			send = false
			inC = this.InC // enable input
		}
	}

	if nil != timerC {
		timer.Stop()
	}
	if nil != this.OnClose {
		this.OnClose()
	}
}

// put an item into this, waiting forever
//
// the item is automatically wrapped in a Delayed
func (this Delayer) Put(it any) {
	this.InC <- this.Wrap(it)
}

// put an item into this, waiting forever
//
// if chan closed, then return false
//
// the item is automatically wrapped in a Delayed
func (this Delayer) PutWaitRecover(it any, timeout time.Duration) (ok bool) {
	defer recover()
	return this.PutWait(it, timeout)
}

// try to put an item into this, waiting no more than specified,
// returning OK if successful
//
// the item is automatically wrapped in a Delayed
func (this Delayer) PutWait(it any, d time.Duration) (ok bool) {
	select {
	case this.InC <- this.Wrap(it):
		ok = true
	default:
		if 0 != d {
			t := time.NewTimer(d)
			select {
			case this.InC <- this.Wrap(it):
				ok = true
			case <-t.C:
			}
			t.Stop()
		}
	}
	return
}

//
// ------------------------------------------------------------------
//

// A channel with a time delay
//
// Items inserted to InC become available on OutC after delay.
type DelayChan struct {
	Delayer
	OutC Chan[any] // where to get delayed things
}

func (this *DelayChan) Open() {
	if nil == this.OutC {
		this.OutC = NewChan[any](this.Cap)
	}
	this.OnItem = func(v any) { this.OutC <- v }
	this.OnClose = func() { close(this.OutC) }
	this.Delayer.Open()
}

// Shutdown and drain OutC in the background, disgarding all items
func (this DelayChan) ShutdownAndDrain() {
	this.Delayer.Shutdown()
	go func() {
		for _ = range this.OutC {
		}
	}()
}

// get an item from this, waiting forever
// false indicates channel closed and not item received
func (this DelayChan) Get() (rv any, ok bool) {
	rv, ok = <-this.OutC
	return
}

// try to get an item from this, without waiting, returning true if got item
// false may indicate channel closed
func (this DelayChan) TryGet() (rv any, ok bool) {
	return this.OutC.TryGet()
}

// try to get an item from this, waiting at most specified time,
// returning true if got item, false may indicate channel closed
func (this DelayChan) GetWait(d time.Duration) (rv any, ok bool) {
	return this.OutC.GetWait(d)
}
