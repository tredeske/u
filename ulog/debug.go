package ulog

import (
	"encoding/hex"
	"log"
	"sync"
)

var ( // see uinit/debug.go
	DebugEnabled      = false      // turn on all debug
	debugEnabledFor_  = sync.Map{} // turn on selective debug
	debugDisabledFor_ = sync.Map{} // turn off selective debug
)

//
// manage debug state for component
//
type Debug struct {
	Enabled   bool
	component string
	Prefix    string
}

func NewDebug(component string) *Debug {
	return (&Debug{}).Construct(component)
}

// construct in place
func (this *Debug) Construct(component string) *Debug {
	this.Enabled = IsDebugEnabledFor(component)
	this.component = component
	this.Prefix = "DEBUG: " + component + ": "
	return this
}

// output a debug message for the component if it is enabled for debug
func (this Debug) F(format string, args ...interface{}) {
	if this.Enabled {
		if 0 == len(args) {
			log.Printf(this.Prefix + format)
		} else {
			log.Printf(this.Prefix+format, args...)
		}
	}
}

//
// output a hex dump as part of a message, preventing its creation if not enabled.
//
//     var d Debug
//     var b []byte
//     d.F("hex dump:\n%s", d.Dump(b))
//
func (this Debug) Dump(b []byte) string {
	if this.Enabled {
		return hex.Dump(b)
	}
	return ""
}

//
// output a hex dump as part of a message, preventing its creation if not enabled.
//
//     enableDump := true
//     var d Debug
//     var b []byte
//     d.F("hex dump:\n%s", d.DumpIf(enableDump, b))
//
func (this Debug) DumpIf(on bool, b []byte) string {
	if on && this.Enabled {
		return hex.Dump(b)
	}
	return "[dump disabled]"
}

//
// these are meant to be set upon program initialization, and then read-only
// from that point on
//

func SetDebugEnabledFor(component string) {
	log.Println("ENABLING DEBUG FOR " + component)
	debugEnabledFor_.Store(component, true)
	//debugEnabledFor_[component] = true
}

func SetDebugDisabledFor(component string) {
	log.Println("DISABLING DEBUG FOR " + component)
	debugDisabledFor_.Store(component, true)
	//debugDisabledFor_[component] = true
}

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
func DebugfFor(component string, format string, args ...interface{}) {
	if IsDebugEnabledFor(component) {
		if 0 == len(args) {
			log.Printf("DEBUG: %s: %s", component, format)
		} else {
			log.Printf("DEBUG: "+component+": "+format, args...)
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

// is debug enabled for component?
func IsDebugEnabledFor(component string) bool {
	if DebugEnabled {
		_, ok := debugDisabledFor_.Load(component)
		return !ok
	} else {
		_, ok := debugEnabledFor_.Load(component)
		return ok
	}
}
