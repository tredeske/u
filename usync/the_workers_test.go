package usync

import "testing"

func TestWorkers(t *testing.T) {

	ai := AtomicInt{}
	pool := Workers{}

	pool.Go(2,

		func(req interface{}) {
			ai.Add(1)
		})

	if pool.IsDrained() {
		t.Fatalf("should not happen")
	} else if pool.IsDraining() {
		t.Fatalf("should not happen")
	}

	for i := 0; i < 10; i++ {
		pool.Put(i)
	}

	pool.Close()
	pool.WaitDone()

	if 10 != ai.Get() {
		t.Fatalf("did not get everything")
	}

	pool.Drain()
	if !pool.IsDrained() {
		t.Fatalf("should not happen")
	} else if !pool.IsDraining() {
		t.Fatalf("should not happen")
	}
}
