package unet

import (
	"errors"
	"io"
	"syscall"
	"time"
)

// allows polling a socket for activity
type SinglePoller struct {
	epfd    int
	fd      int
	sock    *Socket
	events  [4]syscall.EpollEvent
	started bool

	//
	// if set, call when input is ready on socket
	//
	OnInput func(fd int) (ok bool, err error)

	//
	// if set, call when error queue is ready on socket
	//
	OnErrorQ func(fd int) (ok bool, err error)
}

func (this *SinglePoller) Close() {
	if this.started && -1 != this.epfd {
		fd := this.epfd
		this.epfd = -1
		syscall.Close(fd)
	}
}

func (this *SinglePoller) Open(sock *Socket) (err error) {
	if this.started {
		return errors.New("SinglePoller already started")
	}

	defer func() {
		if err != nil {
			this.Close()
		}
	}()

	this.started = true
	this.epfd = -1
	epfd, err := syscall.EpollCreate1(0)
	if err != nil {
		return
	}
	this.epfd = epfd

	var ok bool
	this.fd, ok = sock.Fd.Get()
	if !ok {
		return errors.New("Socket fd not ok")
	}

	var event syscall.EpollEvent
	if nil != this.OnErrorQ {
		event.Events |= syscall.EPOLLERR
	}
	if nil != this.OnInput {
		event.Events |= syscall.EPOLLIN
	}
	event.Fd = int32(this.fd)
	err = syscall.EpollCtl(this.epfd, syscall.EPOLL_CTL_ADD, this.fd, &event)
	return
}

// Poll forever (or until stopped)
// Returning false if callback returned false
func (this *SinglePoller) PollForever() (ok bool, err error) {
	for {
		ok, err = this.Poll(-1)
		if !ok || err != nil {
			return
		}
	}
}

// Poll for duration
// Returning false if callback returned false
func (this *SinglePoller) PollFor(timeout time.Duration) (ok bool, err error) {
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
func (this *SinglePoller) Poll(millis int) (ok bool, err error) {

	nevents, err := syscall.EpollWait(this.epfd, this.events[:], millis)
	if err != nil {
		return
	}

	for i := 0; i < nevents; i++ {
		event := &this.events[i]
		if int(event.Fd) != this.fd {
			panic("should not happen - epoll wrong fd")
		} else if 0 != (syscall.EPOLLHUP & event.Events) {
			err = io.EOF
			return
		}
		if 0 != (syscall.EPOLLERR&event.Events) && nil != this.OnErrorQ {
			ok, err = this.OnErrorQ(this.fd)
			if !ok || err != nil {
				return
			}
		}
		if 0 != (syscall.EPOLLIN&event.Events) && nil != this.OnInput {
			ok, err = this.OnInput(this.fd)
			if !ok || err != nil {
				return
			}
		}
	}
	return true, nil
}
