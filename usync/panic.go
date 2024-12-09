package usync

import (
	"github.com/tredeske/u/ulog"
)

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
