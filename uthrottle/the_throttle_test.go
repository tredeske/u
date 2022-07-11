package uthrottle

import (
	"fmt"
	"runtime"
	"testing"
	"time"
)

func TestThrottles(t *testing.T) {
	const AMOUNT int64 = 1000000
	for name, throttle := range map[string]Throttler{
		"S": &SThrottle{},
		"M": &MThrottle{},
	} {
		t.Run(name, func(t *testing.T) {
			throttle.Start(AMOUNT, 0)
			defer throttle.Stop()

			throttle.Await(AMOUNT / 4) // very 1st time may not wait at all

			startT := time.Now()
			throttle.Await(AMOUNT / 4)
			since := time.Since(startT)

			atLeast := time.Second/4 - 50*time.Millisecond

			if since < atLeast {
				t.Fatalf("Took %s, but should have taken .25s", since)
			}
			fmt.Printf("took: %s\n", since)
		})
	}
}

//
// benchmark both throttles at various gigabit rates
//
func BenchmarkThrottles(b *testing.B) {
	for name, ctor := range map[string]func() Throttler{
		"S": func() Throttler { return &SThrottle{} },
		"M": func() Throttler { return &MThrottle{} },
	} {
		b.Run(name, func(b *testing.B) {
			for _, rate := range []int64{1, 5, 10, 15, 25, 50, 100, 400} {
				b.Run(fmt.Sprintf("%dG", rate), func(b *testing.B) {
					benchmarkThrottle(b, ctor(), rate*1000000000)
				})
			}
		})
	}
}

func benchmarkThrottle(b *testing.B, throttler Throttler, limit int64) {

	const bytesPer = 1500

	throttler.Start(limit/8, 100*time.Millisecond)
	defer throttler.Stop()
	runtime.Gosched()

	startT := time.Now()
	defer func() {
		elapsed := time.Since(startT)
		elapsedS := float64(elapsed) / float64(time.Second)
		rate := float64(b.N) * float64(bytesPer) * 8 / float64(elapsedS)
		percentOff := 100. * (rate - float64(limit)) / float64(limit)
		fmt.Printf("\nIterations: %d, time=%s, rate=%e, off by=%2.3f%%\n",
			b.N, elapsed, rate, percentOff)
	}()

	b.ResetTimer()
	switch t := throttler.(type) { // avoid interface cost for benchmark
	case *SThrottle:
		for i := 0; i < b.N; i++ {
			t.Await(bytesPer)
		}
	case *MThrottle:
		for i := 0; i < b.N; i++ {
			t.Await(bytesPer)
		}
	default:
		b.Fatalf("should not get here")
	}
}
