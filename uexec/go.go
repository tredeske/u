package uexec

import (
	"log"

	"github.com/tredeske/u/ulog"
)

// print out error in case of failure of goroutine
func SelfGo(name string, fn func() error) {
	log.Printf("%s: starting", name)
	err := fn()
	log.Printf("%s: done", name)
	if err != nil {
		ulog.Fatalf("%s: Goroutine failed: %s", name, err.Error())
	}
}

// print out error in case of failure of goroutine
func MakeGo(name string, fn func() error) {
	go SelfGo(name, fn)
}
