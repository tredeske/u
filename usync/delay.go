package usync

import "time"

//
// A channel with a time delay
//
type DelayChan struct {
	InC   chan Delayed     // where to put things
	OutC  chan interface{} // where to get delayed things
	Delay time.Duration    // amount of time to delay
	Cap   int              // capacity
}

//
// All values added to this are wrapped in one of these so that delay time
// can be properly accounted for.
//
// The Put methods do this automatically, but you may need to use the Wrap()
// method if using InC directly.
//
type Delayed struct {
	Deadline time.Time
	Value    interface{}
}

//
// wrap value in Delayed (if using InC directly)
//
func (this DelayChan) Wrap(value interface{}) (rv Delayed) {
	return Delayed{
		Deadline: time.Now().Add(this.Delay),
		Value:    value,
	}
}

//
//
//
func (this *DelayChan) Open() {
	if 0 == this.Delay {
		panic("no delay set")
	} else if 2 > this.Cap {
		panic("invalid Cap")
	}

	if nil == this.InC {
		this.InC = make(chan Delayed, this.Cap)
	}
	if nil == this.OutC {
		this.OutC = make(chan interface{}, this.Cap)
	}

	go this.run()
}

//
// main loop
//
func (this *DelayChan) run() {

	var timer *time.Timer

	defer close(this.OutC)

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
		if 0 < delay {
			if nil == timer {
				timer = time.NewTimer(delay)
			} else {
				timer.Stop()
				timer.Reset(delay)
			}
			<-timer.C
		}
		this.OutC <- delayed.Value
	}
}

//
// close this
//
func (this DelayChan) Close() {
	close(this.InC)
}

//
// put an item into this, waiting forever
//
func (this DelayChan) Put(it interface{}) {
	this.InC <- this.Wrap(it)
}

//
// try to put an item into this without waiting, returning OK if successful
//
func (this DelayChan) TryPut(it interface{}) (ok bool) {
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
func (this DelayChan) WaitPut(it interface{}, d time.Duration) (ok bool) {
	ok = this.TryPut(it)
	if !ok {
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
	if !ok {
		t := time.NewTimer(d)
		select {
		case rv, ok = <-this.OutC:
		case <-t.C:
		}
		t.Stop()
	}
	return
}
