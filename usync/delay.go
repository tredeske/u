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
	Cap     int                     // capacity - max items to hold
	Delay   time.Duration           // amount of time to delay each item
	InC     chan Delayed            // where to put things, or use Put() methods
	OnItem  func(value interface{}) // call for each item after delay
	OnClose func()                  // if set, call after InC closed and drained
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

	go this.run()
}

//
// close this down
//
// InC will be drained and all items will be processed
//
func (this Delayer) Close() {
	close(this.InC)
}

//
// main loop
//
func (this Delayer) run() {

	var timer *time.Timer

	for {

		//
		// await input
		//
		delayed, ok := <-this.InC
		if !ok {
			break // closed - we're done ////////////////////////////////////////
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
			<-timer.C
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
func (this Delayer) Put(it interface{}) {
	this.InC <- this.Wrap(it)
}

//
// try to put an item into this without waiting, returning OK if successful
//
func (this Delayer) TryPut(it interface{}) (ok bool) {
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
func (this Delayer) WaitPut(it interface{}, d time.Duration) (ok bool) {
	ok = this.TryPut(it)
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
	OutC chan interface{} // where to get delayed things
}

func (this *DelayChan) Open() {
	if nil == this.OutC {
		this.OutC = make(chan interface{}, this.Cap)
	}
	this.OnItem = func(v interface{}) { this.OutC <- v }
	this.OnClose = func() { close(this.OutC) }
	this.Delayer.Open()
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
func (this DelayChan) TryGet() (rv interface{}, ok bool) {
	select {
	case rv, ok = <-this.OutC:
	default:
	}
	return
}

//
// try to get an item from this, waiting at most specified time,
// returning true if got item, false may indicate channel closed
//
func (this DelayChan) WaitGet(d time.Duration) (rv interface{}, ok bool) {
	rv, ok = this.TryGet()
	if !ok && 0 != d {
		t := time.NewTimer(d)
		select {
		case rv, ok = <-this.OutC:
		case <-t.C:
		}
		t.Stop()
	}
	return
}
