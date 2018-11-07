package ulog

import "log"

var ( // see uinit/debug.go
	DebugEnabled     = false                 // turn on all debug
	DebugEnabledFor  = make(map[string]bool) // turn on selective debug
	DebugDisabledFor = make(map[string]bool) // turn off selective debug
)

func Debugf(format string, args ...interface{}) {
	if DebugEnabled {
		if 0 == len(args) {
			log.Printf("DEBUG: " + format)
		} else {
			log.Printf("DEBUG: "+format, args...)
		}
	}
}

func Debugln(args ...interface{}) {
	if DebugEnabled {
		arr := [8]interface{}{}
		slice := append(arr[:0], "DEBUG:")
		slice = append(slice, args...)
		log.Println(slice...)
	}
}

func DebugfFor(from string, format string, args ...interface{}) {
	if (DebugEnabled && !DebugDisabledFor[from]) || DebugEnabledFor[from] {
		if 0 == len(args) {
			log.Printf("DEBUG: %s: %s", from, format)
		} else {
			log.Printf("DEBUG: "+from+": "+format, args...)
		}
	}
}

func DebugfIfFor(dbg bool, from string, format string, args ...interface{}) {
	if dbg || (DebugEnabled && !DebugDisabledFor[from]) || DebugEnabledFor[from] {
		if 0 == len(args) {
			log.Printf("DEBUG: %s: %s", from, format)
		} else {
			log.Printf("DEBUG: "+from+": "+format, args...)
		}
	}
}

func DebugfIf(dbg bool, format string, args ...interface{}) {
	if dbg || DebugEnabled {
		if 0 == len(args) {
			log.Printf("DEBUG: %s", format)
		} else {
			log.Printf("DEBUG: "+format, args...)
		}
	}
}

func IsDebugEnabled() bool {
	return DebugEnabled
}

func IsDebugEnabledFor(from string) bool {
	return (DebugEnabled && !DebugDisabledFor[from]) || DebugEnabledFor[from]
}
