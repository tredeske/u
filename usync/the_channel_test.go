package usync

import (
	"testing"
	"time"
)

func TestGenericChan(t *testing.T) {
	const times = 17

	ch := NewChan[int](8)

	//
	// test get from empty chan
	//
	it, ok := ch.TryGet()
	if ok {
		t.Fatalf("Got something from chan when shouldn't have")
	} else if 0 != it {
		t.Fatalf("it should be zero")
	}

	it, ok = ch.GetWait(time.Millisecond)
	if ok {
		t.Fatalf("Got something from chan when shouldn't have")
	} else if 0 != it {
		t.Fatalf("it should be zero")
	}

	//
	// test PutWait
	//

	resultC := make(chan bool)

	go func() {
		for i := 0; i < times; i++ {
			if !ch.PutWait(i, time.Second) {
				resultC <- false
				return
			}
		}
		resultC <- true
	}()

	time.Sleep(100 * time.Millisecond)

	for i := 0; i < times; i++ {
		it, ok = ch.Get()
		if !ok {
			t.Fatalf("channel unreadable!")
		}
		v := it
		if v != i {
			t.Fatalf("incorrect value!")
		}
	}
	ok = <-resultC
	if !ok {
		t.Fatalf("should have been successful")
	}
}

func TestAnyChan(t *testing.T) {

	const times = 17
	var ch Chan[any] = NewChan[any](8)

	//
	// test get from empty chan
	//
	it, ok := ch.TryGet()
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
			if !ch.PutWait(i, time.Second) {
				resultC <- false
				return
			}
		}
		resultC <- true
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
	ok = <-resultC // wait for it
	if !ok {
		t.Fatalf("should have been successful")
	}
	close(ch)
}
