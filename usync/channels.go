package usync

import "time"

//
// Ignore any panics.
//
// Use: defer usync.IgnorePanic()
//
func IgnorePanic() {
	recover()
}

//
// a channel to signal death
//
type DeathChan chan struct{}

//
// a new channel of death!
//
func NewDeathChan() (rv DeathChan) {
	return make(chan struct{})
}

//
// writer: signal to any reader it's time to die
//
func (this DeathChan) Close() {
	//defer func() { recover() }()
	defer IgnorePanic()
	close(this)
}

//
// reader: check to see if it's time to die
//
func (this DeathChan) Check() (timeToDie bool) {
	select {
	case _, ok := <-this:
		timeToDie = !ok
	default:
	}
	return
}

//
// reader: wait up to timeout for death to occur
//
func (this DeathChan) Wait(timeout time.Duration) (timeToDie bool) {
	timeToDie = this.Check()
	if !timeToDie {
		toc := time.After(timeout)
		select {
		case _, ok := <-this:
			timeToDie = !ok
		case <-toc:
		}
	}
	return
}

/*

these are more examples than anything else.

they do not check for channel closed condition, though




// non-blocking read of channel
//
// note: there does not appear to be a way to cast channels, so your channel
// will need to be as specified
//
func ChannelCheck(ch chan interface{}) (rv interface{}, gotIt bool) {
	select {
	case rv = <-ch:
		gotIt = true
	default:
	}
	return
}

// get item from channel or timeout
//
// note: there does not appear to be a way to cast channels, so your channel
// will need to be as specified
//
func ChannelWait(ch chan interface{}, timeout time.Duration) (rv interface{}, gotIt bool) {
	rv, gotIt = ChannelCheck(ch)
	if !gotIt {
		toc := time.After(timeout)
		select {
		case rv = <-ch:
			gotIt = true
		case <-toc:
		}
	}
	return
}

// non-blocking read of channel
//
// note: there does not appear to be a way to cast channels, so your channel
// will need to be as specified
//
func BoolChCheck(ch chan bool) (rv bool, gotIt bool) {
	select {
	case rv = <-ch:
		gotIt = true
	default:
	}
	return
}

// get item from channel or timeout
//
// note: there does not appear to be a way to cast channels, so your channel
// will need to be as specified
//
func BoolChWait(ch chan bool, timeout time.Duration) (rv bool, gotIt bool) {
	rv, gotIt = BoolChCheck(ch)
	if !gotIt {
		toc := time.After(timeout)
		select {
		case rv = <-ch:
			gotIt = true
		case <-toc:
		}
	}
	return
}

// non-blocking read of channel
//
// note: there does not appear to be a way to cast channels, so your channel
// will need to be as specified
//
func ErrorChCheck(ch chan error) (rv error, gotIt bool) {
	select {
	case rv = <-ch:
		gotIt = true
	default:
	}
	return
}

// get item from channel or timeout
//
// note: there does not appear to be a way to cast channels, so your channel
// will need to be as specified
//
func ErrorChWait(ch chan error, timeout time.Duration) (rv error, gotIt bool) {
	rv, gotIt = ErrorChCheck(ch)
	if !gotIt {
		toc := time.After(timeout)
		select {
		case rv = <-ch:
			gotIt = true
		case <-toc:
		}
	}
	return
}

// non-blocking read of channel
//
// note: there does not appear to be a way to cast channels, so your channel
// will need to be as specified
//
func StringChCheck(ch chan string) (rv string, gotIt bool) {
	select {
	case rv = <-ch:
		gotIt = true
	default:
	}
	return
}

// get item from channel or timeout
//
// note: there does not appear to be a way to cast channels, so your channel
// will need to be as specified
//
func StringChWait(ch chan string, timeout time.Duration) (rv string, gotIt bool) {
	rv, gotIt = StringChCheck(ch)
	if !gotIt {
		toc := time.After(timeout)
		select {
		case rv = <-ch:
			gotIt = true
		case <-toc:
		}
	}
	return
}
*/
