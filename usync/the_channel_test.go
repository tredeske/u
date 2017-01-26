package usync

import (
	"sync"
	"testing"
	"time"
)

func TesItChan(t *testing.T) {

	times := 17
	var ch ItChan
	ch = make(chan interface{}, 8)

	//
	// test get from empty chan
	//
	it, ok := ch.GetTry()
	if ok {
		t.Fatalf("Got something from chan when shouldn't have")
	}

	it, ok = ch.GetWait(time.Millisecond)
	if ok {
		t.Fatalf("Got something from chan when shouldn't have")
	}

	//
	// test PutWait
	//
	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		for i := 0; i < times; i++ {
			if i < len(ch) {
				ch.Put(i)
			} else {
				ch.PutWait(i, time.Second)
			}
		}
		ok := ch.PutRecover(-1)
		if ok {
			t.Fatalf("should not have been successful")
		}
		wg.Done()
	}()

	for i := 0; i < times; i++ {
		it, ok = ch.Get()
		if !ok {
			t.Fatalf("channel unreadable!")
		}
		v := it.(int)
		if v != i {
			t.Fatalf("incorrect value!")
		}
	}
	close(ch) // force feeder to pop out of PutRecover
	wg.Wait() // wait for feeder
}
