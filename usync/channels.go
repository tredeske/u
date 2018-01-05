package usync

import (
	"time"
)

//
// a channel to signal death
//
type DeathChan chan struct{}

//
// a new channel of death!
//
func NewDeathChan() (rv DeathChan) {
	return make(chan struct{})
}

//
// writer: signal to any reader it's time to die
//
func (this DeathChan) Close() {
	defer IgnorePanic()
	close(this)
}

//
// reader: check to see if it's time to die
//
func (this DeathChan) Check() (timeToDie bool) {
	select {
	case _, ok := <-this:
		timeToDie = !ok
	default:
	}
	return
}

//
// reader: wait up to timeout for death to occur
//
func (this DeathChan) Wait(timeout time.Duration) (timeToDie bool) {
	timeToDie = this.Check()
	if !timeToDie {
		t := time.NewTimer(timeout)
		select {
		case _, ok := <-this:
			timeToDie = !ok
			t.Stop()
		case <-t.C:
		}
	}
	return
}

//
//
// ---------------------------------------------------------------------
//
//

//
// A generic channel
//
type ItChan chan interface{}

func NewItChan(capacity int) (rv ItChan) {
	return make(chan interface{}, capacity)
}

//
// close this, ignoring any panic if already closed
//
func (this ItChan) Close() {
	defer recover()
	close(this)
}

//
// get an item, wait forever or until closed.  return false if closed
//
func (this ItChan) Get() (rv interface{}, ok bool) {
	rv, ok = <-this
	return
}

//
// try to get an item, returning immediately, return true of got an item
//
func (this ItChan) GetTry() (rv interface{}, ok bool) {
	select {
	case rv, ok = <-this:
	default:
	}
	return
}

//
// try to get an item, waiting up to duration time, return true of got an item
//
func (this ItChan) GetWait(d time.Duration) (rv interface{}, ok bool) {
	rv, ok = this.GetTry()
	if !ok && 0 != d {
		t := time.NewTimer(d)
		select {
		case rv, ok = <-this:
			t.Stop()
		case <-t.C:
		}
	}
	return
}

//
// put an item into this, waiting forever
//
// the item is automatically wrapped in a Delayed
//
func (this ItChan) Put(it interface{}) {
	this <- it
}

//
// put an item into this, waiting forever, recover in case of chan close,
// returning false if chan closed
//
// in general, the writer should 'know' the chan is closed because they
// closed it, but there are sometimes cases where this is not true
//
func (this ItChan) PutRecover(it interface{}) (ok bool) {
	defer recover()
	this <- it
	ok = true
	return
}

//
// try to put an item into this, waiting no more than specified,
// returning OK if successful
//
func (this ItChan) PutWait(it interface{}, d time.Duration) (ok bool) {
	select {
	case this <- it:
		ok = true
	default:
		if 0 != d {
			t := time.NewTimer(d)
			select {
			case this <- it:
				ok = true
				t.Stop()
			case <-t.C:
			}
		}
	}
	return
}

//
// try to put an item into this, waiting no more than specified,
// returning OK if successful and not closed
//
// in general, the writer should 'know' the chan is closed because they
// closed it, but there are sometimes cases where this is not true
//
func (this ItChan) PutWaitRecover(it interface{}, d time.Duration) (ok bool) {
	defer recover()
	return this.PutWait(it, d)
}
