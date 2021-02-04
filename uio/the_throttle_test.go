//+build linux

package uio

import (
	"testing"
	"time"
)

func TestThrottle(t *testing.T) {
	throttle := NewThrottle(1000)

	if throttle.Limit(500) {
		t.Fatalf("Putting in 1st (500) should not clamp: %#v", throttle)
	}

	if throttle.Limit(500) {
		t.Fatalf("Putting in 2nd (500) should not clamp: %#v", throttle)
	}

	if !throttle.Limit(200) {
		t.Fatalf("Putting in 3rd (200) should clamp: %#v", throttle)
	}

	start := time.Now()
	throttle.Wait(0)
	elapsed := time.Since(start).Seconds()
	if .15 > elapsed || .25 < elapsed {
		t.Fatalf("Elapsed time out of range: %f", elapsed)
	}

	if !throttle.Limit(200) {
		t.Fatalf("Putting in 4th (200) should clamp: %#v", throttle)
	}
	start = time.Now()
	throttle.Wait(0)
	elapsed = time.Since(start).Seconds()
	if .15 > elapsed || .25 < elapsed {
		t.Fatalf("2nd Elapsed time out of range: %f", elapsed)
	}
}
