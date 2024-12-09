package uerr

import (
	"strings"
)

// Use in a defer func when sending on or closing a chan that might be closed.
//
//	var thingC chan something
//	defer func() {
//		if uerr.IfClosedChanPanic(recover()) {
//			// ...
//		}
//	}()
//	thingC <- thing
//
// Note the outer func.  The recover() must be done in the func that defer calls.
//
// Sending on a closed chan will cause a panic with an error:
// "send on closed channel"
//
// Closing a closed chan will cause a panic with an error:
// "close of closed channel"
//
// Return true if a panic is detected and it is due to a closed chan.  Return false
// if no panic detected.  If some other panic occurs, re-panic.
func IfClosedChanPanic(recovered any) (ignored bool) {
	if nil != recovered {
		if e, ok := recovered.(error); !ok ||
			-1 == strings.Index(e.Error(), "closed channel") {
			panic(recovered)
		}
		ignored = true
	}
	return
}

// Use directly with defer to ignore panic due to sending on or closing a chan that
// might be closed.
//
// Note: no outer func!  This cannot be used within a func.
//
//	var thingC chan something
//	func f(thing something) (err error) {
//
//		defer uerr.IgnoreClosedChanPanic() // note no outer func!
//
//		err = errDidNotWork // set an error in case of panic
//		thingC <- thing
//		return nil // clear error on success
//	}
func IgnoreClosedChanPanic() {
	IfClosedChanPanic(recover())
}

// Ignore any panics in activity().
//
// Useful in defer funcs when attempting to clean up.
func IgnorePanicIn(activity func()) {
	defer func() { recover() }()
	activity()
}
