package usync

import "sync"

//
// Unlike sync.Pool, this does not lose things.  Use to share/reuse values
// between multiple threads.
//
// Meant to enable reuse of expensive to create values that need to be
// cleaned up properly at some point.
//
// Do not change New or Delete once the pool is in use.
//
// If set, New() will manufacture a new value if no values are in the cache.
// New() will be called while locked, so only one caller at a time will be
// in New() at a time.
//
// If New() is not set or Stop() has been called, and Delete() is set,
// the Put() will result in Delete() being called instead of caching the
// value.  This enables orderly shutdown after Stop().
//
type Pool struct {
	New    func() interface{}  // if set, manufacture a new value
	Delete func(v interface{}) // if set and New not set, use to dispose of Puts
	lock   sync.Mutex          //
	values []interface{}       //
}

//
// prevent allocation of new objects from this point on
//
// if Delete set, run it on all current values and clear the cache
//
func (this *Pool) Stop() {

	this.lock.Lock()
	defer this.lock.Unlock()

	this.New = nil

	if nil != this.Delete {
		for _, it := range this.values {
			this.Delete(it)
		}
		this.values = nil
	}
}

//
// try to get an existing object, or create a new one
//
func (this *Pool) Get() (rv interface{}) {

	this.lock.Lock()
	defer this.lock.Unlock()

	if 0 == len(this.values) {
		if nil != this.New {
			rv = this.New()
		}
	} else {
		last := len(this.values) - 1
		rv = this.values[last]
		this.values = this.values[:last]
	}
	return
}

//
// Put a value back into the pool.
//
// If New is not set and Delete is set, then Delete will be called on value
//
func (this *Pool) Put(v interface{}) {

	if nil == v { // programming error, so panic
		panic("attempt to put a nil value into pool")
	}

	this.lock.Lock()
	defer this.lock.Unlock()

	if nil == this.New && nil != this.Delete {
		this.Delete(v)
	} else {
		this.values = append(this.values, v)
	}
}
