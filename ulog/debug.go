package ulog

import "log"

var ( // see uinit/debug.go
	DebugEnabled     = false                 // turn on all debug
	DebugEnabledFor  = make(map[string]bool) // turn on selective debug
	DebugDisabledFor = make(map[string]bool) // turn off selective debug
)

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
	if (DebugEnabled && !DebugDisabledFor[from]) || DebugEnabledFor[from] {
		if 0 == len(args) {
			log.Printf("DEBUG: %s: %s", from, format)
		} else {
			log.Printf("DEBUG: "+from+": "+format, args...)
		}
	}
}

// output a debug message if dbg && IsDebugEnabledFor("party")
func DebugfIfFor(dbg bool, from string, format string, args ...interface{}) {
	if dbg || (DebugEnabled && !DebugDisabledFor[from]) || DebugEnabledFor[from] {
		if 0 == len(args) {
			log.Printf("DEBUG: %s: %s", from, format)
		} else {
			log.Printf("DEBUG: "+from+": "+format, args...)
		}
	}
}

// output a debug message if dbg
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
	return (DebugEnabled && !DebugDisabledFor[from]) || DebugEnabledFor[from]
}
