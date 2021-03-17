package ulog

import "log"

var ( // see uinit/debug.go
	DebugEnabled      = false                 // turn on all debug
	debugEnabledFor_  = make(map[string]bool) // turn on selective debug
	debugDisabledFor_ = make(map[string]bool) // turn off selective debug
)

//
// these are meant to be set upon program initialization, and then read-only
// from that point on
//

func SetDebugEnabledFor(party string) {
	debugEnabledFor_[party] = true
}

func SetDebugDisabledFor(party string) {
	debugDisabledFor_[party] = true
}

/*
const (
	enableSz_  = 8
	disableSz_ = 8
)

var en_ = enabling_{
	enabledM:  make(map[uintptr]struct{}),
	disabledM: make(map[uintptr]struct{}),
}

type enabling_ struct {
	enabled   [enableSz_]uintptr
	disabled  [disableSz_]uintptr
	enabledM  map[uintptr]struct{}
	disabledM map[uintptr]struct{}
}

func (this enabling_) mapDisabled(h uintptr) bool {
	_, ok := this.disabledM[h]
	return ok
}

func (this enabling_) mapEnabled(h uintptr) bool {
	_, ok := this.enabledM[h]
	return ok
}

func isEnabledFor(h uintptr) bool {
	if DebugEnabled {
		return 0 == en_.disabled[0] ||
			!(en_.disabled[0] == h || en_.disabled[1] == h ||
				en_.disabled[2] == h || en_.disabled[3] == h ||
				en_.disabled[4] == h || en_.disabled[5] == h ||
				en_.disabled[6] == h || en_.disabled[7] == h) ||
			(0 != en_.disabled[7] && en_.mapDisabled(h))
	} else {
		return 0 != en_.enabled[0] &&
			(en_.enabled[0] == h || en_.enabled[1] == h ||
				en_.enabled[2] == h || en_.enabled[3] == h ||
				en_.enabled[4] == h || en_.enabled[5] == h ||
				en_.enabled[6] == h || en_.enabled[7] == h ||
				(0 != en_.enabled[7] && en_.mapEnabled(h)))
	}
}
*/

// output a debug message if DebugEnabled
func Debugf(format string, args ...interface{}) {
	if DebugEnabled {
		if 0 == len(args) {
			log.Printf("DEBUG: " + format)
		} else {
			log.Printf("DEBUG: "+format, args...)
		}
	}
}

// output a debug message if DebugEnabled
func Debugln(args ...interface{}) {
	if DebugEnabled {
		arr := [8]interface{}{}
		slice := append(arr[:0], "DEBUG:")
		slice = append(slice, args...)
		log.Println(slice...)
	}
}

// output a debug message if IsDebugEnabledFor("party")
func DebugfFor(from string, format string, args ...interface{}) {
	if IsDebugEnabledFor(from) {
		if 0 == len(args) {
			log.Printf("DEBUG: %s: %s", from, format)
		} else {
			log.Printf("DEBUG: "+from+": "+format, args...)
		}
	}
}

//
// output a debug message if dbg
//
// hint: set dbg to IsDebugEnabledFor() to compute that once, then reuse
//
func DebugfIf(dbg bool, format string, args ...interface{}) {
	if dbg || DebugEnabled {
		if 0 == len(args) {
			log.Printf("DEBUG: %s", format)
		} else {
			log.Printf("DEBUG: "+format, args...)
		}
	}
}

// is debug enabled globally?
func IsDebugEnabled() bool {
	return DebugEnabled
}

// is debug enabled for 'from'?
func IsDebugEnabledFor(from string) bool {
	return (DebugEnabled && !debugDisabledFor_[from]) || debugEnabledFor_[from]
}
