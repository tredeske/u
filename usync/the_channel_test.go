package usync

import (
	"testing"
	"time"
)

func TesItChan(t *testing.T) {

	times := 17
	var ch ItChan = make(chan interface{}, 8)

	//
	// test get from empty chan
	//
	it, ok := ch.GetTry()
	if ok {
		t.Fatalf("Got something from chan when shouldn't have")
	} else if nil != it {
		t.Fatalf("it should be nil")
	}

	it, ok = ch.GetWait(time.Millisecond)
	if ok {
		t.Fatalf("Got something from chan when shouldn't have")
	} else if nil != it {
		t.Fatalf("it should be nil")
	}

	//
	// test PutWait
	//

	resultC := make(chan bool)

	go func() {
		for i := 0; i < times; i++ {
			if i < len(ch) {
				ch.Put(i)
			} else {
				ch.PutWait(i, time.Second)
			}
		}
		ok := ch.PutRecover(-1)
		resultC <- ok
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
	close(ch)      // force feeder to pop out of PutRecover
	ok = <-resultC // wait for it
	if ok {
		t.Fatalf("should not have been successful")
	}
}
