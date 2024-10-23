package usync

import (
	"fmt"
	"testing"
	"time"
)

func TestProc(t *testing.T) {
	ps := newProcSvc(t, 5, time.Second)

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
	t       testing.TB
	proc    Proc
	result  int
	timeout time.Duration
}

func newProcSvc(t testing.TB, depth int, timeout time.Duration) (rv *procSvc_) {
	rv = &procSvc_{
		timeout: timeout,
	}
	rv.proc.Construct(depth)
	go rv.run()
	return
}

func (ps *procSvc_) AsyncAdd(amount int) {
	ok := ps.proc.Async(func() (svcErr error) {
		ps.result += amount
		return nil
	})
	if !ok {
		ps.t.Fatalf("async call not accepted!")
	}
}

func (ps *procSvc_) SyncAddGet(amount int) (rv int) {
	ok, err := ps.proc.Call(func() (svcErr error) {
		ps.result += amount
		rv = ps.result
		return nil
	})
	if err != nil {
		ps.t.Fatalf("sync call failed! %s", err)
	} else if !ok {
		ps.t.Fatalf("sync call not accepted!")
	}
	return
}

func (ps *procSvc_) Close() {
	close(ps.proc.ProcC)
}

func (ps *procSvc_) run() {
	t := time.NewTimer(ps.timeout)
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

func BenchmarkProcSync(b *testing.B) {
	ps := newProcSvc(b, 256, 15*time.Second)
	defer ps.Close()

	total := 0
	for i := 0; i < b.N; i++ {
		total += ps.SyncAddGet(8)
	}
	if 0 == total {
		b.Fatalf("got %d!", total)
	}
}
func BenchmarkProcAsync(b *testing.B) {
	ps := newProcSvc(b, 256, 15*time.Second)
	defer ps.Close()

	for i := 0; i < b.N; i++ {
		ps.AsyncAdd(8)
	}
}
