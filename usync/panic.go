package usync

import (
	"github.com/tredeske/u/ulog"
)

//
// Ignore any panics.  Prefer IgnorePanicIn instead.
//
// Use: defer usync.IgnorePanic()
//
func IgnorePanic() {
	recover()
}

//
// Ignore any panics in activity().
//
func IgnorePanicIn(activity func()) {
	defer recover()
	activity()
}

//
// Capture any panics in activity().
//
func CapturePanicIn(activity func()) (captured interface{}) {
	defer func() { captured = recover() }()
	activity()
	return
}

//
// Log any panics.
//
// Use: defer usync.LogPanic()
//
func LogPanic(msg string) {
	if it := recover(); it != nil {
		if 0 != len(msg) {
			ulog.Printf("PANIC: %s: %s", msg, it)
		} else {
			ulog.Printf("PANIC: %s", it)
		}
	}
}

//
// Log any panics in activity().
//
func LogPanicIn(msg string, activity func()) {
	defer func() {
		if it := recover(); it != nil {
			if 0 != len(msg) {
				ulog.Printf("PANIC: %s: %s", msg, it)
			} else {
				ulog.Printf("PANIC: %s", it)
			}
		}
	}()
	activity()
}

//
// Log any panics and exit the program.
//
// Use: defer usync.FatalPanic()
//
func FatalPanic(msg string) {
	if it := recover(); it != nil {
		if 0 != len(msg) {
			ulog.Fatalf("PANIC: %s: %s", msg, it)
		} else {
			ulog.Fatalf("PANIC: %s", it)
		}
	}
}
