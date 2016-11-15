package uexit

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

// invoke from *end* of main thread to turn main thread into signal handler, or
// invoke as goroutine
//
func SimpleSignalHandling() {
	ExitOnStandardSignals()
	DumpOnSigQuit()
	WaitForExit(5 * time.Second)
}

func DumpOnSigQuit() {
	go func() {
		sigsC := make(chan os.Signal, 1)
		signal.Notify(sigsC, syscall.SIGQUIT)
		buf := make([]byte, 1<<20)
		for {
			<-sigsC
			runtime.Stack(buf, true)
			log.Println("=== received SIGQUIT ===\n" +
				"*** goroutine dump...\n" +
				string(buf) +
				"\n*** end")
		}
	}()
}

type exitHandler_ struct {
	ch chan<- int
}

var (
	sigCh_         chan os.Signal     = make(chan os.Signal)
	exitCh_        chan int           = make(chan int)
	exitHandlerCh_ chan *exitHandler_ = make(chan *exitHandler_) // sync!
	exitDoneCh_    chan bool          = make(chan bool, 32)
)

// register to receive exit notifications
//
// The exit handler will wait for a brief time for a response on the reply channel
// from each AtExit registration, then exit.
//
func AtExit() (exitNotifyC <-chan int, exitReplyC chan<- bool) {
	notifyC := make(chan int, 2) // non blocking notifications
	exitNotifyC = notifyC

	exitHandlerCh_ <- &exitHandler_{ // may block here til WaitForExit
		ch: notifyC,
	}
	return exitNotifyC, exitDoneCh_
}

// register signals that should cause process exit
func ExitOnSignals(sigs ...os.Signal) {
	signal.Notify(sigCh_, sigs...)
}

// register the usual signals that should cause process exit
func ExitOnStandardSignals() {
	signal.Notify(sigCh_, os.Interrupt, os.Kill, syscall.SIGHUP, syscall.SIGTERM)
}

// invoke from main thread to turn main thread into signal handler, or
// invoke as goroutine
//
func WaitForExit(wait time.Duration) {

	done := 0
	exitStatus := 0
	channels := make([]chan<- int, 0, 8)

	// in case of panic when talking on a chan, exit immediately
	//
	defer func() {
		r := recover()
		if r != nil {
			log.Printf("ERROR: Problem in exit handling: %s", r)
			log.Printf("\n\nExitting with status %d\n\n", exitStatus)
			os.Exit(exitStatus)
		}
	}()

	// wait for a signal indicating time to die,
	// or for a goroutine in this process to call Exit(),
	// or for someone to register to be notified when it is time to die
	//
	for {
		//DebugfFor("exit", "waiting for event")
		select {
		case exitStatus = <-exitCh_:
			goto notify
		case sig := <-sigCh_:
			log.Printf("\n\nExitting due to signal '%s'\n\n", sig)
			goto notify
		case h := <-exitHandlerCh_:
			channels = append(channels, h.ch)
		case <-exitDoneCh_:
			done++
		}
	}

	// we're dying, so notify anyone interested so that can take care
	// of any last minute business.
	//
notify:
	//DebugfFor("exit", "notifying channels")
	for _, ch := range channels {
		select { // nonblock
		case ch <- exitStatus:
		default:
		}
	}

	// wait for any interested parties to chime in
	//
	//DebugfFor("exit", "waiting for done")
	after := time.After(wait)
	for i := done; i < len(channels); i++ {
		select {
		case <-after:
			goto timeToDie
		case <-exitDoneCh_:
			done++
		}
	}

	// time to go
	//
timeToDie:
	log.Printf("\n\nExitting with status %d\n\n", exitStatus)
	os.Exit(exitStatus)
}

//
// cause the process to exit by the deadline
//
func ExitWait(code int, wait time.Duration) {
	exitCh_ <- code
	time.Sleep(wait)
	log.Printf("Exit handler failed to exit on time - exiting now")
	os.Exit(code)
}

//
// cause the process to exit within 5 seconds
//
func Exit(code int) {
	ExitWait(code, 5*time.Second)
}
