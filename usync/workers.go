package usync

import (
	"log"
	"sync"
	"time"

	"github.com/tredeske/u/uerr"
)

type WorkF func(any)

// A pool of workers
//
// Example:
//
//	pool := usync.Workers{}
//
//	// start a bunch of workers
//	pool.Go( 2,
//	    func(req any) {
//	        ...
//	    })
//
//	go func() { // feeder feeds requests to pool
//	    for ... {
//	        pool.RequestC <- ...
//	    }
//	    pool.Close() // feeder closes pool
//	}()
type Workers struct {
	OnDone   func()         // if set, call this when workers done
	RequestC Chan[any]      // where to post requests to be worked on
	drain    AtomicBool     //
	wg       sync.WaitGroup //

	//
	// if this is set, call whenever a goroutine panics
	// if this returns true, then cause the worker to end
	//
	// if this is not set, panic will be captured and logged, and worker will end
	//
	OnPanic func(cause any) (die bool)
}

// start N workers to perform processing
func (this *Workers) Go(workers int, factory func() (work WorkF)) {

	if nil == this.RequestC {
		this.RequestC = NewChan[any](workers * 2)
	}

	this.wg.Add(workers)

	//
	// the workers
	//
	for i := 0; i < workers; i++ {
		go func(work func(any)) {
			defer func() {
				this.wg.Done()
				if it := recover(); nil != it {
					log.Printf("WARN: sync.Worker panic: %s", it)
				}
			}()
			for req := range this.RequestC {

				if !this.drain.IsSet() { // don't do any work if draining
					if nil != this.OnPanic {
						die := false
						func() {
							defer func() {
								if it := recover(); it != nil {
									die = this.OnPanic(it)
								}
							}()
							work(req)
						}()
						if die {
							break
						}
					} else {
						work(req)
					}
				}
			}
		}(factory())
	}

	if nil != this.OnDone {
		go func() {
			this.wg.Wait()
			this.OnDone()
		}()
	}
}

func (this *Workers) PutWaitRecover(req any, wait time.Duration) (ok bool) {
	return this.RequestC.PutWaitRecover(req, wait)
}

func (this *Workers) Put(req any) {
	this.RequestC <- req
}

func (this *Workers) PutWait(req any, d time.Duration) (ok bool) {
	return this.RequestC.PutWait(req, d)
}

// tell workers to pull items from RequestC, but not do any work.
//
// this does not close the request chan, so any workers blocked on that chan
// will remain blocked until Close() is called.
func (this *Workers) Drain() {
	this.drain.Set()
}

// stop the drain
func (this *Workers) Plug() {
	this.drain.Clear()
}

// Return true if drained.  This can only be true if Drain is set.
func (this *Workers) IsDrained() bool {
	return this.drain.IsSet() && 0 == len(this.RequestC)
}

// Return true when drained.  This can only occur if Drain is set.
func (this *Workers) WaitDrained(deadline time.Duration) bool {
	return AwaitTrue(deadline, 0, this.IsDrained)
}

// Return true if requestC currently empty
func (this *Workers) IsEmpty() bool {
	return 0 == len(this.RequestC)
}

// did someone set Drain?
func (this *Workers) IsDraining() bool {
	return this.drain.IsSet()
}

// Wait til all workers done
func (this *Workers) WaitDone() {
	this.wg.Wait()
}

// Tell the pool there's no more work coming
//
// There may be still results being worked on after this
func (this *Workers) Close() {
	defer uerr.IgnoreClosedChanPanic()
	close(this.RequestC)
}

// Cease all work as soon as possible and close this down, throwing away
// any queued requests
func (this *Workers) Shutdown() {
	this.Drain()
	this.Close()
}

//
//
// -----------------------------------------------------------------------
//
//

// a feeder -> workers -> collector flow
type WorkGang struct {
	Pool Workers

	//
	// produce next req for workers, or nil if done.
	//
	// returning nil causes pool to Close() and this will no longer be called
	//
	// if !ok, entire operation is aborted.
	//
	// this will be run from a separate goroutine
	//
	OnFeed func() (req any, ok bool)

	//
	// Function called by worker goroutines to take requests and produce responses
	//
	// resp is passed to OnResponse.  if !ok, then entire operation is aborted.
	//
	OnRequest func(req any) (resp any, ok bool)

	//
	// Function called by workers
	//
	// if !ok, entire operation is aborted
	//
	// this runs in the callers goroutine
	//
	OnResponse func(resp any) (ok bool)
}

// perform the work, returning when done
func (this *WorkGang) Work(workers int) {

	if nil == this.OnFeed {
		panic("OnFeed() not set")
	} else if nil == this.OnRequest {
		panic("OnRequest() not set")
	} else if nil == this.OnResponse {
		panic("OnResponse() not set")
	} else if 0 >= workers {
		panic("workers must be positive")
	}

	defer this.Pool.Shutdown()

	responseC := NewChan[any](workers)

	this.Pool.RequestC = nil
	this.Pool.OnDone = func() { close(responseC) }
	this.Pool.drain.Clear()

	this.Pool.Go(workers,

		func() WorkF {
			return func(req any) {
				resp, ok := this.OnRequest(req)
				if !ok {
					this.Pool.Drain()
				} else {
					responseC <- resp
				}
			}
		})

	//
	// create requests and feed them to workers
	//
	go func() {
		defer this.Pool.Close()
		for !this.Pool.IsDraining() {
			req, ok := this.OnFeed()
			if !ok {
				this.Pool.Drain()
				break
			} else if nil == req {
				break
			}
			this.Pool.RequestC <- req
		}
	}()

	//
	// collect worker responses
	//
	for resp := range responseC {
		if !this.OnResponse(resp) {
			break
		}
	}
}
