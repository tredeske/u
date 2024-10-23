package usync

import (
	"testing"
)

func TestPanicChanSend(t *testing.T) {
	c := make(chan struct{})
	close(c)

	defer func() {
		if !IgnoreClosedChanPanic(recover()) {
			panic("did not detect panic!")
		}
	}()

	c <- struct{}{} // will panic
	t.Fatalf("should not get here")
}

func TestPanicChanSendBare(t *testing.T) {
	c := make(chan struct{})
	close(c)

	defer BareIgnoreClosedChanPanic()

	c <- struct{}{} // will panic
	t.Fatalf("should not get here")
}
