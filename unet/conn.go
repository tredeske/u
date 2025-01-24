package unet

import (
	"errors"
	"io"
	"net"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

// Implement net.Conn based on Socket.
//
// This is more efficient, but slightly more risky.  If the underlying fd is
// closed before all goroutines have ceased using it, a new socket/file/etc might
// be openned that gets the same fd.
//
// Make sure to Shutdown this, which will unstick any goroutines using it.  Then,
// once all goroutines are accounted for, Close.
type Conn struct {
	sock *Socket
	fd   int
}

// wrapper for NewSocketPair that returns Conns instead of Sockets
func NewConnPair() (rv [2]*Conn, err error) {
	pair, err := NewSocketPair()
	if err != nil {
		return
	}
	rv[0], err = pair[0].AsConn()
	if err != nil {
		return
	}
	rv[1], err = pair[1].AsConn()
	return
}

func (this *Conn) Socket() *Socket          { return this.sock }
func (this *Conn) Fd() (fd int, valid bool) { return this.sock.Fd.Get() }
func (this *Conn) ShutdownRead() bool       { return this.sock.Fd.ShutdownRead() }
func (this *Conn) IsShutdown() bool         { return this.sock.Fd.IsDisabled() }

// shutdown both directions, but do not close
//
// useful to unstick goroutines waiting on i/o
func (this *Conn) Shutdown() {
	if nil != this && nil != this.sock {
		this.sock.Fd.Disable()
	}
}

func (this *Conn) SetTimeout(timeout time.Duration) error {
	this.sock.SetDeadline(time.Now().Add(timeout))
	return this.sock.Error
}

func (this *Conn) ClearTimeout() error {
	this.sock.CancelDeadline()
	return this.sock.Error
}

// implement net.Conn
// a zero deadline will cancel deadline
func (this *Conn) SetDeadline(deadline time.Time) error {
	return this.sock.SetDeadline(deadline)
}

func (this *Conn) CancelDeadline() { this.sock.CancelDeadline() }

// implement net.Conn
func (this *Conn) SetReadDeadline(deadline time.Time) error {
	return this.SetDeadline(deadline)
}

// implement net.Conn
func (this *Conn) SetWriteDeadline(deadline time.Time) error {
	return this.SetDeadline(deadline)
}

// implement io.Closer, net.Conn
// preserves disabled state
func (this *Conn) Close() error {
	if nil == this {
		return ErrNil
	}
	return this.sock.Close()
}

// implement net.Conn
func (this *Conn) LocalAddr() net.Addr {
	if nil == this.sock.NearAddr {
		return nil
	}
	addr := &Address{}
	addr.FromSockaddr(this.sock.NearAddr)
	return addr
}

// implement net.Conn
func (this *Conn) RemoteAddr() net.Addr {
	if nil == this.sock.FarAddr {
		return nil
	}
	addr := &Address{}
	addr.FromSockaddr(this.sock.FarAddr)
	return addr
}

// implement io.Reader, net.Conn
func (this *Conn) Read(buff []byte) (nread int, err error) {
	nread, err = syscall.Read(this.fd, buff)
	if 0 == nread && nil == err {
		err = io.EOF
	}
	return
}

// implement io.Writer, net.Conn
func (this *Conn) Write(buff []byte) (nwrote int, err error) {
again:
	n, err := syscall.Write(this.fd, buff)
	nwrote += n
	if nil == err && n != len(buff) {
		buff = buff[n:]
		goto again
	}
	return
}

// UNIX send()
func (this *Conn) Send(buff []byte, flags int) (err error) {
	return unix.Send(this.fd, buff, flags)
}

// UNIX sendto()
func (this *Conn) SendTo(
	buff []byte, flags int, to syscall.Sockaddr,
) (
	err error,
) {
	if nil == to {
		to = this.sock.FarAddr
	}
	return syscall.Sendto(this.fd, buff, flags, to)
}

// UNIX sendmsg()
func (this *Conn) SendMsg(
	msghdr *syscall.Msghdr, flags int,
) (
	nsent int, err error,
) {
	rv1, _, errno := syscall.Syscall(syscall.SYS_SENDMSG,
		uintptr(this.fd), uintptr(unsafe.Pointer(msghdr)), uintptr(flags))
	nsent = int(rv1)
	if 0 != errno {
		err = errno
	}
	return
}

// UNIX recvfrom()
// wrapper for go syscall, which require alloc of from on each recv
// useful for simple, non-performant cases
func (this *Conn) RecvFrom(
	buff []byte, flags int,
) (
	nread int, from syscall.Sockaddr, err error,
) {
	return syscall.Recvfrom(this.fd, buff, flags)
}

// UNIX recvmsg()
func (this *Conn) RecvMsg(
	msghdr *syscall.Msghdr,
	flags int,
) (
	nread int, err error,
) {
	rv1, _, errno := syscall.Syscall(syscall.SYS_RECVMSG,
		uintptr(this.fd), uintptr(unsafe.Pointer(msghdr)), uintptr(flags))
	nread = int(rv1)
	if 0 != errno {
		err = errno
	}
	return
}

// same as recvmsg, but also gets the address that pkt was received on via cmsg
//
// SetOptRecvPktInfo must be set to on for the cmsg info to be there!
func (this *Conn) RecvMsgCmsgDest(
	msghdr *syscall.Msghdr,
	addr *Address,
	flags int,
) (
	nread int, ok bool, err error,
) {
	cmsgB := [48]byte{} // 48 bytes appears to be minimum
	msghdr.Control = &cmsgB[0]
	msghdr.Controllen = uint64(len(cmsgB))

	nread, err = this.RecvMsg(msghdr, flags)
	if err != nil {
		return
	}
	lens := CmsgLens{}
	if !lens.First(msghdr) {
		err = errors.New("No cmsghdr detected")
		return
	}
	ok, err = addr.FromCmsgHdr(&lens)
	return
}
