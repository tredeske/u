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

//
// a set of 64 booleans that are safe to access by multiple threads
//
type AtomicBools struct {
	v uint64
}

func (this *AtomicBools) GetAll() uint64 {
	return atomic.LoadUint64(&this.v)
}

func (this *AtomicBools) SetAll(newV uint64) {
	atomic.StoreUint64(&this.v, newV)
}

// return true if able to set
func (this *AtomicBools) SetAllUnlessSet(oldV, newV uint64) (changed bool) {
	return atomic.CompareAndSwapUint64(&this.v, oldV, newV)
}

func (this *AtomicBools) IsSet(bit int) bool {
	return 0 != ((1 << bit) & atomic.LoadUint64(&this.v))
}

func (this *AtomicBools) Clear(bit int) {
retry:
	oldV := atomic.LoadUint64(&this.v)
	newV := oldV &^ (1 << bit)
	if !atomic.CompareAndSwapUint64(&this.v, oldV, newV) {
		goto retry
	}
}

func (this *AtomicBools) Set(bit int) {
retry:
	oldV := atomic.LoadUint64(&this.v)
	newV := oldV | (1 << bit)
	if !atomic.CompareAndSwapUint64(&this.v, oldV, newV) {
		goto retry
	}
}

// return true if able to set
func (this *AtomicBools) SetUnlessSet(bit int) (changed bool) {
retry:
	oldV := atomic.LoadUint64(&this.v)
	newV := oldV | (1 << bit)
	if oldV != newV {
		if !atomic.CompareAndSwapUint64(&this.v, oldV, newV) {
			goto retry
		}
		return true
	}
	return false
}

//------------------------------------------------------------------

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
