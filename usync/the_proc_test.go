package usync

import (
	"fmt"
	"testing"
	"time"
)

func TestProc(t *testing.T) {
	ps := newProcSvc()

	fmt.Printf(`
GIVEN: simple proc service running
 WHEN: add 5 + 7 + 8
 THEN: get 20
`)
	ps.AsyncAdd(5)
	ps.AsyncAdd(7)
	result := ps.SyncAddGet(8)
	if 20 != result {
		t.Fatalf("Expected 20, got %d", result)
	}
}

type procSvc_ struct { // a test service to add numbers
	proc   Proc
	result int
}

func newProcSvc() (rv *procSvc_) {
	rv = &procSvc_{}
	rv.proc.Construct(5)
	go rv.run()
	return
}

func (ps *procSvc_) AsyncAdd(amount int) {
	ps.proc.Async(func() (svcErr error) {
		ps.result += amount
		return nil
	})
}

func (ps *procSvc_) SyncAddGet(amount int) (rv int) {
	err := ps.proc.Call(func() (callErr, svcErr error) {
		ps.result += amount
		rv = ps.result
		return nil, nil
	})
	if err != nil {
		panic("should not get an error!")
	}
	return
}

func (ps *procSvc_) run() {
	t := time.NewTimer(time.Second)
	defer t.Stop()
	for { // sample service loop doing important things
		select {
		case <-t.C:
			panic("ran out of time!")
		case f, ok := <-ps.proc.ProcC:
			if !ok {
				return // all done
			}
			err := f()
			if err != nil {
				panic(err.Error())
			}
		}
	}

}
