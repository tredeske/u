package unet

import (
	"errors"
	"io"
	"syscall"
	"time"

	"github.com/tredeske/u/uerr"
)

// allows polling a socket for activity
type Poller struct {
	epfd    int
	cntlRFd int
	cntlWFd int
	byFd    map[int]*Polled
	events  [64]syscall.EpollEvent
	started bool
}

type Polled struct {
	NearAddr Address // optional - use as lookup for OnInput
	Sock     *Socket
	fd       int

	//
	// if set, call when socket hangup
	//
	OnHup func(p *Polled) (ok bool, err error)

	//
	// if set, call when input is ready on socket during Poll().
	//
	// stop polling if !ok
	//
	OnInput func(p *Polled) (ok bool, err error)

	//
	// if set, call when error queue is ready on socket
	//
	OnErrorQ func(p *Polled) (ok bool, err error)
}

const (
	errPollNoCallbacks_ = uerr.Const("Must set OnErrorQ or OnInput")
	errPollFd_          = uerr.Const("Socket fd not ok")
	errPollNotStarted_  = uerr.Const("Must Open Poller before")
)

func (this *Poller) IsStarted() bool { return this.started }

// must be called by thread running Poller
// to cause close from another thread, send a cntl message
func (this *Poller) Close() {
	if this.started {
		epfd := this.epfd
		if -1 != epfd {
			this.epfd = -1
			syscall.Close(epfd)
		}
		fd := this.cntlWFd
		if -1 != fd {
			this.cntlWFd = -1
			syscall.Close(fd)
		}
		fd = this.cntlRFd
		if -1 != fd {
			this.cntlRFd = -1
			syscall.Close(fd)
		}
	}
}

func (this *Poller) Open() (err error) {
	if this.started {
		return errors.New("Poller already started")
	}

	this.cntlRFd = -1
	this.cntlWFd = -1
	this.byFd = make(map[int]*Polled, 32)
	this.epfd, err = syscall.EpollCreate1(0)
	if err != nil {
		return
	}
	this.started = true
	return
}

// if other threads need to tell the thread running Poller things send a nudge.
// the actual message should be in a chan or something.
func (this *Poller) NudgeControl() (err error) {
	if !this.started {
		return errPollNotStarted_
	}
	buff := [1]byte{}
	_, err = syscall.Write(this.cntlWFd, buff[:])
	return
}

// if other threads need to tell this thread (the one running Poller) things, then
// use this to add a control pipe to activate your supplied callback
func (this *Poller) AddControlPipe(onCntl func() (bool, error)) (err error) {
	if !this.started {
		return errPollNotStarted_
	} else if nil == onCntl {
		return errPollNoCallbacks_
	}
	var fds [2]int
	err = syscall.Pipe(fds[:])
	if err != nil {
		return
	}
	this.cntlRFd = fds[0]
	this.cntlWFd = fds[1]
	polled := &Polled{
		fd: fds[0],
		OnHup: func(p *Polled) (ok bool, err error) {
			if -1 != this.cntlRFd {
				syscall.Close(this.cntlRFd)
				this.cntlRFd = -1
			}
			if -1 != this.cntlWFd {
				syscall.Close(this.cntlWFd)
				this.cntlWFd = -1
			}
			return false, io.EOF
		},
		OnInput: func(p *Polled) (ok bool, err error) {
			buff := [24]byte{}
			_, err = syscall.Read(this.cntlRFd, buff[:])
			if err != nil {
				return false, err
			}
			return onCntl()
		},
	}
	var event syscall.EpollEvent
	event.Events |= syscall.EPOLLIN
	event.Fd = int32(polled.fd)
	err = syscall.EpollCtl(this.epfd, syscall.EPOLL_CTL_ADD, polled.fd, &event)
	if err != nil {
		return
	}
	this.byFd[polled.fd] = polled
	return
}

func (this *Poller) Add(polled *Polled) (err error) {
	const errSockUnset = uerr.Const("Socket not set")
	if !this.started {
		return errPollNotStarted_
	} else if nil == polled.Sock {
		return errSockUnset
	} else if nil == polled.OnErrorQ && nil == polled.OnInput &&
		nil == polled.OnHup {
		return errPollNoCallbacks_
	}
	var ok bool
	polled.fd, ok = polled.Sock.Fd.Get()
	if !ok {
		return errPollFd_
	}

	var event syscall.EpollEvent
	if nil != polled.OnErrorQ {
		event.Events |= syscall.EPOLLERR
	}
	if nil != polled.OnInput {
		event.Events |= syscall.EPOLLIN
	}
	event.Fd = int32(polled.fd)
	err = syscall.EpollCtl(this.epfd, syscall.EPOLL_CTL_ADD, polled.fd, &event)
	if err != nil {
		return
	}
	this.byFd[polled.fd] = polled
	return
}

func (this *Poller) Remove(polled *Polled) (err error) {
	delete(this.byFd, polled.fd)
	return syscall.EpollCtl(this.epfd, syscall.EPOLL_CTL_DEL, polled.fd, nil)
}

// Poll forever (or until stopped)
// Returning false if callback returned false
func (this *Poller) PollForever() (ok bool, err error) {
	for {
		ok, err = this.Poll(-1)
		if !ok || err != nil {
			return
		}
	}
}

// Poll for duration
// Returning false if callback returned false
func (this *Poller) PollFor(timeout time.Duration) (ok bool, err error) {
	if 0 > timeout {
		panic("negative timeout not allowed!")
	}
	startT := time.Now()
	for {
		since := time.Since(startT)
		if since > timeout {
			break
		}
		millis := int((timeout - since) / 1000000)
		ok, err = this.Poll(millis)
		if !ok || err != nil {
			return
		}
	}
	return true, nil
}

// Poll, waiting up to millis (-1 if forever) for input
// Returning false if a callback returned false
func (this *Poller) Poll(millis int) (ok bool, err error) {

	nevents, err := syscall.EpollWait(this.epfd, this.events[:], millis)
	if err != nil {
		if syscall.EINTR == err {
			return true, nil
		}
		return
	}

	for i := 0; i < nevents; i++ {
		event := &this.events[i]

		var polled *Polled
		polled, ok = this.byFd[int(event.Fd)]
		if !ok {
			continue
		} else if 0 != (syscall.EPOLLHUP & event.Events) {
			err = this.Remove(polled)
			if err != nil {
				return false, err
			}
			if nil != polled.OnHup {
				ok, err = polled.OnHup(polled)
				if !ok || err != nil {
					return false, err
				}
			}
			continue
		}
		if 0 != (syscall.EPOLLERR&event.Events) && nil != polled.OnErrorQ {
			ok, err = polled.OnErrorQ(polled)
			if !ok || err != nil {
				return false, err
			}
		}
		if 0 != (syscall.EPOLLIN&event.Events) && nil != polled.OnInput {
			ok, err = polled.OnInput(polled)
			if !ok || err != nil {
				return false, err
			}
		}
	}
	return true, nil
}
