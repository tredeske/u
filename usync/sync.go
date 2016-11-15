package usync

import (
	"strconv"
	"sync/atomic"
)

//
// boolean that is safe to access by multiple threads
//
type AtomicBool struct {
	v int64
}

func (this *AtomicBool) IsSet() bool {
	return 0 != atomic.LoadInt64(&this.v)
}

func (this *AtomicBool) Clear() {
	atomic.StoreInt64(&this.v, 0)
}

func (this *AtomicBool) Set() {
	atomic.StoreInt64(&this.v, 1)
}

// return true if able to set
func (this *AtomicBool) SetUnlessSet() (changed bool) {
	return atomic.CompareAndSwapInt64(&this.v, 0, 1)
}

//------------------------------------------------------------------

type AtomicInt struct {
	v int64
}

func (this *AtomicInt) Add(amount int64) (result int64) {
	return atomic.AddInt64(&this.v, amount)
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
