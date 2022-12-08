package unet

import (
	"testing"
	"time"
)

func TestDeadliner(t *testing.T) {
	const timeout = 50 * time.Millisecond
	doneC := make(chan struct{})
	startT := time.Now()
	d := newDeadliner(startT.Add(timeout),
		func() {
			close(doneC)
		})

	d.Reset(startT.Add(2 * timeout))

	<-doneC

	elapsed := time.Since(startT)
	if elapsed < 2*timeout {
		t.Fatalf("deadline fired too soon: %s", elapsed)
	}

	d.Cancel()
}
