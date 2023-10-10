package unet

import (
	"fmt"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"
)

// enables use of recvmmsg and sendmmsg system calls
type UdpEndpoint struct {
	Iov  []syscall.Iovec // must be on heap, so keep here
	Hdrs []MMsghdr       // must be on heap, so keep here
}

// this barely inlines (budget: 78)
func RecvMsg(fd, msghdr, flags uintptr) (nread int, err syscall.Errno) {
	var rv1 uintptr
	rv1, _, err = syscall.Syscall(
		syscall.SYS_RECVMSG, fd, // one less M
		msghdr, flags) //syscall.MSG_WAITALL,
	return int(rv1), err
}

// this barely inlines (budget: 78)
func RecvMMsg(fd, hdrs, hdrslen uintptr) (messages int, err syscall.Errno) {
	var rv1 uintptr
	rv1, _, err = syscall.Syscall6(
		SYS_RECVMMSG, fd,
		hdrs, hdrslen,
		syscall.MSG_WAITFORONE, 0, 0)
	return int(rv1), err
}

func RawRecvMMsg(fd, hdrs, hdrslen uintptr) (messages int, err syscall.Errno) {
	var rv1 uintptr
	rv1, _, err = syscall.RawSyscall6(
		SYS_RECVMMSG, fd,
		hdrs, hdrslen,
		syscall.MSG_WAITFORONE, 0, 0)
	return int(rv1), err
}

// receive messages from fd
//
// after this call, you will need to:
// - loop thru the messages and process them
// - for each message, reset NTranferred back to 0
// - (optionally) provide new buffers for the iovs used (if not reusing them)
//
// this code is escape optimized
func (this *UdpEndpoint) RecvMMsg(fd int) (messages int, err syscall.Errno) {
	return RecvMMsg(
		uintptr(fd),
		uintptr(unsafe.Pointer(&this.Hdrs[0])),
		uintptr(len(this.Hdrs)))
}

// send n messages (previously set up in Hdrs) on fd
func (this *UdpEndpoint) SendMMsgRetry(fd, n int) (retries int, err syscall.Errno) {
	return SendMMsgRetry(uintptr(fd), this.Hdrs, n)
}

// this barely inlines (budget: 78)
func SendMsg(fd, hdr, flags uintptr) (nsent int, err syscall.Errno) {

	var rv1 uintptr
	rv1, _, err = syscall.Syscall(
		syscall.SYS_SENDMSG, fd, // one less M
		hdr, flags)
	return int(rv1), err
}

// this barely inlines (budget: 78)
func SendMMsg(fd, hdrs, hdrslen uintptr) (nsent int, err syscall.Errno) {

	var rv1 uintptr
	rv1, _, err = syscall.Syscall6(
		SYS_SENDMMSG, fd,
		hdrs, hdrslen,
		0, 0, 0)
	return int(rv1), err
}

func RawSendMMsg(fd, hdrs, hdrslen uintptr) (nsent int, err syscall.Errno) {

	var rv1 uintptr
	rv1, _, err = syscall.RawSyscall6(
		SYS_SENDMMSG, fd,
		hdrs, hdrslen,
		0, 0, 0)
	return int(rv1), err
}

// We are making syscall, so we need to take care that parameters we pass
// will not be gc'd or moved by gc.  Easiest way to do that is to ensure
// that all values are on the heap.
//
// this code is escape optimized
func SendMMsgRetry(
	fd uintptr,
	hdrs []MMsghdr,
	avail int,
) (
	retries int,
	err syscall.Errno,
) {

	start := 0
	var messages int
	//
	// sendmmsg may fail for spurious reasons, so retry as necessary
	//
retry:
	messages, err = SendMMsg(
		fd,
		uintptr(unsafe.Pointer(&hdrs[start])),
		uintptr(avail))

	if 0 != err {
		//
		// connected udp sockets will get ECONNREFUSED if remote endpoint not
		// up at the moment, but that can clear
		//
		if err == syscall.EINTR || err == syscall.EAGAIN ||
			err == syscall.ECONNREFUSED {

			retries++
			runtime.Gosched()
			err = 0
			goto retry

		} else {
			return
		}

	} else if messages != avail {
		if 0 >= messages {
			panic("messages not positive: " + strconv.Itoa(messages))

		} else if messages > avail {
			panic("should not be able to send more messages than available")
		}
		retries++
		start += messages
		avail -= messages
		goto retry
	}
	return
}

// suitable as nameFill parameter for receiving endpoints
func (this *UdpEndpoint) RecvNamer() (name *byte, namelen uint32) {
	namelen = syscall.SizeofSockaddrAny
	name = &make([]byte, namelen)[0]
	return
}

// suitable to create function for nameFill parameter of sending endpoints where
// all packets will go to the same place.
//
// The space param needs to point to heap storage and be at least 28 bytes.
func (this *UdpEndpoint) SendNamer(
	dst syscall.Sockaddr,
	space []byte,
) (
	rv func() (*byte, uint32),
) {
	name, namelen, err := RawSockaddrAsNameBytes(dst, space)
	if err != nil {
		panic(fmt.Sprintf("should not happen!  unable to set up sockaddr: %s", err))
	}
	return func() (*byte, uint32) {
		return name, namelen
	}
}

// returns a func that is suitable for use with SetupVector
func (this *UdpEndpoint) IovFiller(size int) func([]syscall.Iovec) {
	return func(iov []syscall.Iovec) {
		for i, N := 0, len(iov); i < N; i++ {
			b := make([]byte, size)
			iov[i].Base = &b[0]
			iov[i].Len = uint64(size)
		}
	}
}

// setup the initial vectors.
//
// for sending, name should be set.  for receiving, name should be nil.
//
// if iovFill is nil, then it is up to you to later do that.
//
// if nameFill is nil, then name and namelen will be left empty.  This is a
// good option for connected datagram sockets.
func (this *UdpEndpoint) SetupVectors(
	messages, iovsPer int,
	iovFill func(iov []syscall.Iovec),
	nameFill func() (name *byte, namelen uint32),
) {
	if 0 >= messages {
		panic("messages must be positive")
	} else if 0 >= iovsPer {
		panic("iovsPer must be positive")
	}

	this.Iov = make([]syscall.Iovec, messages*iovsPer)
	this.Hdrs = make([]MMsghdr, messages)
	iop := 0
	for i := 0; i < messages; i++ {
		if nil != iovFill {
			iovFill(this.Iov[iop : iop+iovsPer])
		}
		this.Hdrs[i].Iov = &this.Iov[iop]
		this.Hdrs[i].Iovlen = uint64(iovsPer)
		if nil != nameFill {
			this.Hdrs[i].Name, this.Hdrs[i].Namelen = nameFill()
		}
		iop += iovsPer
	}
}
