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
