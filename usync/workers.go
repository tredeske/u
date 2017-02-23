package usync

import (
	"sync"
	"time"
)

//
// A pool of workers
//
// Example:
//    pool := usync.Workers{}
//
//    // start a bunch of workers
//    pool.Go( 2,
//        func(req interface{}) {
//            ...
//        })
//
//    go func() { // feeder feeds requests to pool
//        for ... {
//            pool.RequestC <- ...
//        }
//        pool.Close() // feeder closes pool
//    }()
//
//
type Workers struct {
	OnDone   func()         // if set, call this when workers done
	RequestC ItChan         // where to post requests to be worked on
	drain    AtomicBool     //
	wg       sync.WaitGroup //
}

//
// start N workers to perform processing
//
func (this *Workers) Go(workers int, work func(interface{})) {

	if nil == this.RequestC {
		this.RequestC = make(chan interface{}, workers*2)
	}

	this.wg.Add(workers)

	//
	// the workers
	//
	for i := 0; i < workers; i++ {
		go func() {
			defer this.wg.Done()
			for req := range this.RequestC {

				if !this.drain.IsSet() { // don't do any work if draining
					work(req)
				}
			}
		}()
	}

	if nil != this.OnDone {
		go func() {
			this.wg.Wait()
			this.OnDone()
		}()
	}
}

func (this Workers) PutRecover(req interface{}) (ok bool) {
	return this.RequestC.PutRecover(req)
}

func (this Workers) Put(req interface{}) {
	this.RequestC <- req
}

func (this Workers) PutTry(req interface{}) (ok bool) {
	return this.RequestC.PutTry(req)
}

func (this Workers) PutWait(req interface{}, d time.Duration) (ok bool) {
	return this.RequestC.PutWait(req, d)
}

//
// tell workers to pull items from RequestC, but not do any work.
//
// this does not close the request chan, so any workers blocked on that chan
// will remain blocked until Close() is called.
//
func (this *Workers) Drain() {
	this.drain.Set()
}

//
// stop the drain
//
func (this *Workers) Plug() {
	this.drain.Clear()
}

//
// Return true if drained.  This can only be true if Drain is set.
//
func (this Workers) IsDrained() bool {
	return this.drain.IsSet() && 0 == len(this.RequestC)
}

//
// Return true when drained.  This can only occur if Drain is set.
//
func (this Workers) WaitDrained(deadline time.Duration) bool {
	return AwaitTrue(deadline, 0, this.IsDrained)
}

//
// Return true if requestC currently empty
//
func (this Workers) IsEmpty() bool {
	return 0 == len(this.RequestC)
}

//
// did someone set Drain?
//
func (this Workers) IsDraining() bool {
	return this.drain.IsSet()
}

//
// Wait til all workers done
//
func (this *Workers) WaitDone() {
	this.wg.Wait()
}

//
// Tell the pool there's no more work coming
//
// There may be still results being worked on after this
//
func (this *Workers) Close() {
	defer IgnorePanic()
	close(this.RequestC)
}

//
// Cease all work as soon as possible and close this down, throwing away
// any queued requests
//
func (this *Workers) Shutdown() {
	this.Drain()
	this.Close()
}

//
//
//
//
//

//
//
//
type WorkGang struct {
	Pool Workers

	//
	// produce next req for workers, or nil if done.
	//
	// returning nil causes pool to Close() and this will no longer be called
	//
	// if !ok, pool will be Close()ed and Drain()ed, and this will no
	// longer be called.  In other words, all work will be aborted ASAP.
	//
	// this will be run from a separate goroutine
	//
	OnFeed func() (req interface{}, ok bool)

	//
	// Function called by workers
	// resp is passed to OnResponse.  if !ok, then entire operation is aborted.
	//
	OnRequest func(req interface{}) (resp interface{}, ok bool)

	//
	// Function called by workers
	// if !ok, Shutdown() the pool
	//
	// this runs in the callers goroutine
	//
	OnResponse func(resp interface{}) (ok bool)
}

//
// perform the work, returning when done
//
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

	responseC := NewItChan(workers)

	this.Pool.RequestC = nil
	this.Pool.OnDone = responseC.Close
	this.Pool.drain.Clear()

	this.Pool.Go(workers,
		func(req interface{}) {
			resp, ok := this.OnRequest(req)
			if !ok {
				this.Pool.Drain()
			} else {
				responseC <- resp
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
