package usync

import (
	"log"
	"testing"
	"time"
)

func TestDelay(t *testing.T) {

	data := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17}
	timesC := make(chan time.Time, len(data))

	dc := DelayChan{
		Delayer: Delayer{
			Delay: 100 * time.Millisecond,
			Cap:   4,
		},
	}

	dc.Open()

	//
	// feeder
	//
	go func() {
		for idx, i := range data {
			timesC <- time.Now()
			dc.Put(i)
			log.Printf("Put %d", idx)
		}

		dc.Close()
	}()

	//
	//
	//
	for idx, i := range data {
		it, ok := dc.Get()
		log.Printf("Got %d", idx)
		j := it.(int)
		if !ok {
			t.Fatalf("Unable to get %d", idx)
		} else if i != j {
			t.Fatalf("Number %d does not match (%d != %d)", idx, i, j)
		}
		tStart := <-timesC
		elapsed := time.Since(tStart)
		if dc.Delay > elapsed {
			t.Fatalf("Number %d was only delayed by %dns", idx, elapsed)
		}
		log.Printf("%d: delay=%d", idx, elapsed)
	}

	it, ok := dc.TryGet()
	if ok {
		t.Fatalf("TryGet should fail!")
	} else if nil != it {
		t.Fatalf("GetTry should not have returned non-nil value!")
	}

	it, ok = dc.Get()
	if ok {
		t.Fatalf("Did not get close")
	} else if nil != it {
		t.Fatalf("GetTry should not have returned non-nil value!")
	}

}
