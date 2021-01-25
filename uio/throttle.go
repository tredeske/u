package uio

import "time"

type Throttle struct {
	last    time.Time // last time mark
	allowed float64   // current number of allowed things.  may be negative.
	Rate    float64   // limit per time quantum rate
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
	this.Rate = float64(rate)
	this.allowed = this.Rate
}

func (this *Throttle) IsSet() bool {
	return 0. < this.Rate
}

// return true if reached limit
//
func (this *Throttle) Limit(amount int) (clamped bool) {
	now := time.Now()
	// get elapsed time in seconds
	elapsed := float64(now.Sub(this.last)) * float64(.000000001)
	this.last = now
	this.allowed += elapsed * this.Rate

	if this.allowed > this.Rate { // clamp to avoid overflow
		this.allowed = this.Rate
	}
	this.allowed -= float64(amount)
	return 0.0 >= this.allowed
}

// return when throttle satisfied
//
func (this *Throttle) Wait(amount int) {
	if this.Limit(amount) && 0.0 > this.allowed {
		waitNanos := (-this.allowed) * float64(1000000000) / this.Rate
		dur := time.Duration(waitNanos)
		time.Sleep(dur)
	}
}
