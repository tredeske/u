package u

import (
	"errors"
	"time"
)

var TimeoutError = errors.New("Operation timed out")

//
// invoke done until it returns true or error, or the timeout occurs.
// return error if done errors or timeout occurs
//
func Await(timeout time.Duration, done func() (bool, error)) error {

	deadline := time.Now().Add(timeout)
	interval := 100 * time.Millisecond
	if interval*10 > timeout {
		interval = timeout / 10
	}
	for deadline.After(time.Now()) {
		complete, err := done()
		if err != nil || complete {
			return err
		}
		time.Sleep(interval)
	}
	return TimeoutError
}
