package usync

import (
	"errors"
	"time"
)

var ErrTimeout = errors.New("Operation timed out")

//
// invoke done until it returns true or error, or the timeout occurs.
// return error if done errors or timeout occurs
//
func Await(timeout, interval time.Duration, done func() (bool, error)) error {

	var deadline time.Time
	deadline, interval = setDeadline(timeout, interval)
	for {
		complete, err := done()
		if err != nil || complete {
			return err
		} else if !deadline.After(time.Now()) {
			break
		}
		time.Sleep(interval)
	}
	return ErrTimeout
}

//
// invoke done() until it returns true, or the timeout occurs.
// return true iff done() returns true before timeout
//
func AwaitTrue(timeout, interval time.Duration, done func() bool) (rv bool) {

	var deadline time.Time
	deadline, interval = setDeadline(timeout, interval)
	for {
		rv = done()
		if rv || !deadline.After(time.Now()) {
			break
		}
		time.Sleep(interval)
	}
	return
}

func setDeadline(timeout, i time.Duration,
) (deadline time.Time, interval time.Duration) {

	deadline = time.Now().Add(timeout)
	interval = i
	if 0 == interval {
		interval = timeout / 10
		if interval < 100*time.Millisecond {
			interval = 100 * time.Millisecond
			if interval > timeout {
				interval = timeout
			}
		}
	} else if interval > timeout {
		interval = timeout
	}
	return
}
