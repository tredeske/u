package uerr

import (
	"testing"
)

func TestPanicChanSend(t *testing.T) {
	c := make(chan struct{})
	close(c)

	defer func() {
		if !IfClosedChanPanic(recover()) {
			panic("did not detect panic!")
		}
	}()

	c <- struct{}{} // will panic
	t.Fatalf("should not get here")
}

func TestPanicChanSendBare(t *testing.T) {
	c := make(chan struct{})
	close(c)

	defer IgnoreClosedChanPanic()

	c <- struct{}{} // will panic
	t.Fatalf("should not get here")
}
