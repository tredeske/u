package usync

import (
	"strconv"
	"sync/atomic"
)

type AtomicInt struct {
	v int64
}

func (this *AtomicInt) Add(amount int64) (result int64) {
	return atomic.AddInt64(&this.v, amount)
}

func (this *AtomicInt) AddIfLessThan(
	amount, lessThan int64,
) (
	result int64, added bool,
) {

retry:
	oldV := atomic.LoadInt64(&this.v)
	newV := oldV + amount
	if newV < lessThan {
		added = this.Cas(oldV, newV)
		if !added {
			goto retry
		}
		result = newV
	}
	return
}

func (this *AtomicInt) Cas(oldV, newV int64) (swapped bool) {
	return atomic.CompareAndSwapInt64(&this.v, oldV, newV)
}

func (this *AtomicInt) Get() (rv int64) {
	return atomic.LoadInt64(&this.v)
}

func (this *AtomicInt) Set(amount int64) {
	atomic.StoreInt64(&this.v, amount)
}

func (this AtomicInt) String() (rv string) {
	return strconv.FormatInt(this.v, 10)
}

//------------------------------------------------------------------

//
// a counting semaphore
//
type Semaphore chan empty_

type empty_ struct{}

var theEmpty_ = empty_{}

func NewSemaphore(size int) Semaphore {
	return make(Semaphore, size)
}

func (this Semaphore) Acquire() {
	this <- theEmpty_
}

func (this Semaphore) AcquireN(amount int) {
	for i := 0; i < amount; i++ {
		this <- theEmpty_
	}
}

func (this Semaphore) Release() {
	<-this
}

func (this Semaphore) ReleaseN(amount int) {
	for i := 0; i < amount; i++ {
		<-this
	}
}
