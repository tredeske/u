package usync

/*

import (
	"sync"

	"github.com/tredeske/u/uconfig"
)

//
// A pool of workers
//
// Example:
//    pool := u.WorkPool{}
//
//    // start a bunch of workers
//    pool.Go( 2,
//        func(req interface{}) (resp interface{}) {
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
//    for resp:= range pool.ResponceC { // collect results
//        ...
//    }
//    pool.Drain()
//
type WorkPool struct {
	stopNow   AtomicBool
	RequestC  ItChan
	ResponseC ItChan
}

//
// start N workers to perform processing
//
func (this *WorkPool) Go(workers int, work func(interface{}) interface{}) {

	if nil == this.RequestC {
		this.RequestC = make(chan interface{}, workers*2)
	}
	if nil == this.ResponseC {
		this.ResponseC = make(chan interface{}, workers*2)
	}

	var wg sync.WaitGroup
	wg.Add(workers)

	//
	// the workers
	//
	for i := 0; i < workers; i++ {
		go func() {
			for req := range this.RequestC {

				if this.stopNow.IsSet() { // don't do any work if stopped
					break
				}

				resp := work(req)
				this.ResponseC <- resp

				if this.stopNow.IsSet() { // don't check req chan if stopped
					break
				}
			}
			wg.Done()
		}()
	}

	//
	// when all workers done, close responseC
	//
	go func() {
		defer IgnorePanic()
		wg.Wait()
		close(this.ResponseC)
	}()
}

//
// tell workers to stop immediately.
//
// stop means to not begin any new work and to not check the request chan
// for more work.
//
// this does not close the request chan, so any workers blocked on that chan
// will remain blocked until Close() is called.
//
func (this *WorkPool) StopNow() {
	this.stopNow.Set()
}

//
// did someone throw the big red switch?
//
func (this *WorkPool) IsStopped() bool {
	return this.stopNow.IsSet()
}

//
// Tell the pool there's no more work coming
//
// There may be still results being worked on after this
//
func (this *WorkPool) Close() {
	defer IgnorePanic()
	close(this.RequestC)
}

//
// throw away any remaining responses
//
func (this *WorkPool) Drain() {
	for _ = range this.ResponseC {
	}
}

//
// Get the next result nicely.
//
// rv needs to be a pointer to the type you want
//
// var myStruct *MyStruct // what workers produce
// var pool WorkPool
// conversionError := pool.Next( &myStruct ) // ptr to what workers produce
//
func (this *WorkPool) Next(rv interface{}) (err error) {
	resp := <-this.ResponseC
	if nil != resp {
		err = uconfig.Assign("worker", rv, resp)
	}
	return
}
*/
