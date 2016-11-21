package uexit

import (
	"log"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"
)

//
// Time to wait for stuff to die, if anything registered
//
var WaitTime = 5 * time.Second

// invoke from *end* of main thread to turn main thread into signal handler, or
// invoke as goroutine
//
func SimpleSignalHandling() {
	ExitOnStandardSignals()
	DumpOnSigQuit()
	WaitForExit()
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
	sigC_         chan os.Signal     = make(chan os.Signal)
	exitC_        chan int           = make(chan int)
	exitHandlerC_ chan *exitHandler_ = make(chan *exitHandler_) // sync!
	exitDoneC_    chan bool          = make(chan bool, 32)
	waitC_        chan bool          = start()
)

func start() (rv chan bool) {
	rv = make(chan bool, 2)
	go manageExit(rv)
	return
}

func manageExit(waitC chan bool) {

	done := 0
	exitStatus := 0
	channels := make([]chan<- int, 0, 8)

	//
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

	//
	// wait for a signal indicating time to die,
	// or for a goroutine in this process to call Exit(),
	// or for someone to register to be notified when it is time to die
	//
	for notify := false; !notify; {
		//DebugfFor("exit", "waiting for event")
		select {
		case exitStatus = <-exitC_:
			log.Printf("\n\nReceived exit code %d\n\n", exitStatus)
			notify = true
		case sig := <-sigC_:
			log.Printf("\n\nReceived exit signal '%s'\n\n", sig)
			notify = true
		case h := <-exitHandlerC_:
			channels = append(channels, h.ch)
		case <-exitDoneC_:
			done++
		}
	}

	//
	// it is now time to die
	//
	// we have work to to if there are registered handlers
	//
	if 0 != len(channels) {
		//
		// we're dying, so notify anyone interested so that can take care
		// of any last minute business.
		//
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
		after := time.After(WaitTime)
		timeToDie := false
		for i := done; !timeToDie && i < len(channels); i++ {
			select {
			case <-after:
				timeToDie = true
			case <-exitDoneC_:
				done++
			}
		}
	}

	// time to go
	//
	waitC <- true
	log.Printf("\n\nExitting with status %d\n\n", exitStatus)
	os.Exit(exitStatus)
}

// register to receive exit notifications
//
// The exit handler will wait for a brief time for a response on the reply channel
// from each AtExit registration, then exit.
//
func AtExit() (exitNotifyC <-chan int, exitReplyC chan<- bool) {
	notifyC := make(chan int, 2) // non blocking notifications
	exitNotifyC = notifyC

	exitHandlerC_ <- &exitHandler_{ // may block here til WaitForExit
		ch: notifyC,
	}
	return exitNotifyC, exitDoneC_
}

// register signals that should cause process exit
func ExitOnSignals(sigs ...os.Signal) {
	signal.Notify(sigC_, sigs...)
}

//
// register the usual signals that should cause process exit
//
func ExitOnStandardSignals() {
	signal.Notify(sigC_, os.Interrupt, os.Kill, syscall.SIGHUP, syscall.SIGTERM)
}

//
// invoke from main thread to park it until process death
//
func WaitForExit() {

	<-waitC_
}

//
// cause the process to exit by the deadline
//
func ExitWait(code int, wait time.Duration) {
	exitC_ <- code
	time.Sleep(wait)
	log.Printf("Exit handler failed to exit on time - exiting now")
	os.Exit(code)
}

//
// cause the process to exit within WaitTime seconds
//
func Exit(code int) {
	ExitWait(code, WaitTime)
}
