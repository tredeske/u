package usync

import (
	"strings"

	"github.com/tredeske/u/ulog"
)

// use in a defer when sending on a chan that might be closed.
//
//	defer func() {
//		usync.IgnorechanSendPanic(recover())
//	}()
//
// note the outer func.  the recover() must be done in the func that defer calls.
//
// sending on a closed chan will cause a panic with an error:
// "send on closed channel"
//
// closing a closed chan will cause a panic with an error:
// "close of closed channel"
//
// if this is detected, we ignore it, but if some other panic is detected, then
// we re-panic
//
// return true if such a panic was ignored.  this usually indicates the service
// on the other end of the chan is dead.
func IgnoreClosedChanPanic(recovered any) (ignored bool) {
	if nil != recovered {
		if e, ok := recovered.(error); !ok ||
			-1 == strings.Index(e.Error(), "closed channel") {
			panic(recovered)
		}
		ignored = true
	}
	return
}

// same as IgnoreClosedChanPanic, but no outer func
//
//	defer usync.BareIgnorechanSendPanic()
//
// note that there is no outer func - this can only be used if directly used by
// defer as it calls recover().
func BareIgnoreClosedChanPanic() {
	IgnoreClosedChanPanic(recover())
}

// Ignore any panics.  Prefer IgnorePanicIn instead.
//
// Use: defer usync.IgnorePanic()
//
// Do not: defer func() { usync.IgnorePanic() }
//
// note that there is no outer func - this can only be used if directly used by
// defer.
func IgnorePanic() {
	recover()
}

// Ignore any panics in activity().
func IgnorePanicIn(activity func()) {
	defer func() { recover() }()
	activity()
}

/*
// Capture any panics in activity().
func CapturePanicIn(activity func()) (captured any) {
	defer func() { captured = recover() }()
	activity()
	return
}
*/

// Log any panics.
//
// Use: defer usync.LogPanic(recover(), msg)
//
// note that there is no outer func - this can only be used if directly used by
// defer.
func LogPanic(x any, msg string) {
	if x != nil {
		if 0 != len(msg) {
			ulog.Printf("PANIC: %s: %s", msg, x)
		} else {
			ulog.Printf("PANIC: %s", x)
		}
	}
}

// Log any panics in activity().
func LogPanicIn(msg string, activity func()) {
	defer LogPanic(recover(), msg)
	activity()
}

// Log any panics and exit the program.
//
// Use: defer usync.FatalPanic(recover(), "boo")
func FatalPanic(x any, msg string) {
	if x != nil {
		if 0 != len(msg) {
			ulog.Fatalf("PANIC: %s: %s", msg, x)
		} else {
			ulog.Fatalf("PANIC: %s", x)
		}
	}
}
