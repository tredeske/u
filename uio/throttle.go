package uio

import "time"

type Throttle struct {
	last    time.Time // last time mark
	allowed float64   // current number of allowed things.  may be negative.
	rate    float64   // limit per time quantum rate
}

// create throttle set to rate things per quantum
func NewThrottle(rate int64) (this *Throttle) {
	this = &Throttle{}
	this.Construct(rate)
	return
}

// in place construct throttle to allow rate things per quantum
func (this *Throttle) Construct(rate int64) {
	this.last = time.Now()
	this.rate = float64(rate)
	this.allowed = this.rate
}

// return true if reached limit
//
func (this *Throttle) Limit(amount int) (clamped bool) {
	now := time.Now()
	// get elapsed time in seconds
	elapsed := float64(now.Sub(this.last)) * float64(.000000001)
	this.last = now
	this.allowed += elapsed * this.rate

	if this.allowed > this.rate { // clamp to avoid overflow
		this.allowed = this.rate
	}
	this.allowed -= float64(amount)
	return 0.0 >= this.allowed
}

// return when throttle satisfied
//
func (this *Throttle) Wait(amount int) {
	if this.Limit(amount) && 0.0 > this.allowed {
		waitNanos := (-this.allowed) * float64(1000000000) / this.rate
		dur := time.Duration(waitNanos)
		time.Sleep(dur)
	}
}
