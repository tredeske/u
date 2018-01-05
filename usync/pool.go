package usync

import "sync"

//
// Unlike sync.Pool, this does not lose things
//
// New() will be called while locked, so only one caller at a time will be
// allowed to use it.
//
type Pool struct {
	New    func() interface{} // if set, manufacture a new value
	lock   sync.Mutex         //
	values []interface{}      //
}

//
// prevent allocation of new objects from this point on
//
func (this *Pool) Stop() {

	this.lock.Lock()
	defer this.lock.Unlock()

	this.New = nil
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
// try to get an existing object, or create a new one
//
func (this *Pool) Put(v interface{}) {

	this.lock.Lock()
	defer this.lock.Unlock()

	this.values = append(this.values, v)
}
