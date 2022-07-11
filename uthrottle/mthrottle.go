//
// throttles for limiting work done per interval
//
package uthrottle

import (
	"sync"
	"sync/atomic"
	"time"
)

const INTERVAL_MIN time.Duration = 10 * time.Millisecond

type Throttler interface {
	Await(amount int64)
	Account(amount int64)
	SetRate(rate int64)
	Start(rate int64, interval time.Duration)
	Stop() // not to be used to stop user of Throttler
}

//
// A throttle that allows a certain amount of work to be performed
// per time interval by multiple goroutines.
//
// A record is kept of previous time quanta, which can have a decaying effect
// on the throttle.  This helps account for if a previous interval was unused,
// the worker can "catch up" to some extent.
//
// The implementation is semi-lock-free.  A worker only acquires the lock
// when the worker must sleep before continuing.
//
type MThrottle struct {
	used     int64         // units used by worker
	avail    int64         // units available to worker
	lock     sync.Mutex    // for waiting
	cond     *sync.Cond    // for waiting
	rate     int64         // units per second, or 0 if inactive
	interval time.Duration // time interval
}

//
// tell throttle amount, but do NOT wait
//
func (this *MThrottle) Account(amount int64) {
	atomic.AddInt64(&this.used, amount)
}

//
// tell throttle amount, and wait for next time to do something
//
func (this *MThrottle) Await(amount int64) {
	if atomic.LoadInt64(&this.avail) < atomic.AddInt64(&this.used, amount) {
		this.await() // allow this outer call to inline
	}
}

func (this *MThrottle) await() {
	this.lock.Lock()
	for atomic.LoadInt64(&this.avail) < atomic.LoadInt64(&this.used) {
		this.cond.Wait()
	}
	this.lock.Unlock()
}

func (this *MThrottle) Start(rate int64, interval time.Duration) {
	if rate < 0 {
		rate = 0
	}
	if interval < INTERVAL_MIN {
		interval = INTERVAL_MIN
	}
	this.lock.Lock()
	defer this.lock.Unlock()
	if 0 != this.rate {
		panic("throttle already started!")
	}
	this.rate = rate
	this.interval = interval
	if nil == this.cond {
		this.cond = sync.NewCond(&this.lock)
	}

	if 0 < this.rate {
		go this.run()
	}
}

func (this *MThrottle) Stop() {
	this.lock.Lock()
	this.rate = 0
	this.lock.Unlock()
}

func (this *MThrottle) SetRate(rate int64) {
	if 0 >= rate {
		rate = 1
	}
	this.lock.Lock()
	needStart := 0 == this.rate
	this.rate = rate
	this.lock.Unlock()
	if needStart {
		go this.run()
	}
}

func (this *MThrottle) run() {
	const Q_LEN = 4
	var quanta [Q_LEN]int64
	timesPerSec := int64(time.Second / this.interval)
	rate := this.rate
	dole := rate / timesPerSec

	t := time.NewTicker(this.interval)
	defer t.Stop()

	for {
		<-t.C // wait next interval

		this.lock.Lock()
		if rate != this.rate {
			if 0 == this.rate {
				this.lock.Unlock()
				break
			}
			rate = this.rate
			dole = rate / timesPerSec
		}

		//
		// the sum of the quanta is what is available to the worker
		//
		// when work is done, used is increased by the worker
		//
		// we subtract used from the quanta, and shift the quanta right,
		// exponentially decaying (divide by 4) it as we do that
		//
		// we end up with a sum for the next avail, and also with the
		// deduction from used
		//
		used := atomic.LoadInt64(&this.used)
		remaining := used
		prev := dole    // start with the dole for 1st quantum
		var avail int64 // computes the new avail amount
		for i := 0; i < Q_LEN-1; i++ {
			curr := quanta[i]
			if remaining > curr {
				remaining -= curr
				curr = 0
			} else {
				curr -= remaining
				remaining = 0
			}
			quanta[i] = prev
			avail += prev
			prev = curr >> 2 // exponential decay
		}

		// order matters
		atomic.StoreInt64(&this.avail, avail)
		atomic.AddInt64(&this.used, remaining-used)

		this.lock.Unlock()
		this.cond.Broadcast()
	}
}
