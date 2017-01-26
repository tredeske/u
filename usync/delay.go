package usync

import "time"

//
// All values added to Delayer are wrapped in one of these so that delay time
// can be properly accounted for.
//
// The Put() methods do this automatically, but you may need to use the Wrap()
// method if using InC directly.
//
type Delayed struct {
	Deadline time.Time
	Value    interface{}
}

//
// Delays things
//
type Delayer struct {
	Cap       int                     // capacity - max items to hold
	Delay     time.Duration           // amount of time to delay each item
	InC       chan Delayed            // where to put things, or use Put() methods
	shutdownC chan struct{}           //
	OnItem    func(value interface{}) // call for each item after delay
	OnClose   func()                  // if set, call after InC closed and drained
}

//
// wrap value in Delayed (if using InC directly)
//
func (this Delayer) Wrap(value interface{}) (rv Delayed) {
	return Delayed{
		Deadline: time.Now().Add(this.Delay),
		Value:    value,
	}
}

//
// Start this up
//
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
	this.shutdownC = make(chan struct{})

	go this.run()
}

//
// close this down
//
// InC will be drained and all items will be processed with normal delay
//
func (this Delayer) Close() {
	close(this.InC)
}

//
// Shutdown this ASAP
//
// All items will be discarded and all delays cancelled
//
func (this Delayer) Shutdown() {
	close(this.shutdownC)
	close(this.InC)
}

//
// main loop
//
func (this Delayer) run() {

	var timer *time.Timer
	var delayed Delayed
	var ok bool

loop:
	for {

		//
		// await input
		//
		select {
		case <-this.shutdownC:
			break loop // shutdown - we're done //////////////////////////
		case delayed, ok = <-this.InC:
			if !ok {
				break loop // closed - we're done //////////////////////////
			}
		}

		//
		// send when ready
		//
		delay := delayed.Deadline.Sub(time.Now())
		if 100 < delay {
			if nil == timer {
				timer = time.NewTimer(delay)
			} else {
				timer.Stop()
				timer.Reset(delay)
			}
			select {
			case <-this.shutdownC:
				timer.Stop()
				break loop // shutdown - we're done //////////////////////////
			case <-timer.C:
			}
		}
		this.OnItem(delayed.Value)
	}

	if nil != this.OnClose {
		this.OnClose()
	}
}

//
// put an item into this, waiting forever
//
// the item is automatically wrapped in a Delayed
//
func (this Delayer) Put(it interface{}) {
	this.InC <- this.Wrap(it)
}

//
// try to put an item into this without waiting, returning OK if successful
//
// the item is automatically wrapped in a Delayed
//
func (this Delayer) PutTry(it interface{}) (ok bool) {
	select {
	case this.InC <- this.Wrap(it):
		ok = true
	default:
	}
	return
}

//
// try to put an item into this, waiting no more than specified,
// returning OK if successful
//
// the item is automatically wrapped in a Delayed
//
func (this Delayer) PutWait(it interface{}, d time.Duration) (ok bool) {
	ok = this.PutTry(it)
	if !ok && 0 != d {
		t := time.NewTimer(d)
		select {
		case this.InC <- this.Wrap(it):
			ok = true
		case <-t.C:
		}
		t.Stop()
	}
	return
}

//
// ------------------------------------------------------------------
//

//
// A channel with a time delay
//
// Items inserted to InC become available on OutC after delay.
//
type DelayChan struct {
	Delayer
	OutC ItChan // where to get delayed things
}

//
//
//
func (this *DelayChan) Open() {
	if nil == this.OutC {
		this.OutC = make(chan interface{}, this.Cap)
	}
	this.OnItem = func(v interface{}) { this.OutC <- v }
	this.OnClose = func() { close(this.OutC) }
	this.Delayer.Open()
}

//
// Shutdown and drain OutC in the background, disgarding all items
//
func (this DelayChan) ShutdownAndDrain() {
	this.Delayer.Shutdown()
	go func() {
		for _ = range this.OutC {
		}
	}()
}

//
// get an item from this, waiting forever
// false indicates channel closed and not item received
//
func (this DelayChan) Get() (rv interface{}, ok bool) {
	rv, ok = <-this.OutC
	return
}

//
// try to get an item from this, without waiting, returning true if got item
// false may indicate channel closed
//
func (this DelayChan) GetTry() (rv interface{}, ok bool) {
	return this.OutC.GetTry()
}

//
// try to get an item from this, waiting at most specified time,
// returning true if got item, false may indicate channel closed
//
func (this DelayChan) GetWait(d time.Duration) (rv interface{}, ok bool) {
	return this.OutC.GetWait(d)
}
