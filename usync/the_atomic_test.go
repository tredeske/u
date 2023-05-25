package usync

import (
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

func BenchmarkChanPoll(b *testing.B) {
	var result uint64
	c := make(chan struct{}, 8)
	var done AtomicBool
	go func() {
		for !done.IsSet() {
			c <- struct{}{}
			runtime.Gosched()
		}
	}()
	for i := 0; i < b.N; i++ {
		select {
		case <-c:
			result++
			//b.Fatalf("should not get here: %v %v", s, ok)
		default:
		}
	}
	done.Set()
	//fmt.Printf("Result: %d\n", result)
}

var (
	atomicBool_      AtomicBool
	atomicInt_       atomic.Int64
	atomicMutex_     sync.Mutex
	atomicMutexBool_ bool
)

func BenchmarkAtomicMutexPoll(b *testing.B) {
	var result uint64
	var done AtomicBool
	go func() {
		for !done.IsSet() {
			atomicMutex_.Lock()
			atomicMutexBool_ = true
			atomicMutex_.Unlock()
			runtime.Gosched()
		}
	}()
	for i := 0; i < b.N; i++ {
		atomicMutex_.Lock()
		v := atomicMutexBool_
		atomicMutex_.Unlock()
		if v {
			result++
		}
	}
	done.Set()
	//fmt.Printf("Result: %d\n", result)
}

func BenchmarkAtomicBoolClearUnlessPoll(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if atomicBool_.ClearUnlessClear() {
			b.Fatalf("should not get here")
		}
	}
}

func BenchmarkAtomicBoolIsSetPoll(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if atomicBool_.IsSet() {
			atomicBool_.Clear()
			b.Fatalf("should not get here")
		}
	}
}

func BenchmarkAtomicIntPoll(b *testing.B) {
	var result uint64
	var done AtomicBool
	go func() {
		for !done.IsSet() {
			atomicInt_.Add(1)
			runtime.Gosched()
		}
	}()
	for i := 0; i < b.N; i++ {
		v := atomicInt_.Load()
		if 0 != v {
			atomicInt_.Add(-v)
			result += uint64(v)
			//b.Fatalf("should not get here")
		}
	}
	done.Set()
	//fmt.Printf("Result: %d\n", result)
}
