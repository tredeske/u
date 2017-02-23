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

func TestWorkGang(t *testing.T) {

	maxReqs := 10
	reqs := 0
	reqsRx := 0
	reqsRxHighest := -1

	wg := WorkGang{
		OnFeed: func() (req interface{}, ok bool) {
			if reqs < maxReqs {
				req = reqs
				reqs++
			}
			return req, true
		},

		OnRequest: func(req interface{}) (resp interface{}, ok bool) {
			return req, true
		},

		OnResponse: func(resp interface{}) (ok bool) {
			v := 0
			v, ok = resp.(int)
			if ok {
				reqsRx++
				if reqsRxHighest < v {
					reqsRxHighest = v
				}
			}
			return
		},
	}

	wg.Work(3)

	if reqsRx != maxReqs {
		t.Fatalf("Received %d instead of %d", reqsRx, maxReqs)
	} else if reqsRxHighest != maxReqs-1 {
		t.Fatalf("Highest was %d instead of %d", reqsRxHighest, maxReqs-1)
	}
}
