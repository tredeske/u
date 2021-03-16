//
// Package ulog provides a richer wrapper for the std log library.
//
// Various severity levels are supported
//  * TODO - to mark a todo item
//  * Debug - may be important to devs - may be disabled
//  * normal - informational
//  * Warn - a problem is about to occur, or a problem that is not an error
//  * Error - an important feature has failed and cannot make progress
//  * Fatal - log message and exit program
//
// Debug:
//
// Various debug methods are provided.
//
//     ulog.Debugf("some debug message: %s", aString)
//     ulog.DebugfFor("party", "some debug message: %s", aString)
//
// Guard methods help detect when debug is disabled so that expensive operations
// are not performed.
//
//     if ulog.DebugEnabled {
//         ulog.Debugf("some debug message: %s", expensive())
//     }
//     if ulog.IsDebugEnabledFor("party") {
//         ulog.DebugfFor("party", "some debug message: %s", expensive())
//     }
//
// The 'For' methods indicate you are emitting a debug message 'for' a
// party / component.  These can be selectively enabled / disabled.
//
//
// Config / Setup:
//
// The log will be set up according to the command line flags
// and/or the YAML config.
//  -debug     - turn on debug globally
//  -log FILE  - send output to FILE, or stdout if FILE is 'stdout'
//
// Log files will be automatically rotated and aged off.
//
// Debugging can be enabled selectively or globally thru the config file by
// adding a debug section.
//
//     properties:
//         ...
//     debug:
//         enable: ["list", "of", "components"]
//         disable: ["list", "of", "components"]
//     components:
//
// The special "all" string is reserved for enabling/disabling all debug.  Example:
//
//     debug:
//         enable: ["all"]
//
// The following will enable all debugging, except for certain components.
//
//     debug:
//         enable: ["all"]
//         disable: ["compA", "compB"]
//
// The list of strings works with methods such as
//
//     DebugfFor("party", "format", ...)
//
// So, it is up to your code to identify itself.
//
package ulog

import (
	"fmt"
	"log"

	"github.com/tredeske/u/uexit"
)

func TODO(format string, args ...interface{}) {
	if 0 == len(args) {
		log.Printf("TODO: " + format)
	} else {
		log.Printf("TODO: "+format, args...)
	}
}

func Println(args ...interface{}) {
	log.Println(args...)
}

func Printf(format string, args ...interface{}) {
	log.Printf(format, args...)
}

func Warnf(format string, args ...interface{}) {
	if 0 == len(args) {
		log.Printf("WARN: " + format)
	} else {
		log.Printf("WARN: "+format, args...)
	}
}

func Errorf(format string, args ...interface{}) {
	if 0 == len(args) {
		log.Printf("ERROR: " + format)
	} else {
		log.Printf("ERROR: "+format, args...)
	}
}

// log message and exit program with status 1
//
func Fatalf(format string, args ...interface{}) {
	if 0 == len(args) {
		log.Printf("FATAL: " + format)
	} else {
		log.Printf("FATAL: "+format, args...)
	}
	uexit.Exit(1)
}

// cause a panic of the program with the provided message
//
func Panicf(format string, args ...interface{}) {
	if 0 == len(args) {
		panic(format)
	} else {
		panic(fmt.Sprintf(format, args...))
	}
}

// If cond is false, then panic with the provided message
//
func Assertf(cond bool, format string, args ...interface{}) {
	if !cond {
		if 0 == len(args) {
			panic(format)
		} else {
			panic(fmt.Sprintf(format, args...))
		}
	}
}
