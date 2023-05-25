package usync

import (
	"sync/atomic"
)

// boolean that is safe to access by multiple threads
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

// return true if able to clear
func (this *AtomicBool) ClearUnlessClear() (changed bool) {
	return atomic.CompareAndSwapInt64(&this.v, 1, 0)
}

// smaller boolean that is safe to access by multiple threads
type AtomicBool32 struct {
	v int32
}

func (this *AtomicBool32) IsSet() bool {
	return 0 != atomic.LoadInt32(&this.v)
}

func (this *AtomicBool32) Clear() {
	atomic.StoreInt32(&this.v, 0)
}

func (this *AtomicBool32) Set() {
	atomic.StoreInt32(&this.v, 1)
}

// return true if able to set
func (this *AtomicBool32) SetUnlessSet() (changed bool) {
	return atomic.CompareAndSwapInt32(&this.v, 0, 1)
}

// return true if able to clear
func (this *AtomicBool32) ClearUnlessClear() (changed bool) {
	return atomic.CompareAndSwapInt32(&this.v, 1, 0)
}

//------------------------------------------------------------------

// a set of 64 booleans that are safe to access by multiple threads
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

// a set of 32 booleans that are safe to access by multiple threads
type AtomicBools32 struct {
	v uint32
}

func (this *AtomicBools32) GetAll() uint32 {
	return atomic.LoadUint32(&this.v)
}

func (this *AtomicBools32) SetAll(newV uint32) {
	atomic.StoreUint32(&this.v, newV)
}

// return true if able to set
func (this *AtomicBools32) SetAllUnlessSet(oldV, newV uint32) (changed bool) {
	return atomic.CompareAndSwapUint32(&this.v, oldV, newV)
}

func (this *AtomicBools32) IsSet(bit int) bool {
	return 0 != ((1 << bit) & atomic.LoadUint32(&this.v))
}

func (this *AtomicBools32) Clear(bit int) {
retry:
	oldV := atomic.LoadUint32(&this.v)
	newV := oldV &^ (1 << bit)
	if !atomic.CompareAndSwapUint32(&this.v, oldV, newV) {
		goto retry
	}
}

func (this *AtomicBools32) Set(bit int) {
retry:
	oldV := atomic.LoadUint32(&this.v)
	newV := oldV | (1 << bit)
	if !atomic.CompareAndSwapUint32(&this.v, oldV, newV) {
		goto retry
	}
}

// return true if able to set
func (this *AtomicBools32) SetUnlessSet(bit int) (changed bool) {
retry:
	oldV := atomic.LoadUint32(&this.v)
	newV := oldV | (1 << bit)
	if oldV != newV {
		if !atomic.CompareAndSwapUint32(&this.v, oldV, newV) {
			goto retry
		}
		return true
	}
	return false
}
