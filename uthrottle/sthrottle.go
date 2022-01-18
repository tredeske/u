package uthrottle

import (
	"sync/atomic"
	"time"
)

//
// A throttle that allows a certain amount of work to be performed
// per time interval for a single goroutine.
//
// works great, but only needed if we can show it is less overhead than Throttle
//
// running the benchmarks doesn't appear to show that for uncontended (single
// goroutine) case.  at 400Gb/s, they both take 26% +- 1% of cpu.
//
type SThrottle struct {
	used     int64         // units used by worker
	avail    int64         // units available to worker
	dole     int64         // units per second, or 0 if inactive
	lastT    int64         //
	interval time.Duration // time interval
	rate     int64         // units per second, or 0 if inactive
}

//
// report units used, but do NOT wait
//
func (this *SThrottle) Account(amount int64) {
	this.used += amount
}

//
// report units used, and, if necessary, wait for next time to do something
//
func (this *SThrottle) Await(amount int64) {
	this.used += amount
	if this.avail < this.used {
		this.adjust()
	}
}

func (this *SThrottle) adjust() {
	this.used -= this.avail
	this.avail = 0
	dole := atomic.LoadInt64(&this.dole)
	nIntervals := 1 + this.used/dole
	nextT := this.lastT + (nIntervals * int64(this.interval))
	now := time.Now().UnixNano()
	if now < nextT {
		time.Sleep(time.Duration(nextT - now))
	}
	this.used = 0
	this.avail = dole
	this.lastT = now
}

func (this *SThrottle) Start(rate int64, interval time.Duration) {
	if rate < 0 {
		rate = 0
	}
	if interval < INTERVAL_MIN {
		interval = INTERVAL_MIN
	}
	if 0 != this.rate {
		panic("throttle already started!")
	}
	this.interval = interval
	this.SetRate(rate)
}

func (this *SThrottle) Stop() {
	//atomic.StoreInt64(&this.dole, 0)
}

func (this *SThrottle) SetRate(rate int64) {
	if 0 >= rate {
		rate = 1
	}
	timesPerSec := int64(time.Second / this.interval)
	dole := rate / timesPerSec
	if 0 >= dole {
		dole = 1
	}
	atomic.StoreInt64(&this.dole, dole)
	atomic.StoreInt64(&this.rate, rate)
}
