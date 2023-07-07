package unet

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"
	"golang.org/x/sys/unix"
)

// from golang.org/x/sys/unix
const (
	UDP_SEGMENT int = 103
)

const (
	ErrNotInitialized     = uerr.Const("Not initialized.  Call Construct first.")
	ErrFdDisabled         = uerr.Const("File descriptor disabled")
	ErrFdTransfer         = uerr.Const("Could not transfer file descriptor")
	ErrAlreadyInitialized = uerr.Const("Already initialized")
	ErrNil                = uerr.Const("sock is nil")
)

// get the underlying file descriptor from the socket.  we also have to
// return the underlying os.File and the caller has to hold onto that
// or it will be gc'd and the fd will be unusable.
//
// WARNING: after this call, setting deadlines (SetReadDeadline) on conn
// will no longer work!  See: https://github.com/golang/go/issues/7605
//
// NOTE: This call uses dup(), so there will be 2 file descriptors when done
func GetSocketFd(conn *net.TCPConn) (fd int, f *os.File) {
	f, err := conn.File()
	if err != nil {
		panic("unable to get fd from TCPConn: " + err.Error())
	}
	return int(f.Fd()), f
}

// Enable reasonable access to the Berkeley socket API.
//
// needed when golang does not allow in high level interface, or for things
// like setting socket options before bind.
//
// implements net.Conn, io.Closer, io.Reader, io.Writer
//
//	sock := unet.Socket{}
//	err := sock.
//	    ResolveFarAddr(host, port).
//	    ConstructTcp().
//	    SetTimeout(7*time.Second).
//	    Connect().
//	    Done()
type Socket struct {
	Fd        ManagedFd
	FarAddr   syscall.Sockaddr
	NearAddr  syscall.Sockaddr
	Error     error
	deadliner *deadliner_
}

func NewSocket() *Socket { return &Socket{} }

type SockOpt func(s *Socket) (err error)

// be careful!  if Fd holds a live fd, it may be lost!
func (this *Socket) Reset() *Socket {
	this.Fd.Clear()
	this.FarAddr = nil
	this.NearAddr = nil
	this.Error = nil
	return this
}

func (this *Socket) Log(msg string, args ...interface{}) *Socket {
	if nil == this.Error {
		ulog.Printf(msg, args...)
	}
	return this
}

func (this *Socket) SetFarIpPort(ip net.IP, port int, unless ...bool) *Socket {
	return this.SetFarAddr(AsSockaddr(ip, port), unless...)
}

func (this *Socket) SetFarAddress(far *Address, unless ...bool) *Socket {
	return this.SetFarAddr(far.AsSockaddr(), unless...)
}

func (this *Socket) SetFarAddr(far syscall.Sockaddr, unless ...bool) *Socket {
	if nil == far {
		this.Error = errors.New("No far provided")
		return this
	}
	if nil == this.Error && (0 == len(unless) || !unless[0]) {
		this.FarAddr = far
	}
	return this
}

func (this *Socket) ResolveFarAddr(host string, port int, unless ...bool) *Socket {
	if nil == this.Error && (0 == len(unless) || !unless[0]) {
		if 0 == len(host) {
			this.Error = errors.New("No far host provided")
			return this
		} else if 0 >= port {
			this.Error = errors.New("No far port provided")
			return this
		}
		this.FarAddr, this.Error = ResolveSockaddr(host, port)
		this.closeIfError()
	}
	return this
}

func (this *Socket) SetNearIpPort(ip net.IP, port int, unless ...bool) *Socket {
	return this.SetNearAddr(AsSockaddr(ip, port), unless...)
}

func (this *Socket) SetNearAddress(near *Address, unless ...bool) *Socket {
	return this.SetNearAddr(near.AsSockaddr(), unless...)
}

func (this *Socket) SetNearAddr(near syscall.Sockaddr, unless ...bool) *Socket {
	if nil == this.Error && (0 == len(unless) || !unless[0]) {
		if nil == near {
			this.Error = errors.New("No near provided")
			return this
		}
		this.NearAddr = near
	}
	return this
}

func (this *Socket) ResolveNearAddr(host string, port int, unless ...bool) *Socket {
	if nil == this.Error && (0 == len(unless) || !unless[0]) {
		if 0 == len(host) {
			host = "0.0.0.0"
		}
		this.NearAddr, this.Error = ResolveSockaddr(host, port)
		this.closeIfError()
	}
	return this
}

// if transfer of state fails, then fd will be closed
func (this *Socket) ConstructFd(mfd *ManagedFd) *Socket {
	if !this.Fd.From(mfd) {
		mfd.Close()
		if this.Fd.IsDisabled() {
			this.Error = ErrFdDisabled
		} else {
			this.Error = ErrAlreadyInitialized
		}
	}
	return this
}

func (this *Socket) ConstructTcp() *Socket {
	return this.Construct(syscall.SOCK_STREAM, syscall.IPPROTO_TCP)
}

func (this *Socket) ConstructUdp() *Socket {
	return this.Construct(syscall.SOCK_DGRAM, syscall.IPPROTO_UDP)
}

func (this *Socket) Construct(sockType, proto int) *Socket {
	if this.Fd.IsSet() {
		this.Error = ErrAlreadyInitialized
	}
	if nil == this.Error {
		family := this.getFamily()
		if this.Error != nil {
			return this
		}
		var fd int
		fd, this.Error = syscall.Socket(family, sockType, proto)
		if this.Error != nil {
			this.Error = uerr.Chainf(this.Error, "syscall.Socket")
		} else if !this.Fd.Set(fd) {
			syscall.Close(fd)
			this.Error = ErrAlreadyInitialized
		}
	}
	return this
}

func (this *Socket) getFamily() (family int) {
	var addr syscall.Sockaddr
	if nil != this.FarAddr {
		addr = this.FarAddr
	} else if nil != this.NearAddr {
		addr = this.NearAddr
	} else if !this.Fd.IsClosed() {
		this.GetSockName()
		addr = this.NearAddr
	}
	if nil == addr {
		this.Error = errors.New("src or dst addr must be set before construct")
	} else {
		family, this.Error = SockaddrFamily(addr)
		if this.Error != nil {
			this.Error = uerr.Chainf(this.Error, "getting sockaddr family")
		}
	}
	return
}

func (this *Socket) closeUnconditionally() {
	_, err := this.Fd.Close()
	if err != nil {
		if this.Error != nil {
			this.Error = uerr.Chainf(this.Error,
				"close failed due to: %s", err.Error())
		} else {
			this.Error = err
		}
	}
}

func (this *Socket) SetOpt(opt SockOpt, unless ...bool) *Socket {
	if (0 == len(unless) || !unless[0]) && this.goodToGo() {
		this.Error = opt(this)
		this.closeIfError()
	}
	return this
}

func (this *Socket) GetOptInt(layer, key int, value *int) *Socket {
	fd, good := this.goodFd()
	if good {
		v, err := syscall.GetsockoptInt(fd, layer, key)
		if err != nil {
			this.Error = uerr.Chainf(err, "syscall.Setsockopt")
		} else {
			*value = v
		}
		this.closeIfError()
	}
	return this
}

func (this *Socket) SetOptInt(layer, key, value int, unless ...bool) *Socket {
	fd, good := this.goodFd()
	if good && (0 == len(unless) || !unless[0]) {
		err := syscall.SetsockoptInt(fd, layer, key, value)
		if err != nil {
			this.Error = uerr.Chainf(err, "syscall.Setsockopt")
		}
		this.closeIfError()
	}
	return this
}

func (this *Socket) SetOptTristate(layer, key int, tristate []int) *Socket {
	if opt, enabled := tristateEnabled(tristate); enabled {
		return this.SetOptInt(layer, key, opt)
	}
	return this
}

// if the slice is empty, then default to 1
// rv should be 0 or 1.  any other value (say -1) means make no change
func tristateEnabled(tristate []int) (rv int, enabled bool) {
	rv = 1
	if 0 != len(tristate) {
		rv = tristate[0]
	}
	return rv, 0 == rv || 1 == rv
}

// About GSO (generic segment offload):
//
// https://blog.cloudflare.com/accelerating-udp-packet-transmission-for-quic/
//
// kernel 4.18+ is required.
//
// We have not tested with kernel 4.x, but have tested with 5.4+.
//
// This feature can be controlled using the UDP_SEGMENT socket option:
//
//	setsockopt(fd, SOL_UDP, UDP_SEGMENT, &gso_size, sizeof(gso_size)))
//
// As well as via ancillary data, to control segmentation for each
// sendmsg() call:
//
//	cm = CMSG_FIRSTHDR(&msg);
//	cm->cmsg_level = SOL_UDP;
//	cm->cmsg_type = UDP_SEGMENT;
//	cm->cmsg_len = CMSG_LEN(sizeof(uint15_t));
//	*((uint15_t *) CMSG_DATA(cm)) = gso_size;
//
// Where gso_size is the size of each segment that form the "super buffer"
// passed to the kernel from the application. Once configured, the
// application can provide one contiguous large buffer containing a
// number of packets of gso_size length (as well as a final smaller packet),
// that will then be segmented by the kernel (or the NIC if hardware
// segmentation offloading is supported and enabled).
func (this *Socket) SetOptGso(size int, unless ...bool) *Socket {
	if 0 < size && (0 == len(unless) || !unless[0]) && nil == this.Error {
		this.SetOptInt(syscall.IPPROTO_UDP, UDP_SEGMENT, size)
		if this.Error != nil {
			this.Error = uerr.Chainf(this.Error, "GSO requires kernel 4.18+")
		}
	}
	return this
}

// must be before bind
//
// default to 'on'.  if specified, values are 0 (off), 1 (on) - any other value
// is 'no change'
//
// see https://idea.popcount.org/2014-04-03-bind-before-connect/
func (this *Socket) SetOptReuseAddr(tristate ...int) *Socket {
	return this.SetOptTristate(syscall.SOL_SOCKET, syscall.SO_REUSEADDR, tristate)
}

// set SO_RCVTIMEO on socket if timeout is positive
func (this *Socket) SetOptRcvTimeout(timeout time.Duration, unless ...bool) *Socket {
	if 0 < timeout && (0 == len(unless) || !unless[0]) {
		fd, good := this.goodFd()
		if good {
			tv := syscall.NsecToTimeval(int64(timeout))
			err := syscall.SetsockoptTimeval(
				fd, syscall.SOL_SOCKET, syscall.SO_RCVTIMEO, &tv)
			if err != nil {
				this.Error = uerr.Chainf(err, "syscall.Setsockopt SO_RCVTIMEO")
			}
			this.closeIfError()
		}
	}
	return this
}

// set SO_SNDTIMEO on socket if timeout is positive
func (this *Socket) SetOptSndTimeout(timeout time.Duration, unless ...bool) *Socket {
	if 0 < timeout && (0 == len(unless) || !unless[0]) {
		fd, good := this.goodFd()
		if good {
			tv := syscall.NsecToTimeval(int64(timeout))
			err := syscall.SetsockoptTimeval(
				fd, syscall.SOL_SOCKET, syscall.SO_SNDTIMEO, &tv)
			if err != nil {
				this.Error = uerr.Chainf(err, "syscall.Setsockopt SO_SNDTIMEO")
			}
			this.closeIfError()
		}
	}
	return this
}

// must be before bind
//
// default to 'on'.  if specified, values are 0 (off), 1 (on) - any other value
// is 'no change'
func (this *Socket) SetOptReusePort(tristate ...int) *Socket {
	return this.SetOptTristate(syscall.SOL_SOCKET, unix.SO_REUSEPORT, tristate)
}

func (this *Socket) GetOptRcvBuf(size *int) *Socket {
	return this.GetOptInt(syscall.SOL_SOCKET, syscall.SO_RCVBUF, size)
}

func (this *Socket) SetOptRcvBuf(size int, unless ...bool) *Socket {
	if 0 < size && (0 == len(unless) || !unless[0]) {
		this.SetOptInt(syscall.SOL_SOCKET, syscall.SO_RCVBUF, size)
		var v int
		this.GetOptInt(syscall.SOL_SOCKET, syscall.SO_RCVBUF, &v)
		if nil == this.Error && size > v {
			this.Error = fmt.Errorf("SO_RCVBUF was set to %d, but it is %d. "+
				"check sysctl net.core.rmem_max", size, v)
		}
	}
	return this
}

func (this *Socket) GetOptSndBuf(size *int) *Socket {
	return this.GetOptInt(syscall.SOL_SOCKET, syscall.SO_SNDBUF, size)
}

func (this *Socket) SetOptSndBuf(size int, unless ...bool) *Socket {
	if 0 < size && (0 == len(unless) || !unless[0]) {
		this.SetOptInt(syscall.SOL_SOCKET, syscall.SO_SNDBUF, size)
		var v int
		this.GetOptInt(syscall.SOL_SOCKET, syscall.SO_SNDBUF, &v)
		if nil == this.Error && size > v {
			this.Error = fmt.Errorf("SO_SNDBUF was set to %d, but it is %d. "+
				"check sysctl net.core.wmem_max", size, v)
		}
	}
	return this
}

// set IP DSCP / TOS (priority) bits.
func (this *Socket) SetOptDscpTos(tos byte, unless ...bool) *Socket {
	if (0 == len(unless) || !unless[0]) && this.goodToGo() {
		this.Error = ValidDscpTosCode(tos)
		if nil == this.Error {
			//this.SetOptInt(syscall.IPPROTO_IP, syscall.IP_TOS, int(tos))
			this.SetOptInt(syscall.SOL_SOCKET, syscall.SO_PRIORITY, int(tos))
		}
	}
	return this
}

// for recvmsg/recvmmsg, allow receipt of cmsghdr
//
// default to 'on'.  if specified, values are 0 (off), 1 (on) - any other value
// is 'no change'
func (this *Socket) SetOptRecvPktInfo(tristate ...int) *Socket {
	if opt, enabled := tristateEnabled(tristate); enabled {
		switch this.NearAddr.(type) {
		case *syscall.SockaddrInet4:
			return this.SetOptInt(syscall.IPPROTO_IP, syscall.IP_PKTINFO, opt)
		case *syscall.SockaddrInet6:
			this.SetOptInt(syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, 0)
			return this.SetOptInt(syscall.IPPROTO_IPV6, syscall.IPV6_RECVPKTINFO, opt)
		}
		panic("should not get here")
	}
	return this
}

func (this *Socket) BindTo(host string, port int, unless ...bool) *Socket {
	this.ResolveNearAddr(host, port, unless...)
	return this.Bind(unless...)
}

// see https://idea.popcount.org/2014-04-03-bind-before-connect/
func (this *Socket) Bind(unless ...bool) *Socket {
	fd, good := this.goodFd()
	if good && (0 == len(unless) || !unless[0]) {
		if nil == this.NearAddr {
			this.Error = errors.New("ResolveNearAddr must be called before Bind")
			return this
		}
		this.Error = syscall.Bind(fd, this.NearAddr)
		if this.Error != nil {
			this.Error = uerr.Chainf(this.Error, "syscall.Bind")
		}
		this.closeIfError()
		this.GetSockName()
	}
	return this
}

func (this *Socket) GetNearAddress(rv *Address) *Socket {
	if nil == this.NearAddr {
		this.GetSockName()
	}
	if nil != this.NearAddr {
		rv.FromSockaddr(this.NearAddr)
	}
	return this
}

func (this *Socket) GetSockName(unless ...bool) *Socket {
	fd, good := this.goodFd()
	if good && (0 == len(unless) || !unless[0]) {
		var addr syscall.Sockaddr
		addr, this.Error = syscall.Getsockname(fd)
		this.closeIfError()
		if nil == this.Error {
			this.NearAddr = addr
		}
	}
	return this
}

func (this *Socket) Listen(depth int, unless ...bool) *Socket {
	fd, good := this.goodFd()
	if good && (0 == len(unless) || !unless[0]) {
		if nil == this.NearAddr {
			this.Error = errors.New("ResolveNearAddr must be called before Listen")
			return this
		}
		this.Error = syscall.Listen(fd, depth)
		this.closeIfError()
	}
	return this
}

// accept a connection, storing the fd and addresses in sock
func (this *Socket) Accept(sock *Socket) (err error) {
	if sock.Fd.IsSet() {
		return ErrAlreadyInitialized
	}
	listenFd, good := this.goodFd()
	if good {
		var fd int
		var sa syscall.Sockaddr
		fd, sa, err = syscall.Accept(listenFd)
		if nil != err {
			if this.Fd.IsDisabled() {
				this.Error = ErrFdDisabled
				err = ErrFdDisabled
			}
			return
		}
		if !sock.Fd.Set(fd) {
			syscall.Close(fd)
			if sock.Fd.IsDisabled() {
				sock.Error = ErrFdDisabled
			} else if nil == sock.Error {
				sock.Error = ErrAlreadyInitialized
			}
			return sock.Error
		}
		sock.SetNearAddr(this.NearAddr).SetFarAddr(sa)
	}
	return this.Error
}

func (this *Socket) Connect(unless ...bool) *Socket {
	fd, good := this.goodFd()
	if good && (0 == len(unless) || !unless[0]) {
		if nil == this.FarAddr {
			this.Error = errors.New("ResolveFarAddr must be called before Connect")
			return this
		}
		this.Error = syscall.Connect(fd, this.FarAddr)
		this.closeIfError()
	}
	return this
}

// safely transfer the internal fd to managed, if managed not nil or already set
func (this *Socket) ManageFd(managed *ManagedFd) *Socket {
	if nil != managed && this.goodToGo() {
		if !managed.From(&this.Fd) {
			this.Error = ErrFdTransfer
			this.closeUnconditionally()
		}
	}
	return this
}

// really just for test situations
func (this *Socket) GiveMeTheFreakingFd(pfd *int) *Socket {
	fd, valid := this.Fd.Get()
	if valid {
		*pfd = fd
	} else {
		*pfd = -1
	}
	return this
}

func (this *Socket) SetTimeout(timeout time.Duration) *Socket {
	this.SetDeadline(time.Now().Add(timeout))
	return this
}

// func (this *Socket) ClearTimeout(timeout time.Duration) *Socket {
func (this *Socket) ClearTimeout() *Socket {
	this.CancelDeadline()
	return this
}

// end chain, clean up, return any error
func (this *Socket) Done() (err error) {
	this.CancelDeadline()
	return this.Error
}

// perform user specified validation
func (this *Socket) Then(thenF func(*Socket) error) *Socket {
	if this.goodToGo() {
		this.Error = thenF(this)
		this.closeIfError()
	}
	return this
}

func (this *Socket) closeIfError() *Socket {
	if nil != this.Error {
		this.closeUnconditionally()
	}
	return this
}

func (this *Socket) goodToGo() (good bool) {
	if nil == this.Error {
		open, disabled, _ := this.Fd.GetStatus()
		if disabled {
			this.Error = ErrFdDisabled
		} else if !open {
			this.Error = ErrNotInitialized
		} else {
			good = true
		}
	}
	return
}

func (this *Socket) goodFd() (fd int, good bool) {
	if nil == this.Error {
		fd, good = this.Fd.Get()
		if !good {
			open, disabled, _ := this.Fd.GetStatus()
			if disabled {
				this.Error = ErrFdDisabled
			} else if !open {
				this.Error = ErrNotInitialized
			}
		}
	}
	return
}

// shutdown
func (this *Socket) Disable() {
	if nil != this {
		this.Fd.Disable()
	}
}
func (this *Socket) IsDisabled() bool { return this.Fd.IsDisabled() }

//
// implement net.Conn, io.Closer, io.Reader, io.Writer
//

// preserves disabled state
func (this *Socket) Close() error {
	if nil == this {
		return ErrNil
	}
	this.closeUnconditionally()
	this.CancelDeadline()
	return this.Error
}

func (this *Socket) LocalAddr() net.Addr {
	if nil == this.NearAddr {
		return nil
	}
	addr := &Address{}
	addr.FromSockaddr(this.NearAddr)
	return addr
}

func (this *Socket) RemoteAddr() net.Addr {
	if nil == this.FarAddr {
		return nil
	}
	addr := &Address{}
	addr.FromSockaddr(this.FarAddr)
	return addr
}

func (this *Socket) Read(buff []byte) (nread int, err error) {
	fd, good := this.goodFd()
	if good {
		nread, err = syscall.Read(fd, buff)
		if 0 == nread && nil == err {
			err = io.EOF
		}
	} else {
		err = this.Error
	}
	return
}

func (this *Socket) Write(buff []byte) (nwrote int, err error) {
	fd, good := this.goodFd()
	if good {
		nwrote, err = syscall.Write(fd, buff)
	} else {
		err = this.Error
	}
	return
}

// set a zero deadline to cancel deadline
func (this *Socket) SetDeadline(t time.Time) error {
	if this.goodToGo() {
		d := this.deadliner
		if nil == d {
			if !t.IsZero() {
				this.deadliner = newDeadliner(t, func() { this.Fd.Disable() })
			}
		} else {
			if t.IsZero() {
				d.Cancel()
				this.deadliner = nil
			} else {
				d.Reset(t)
			}
		}
	}
	return this.Error
}

func (this *Socket) CancelDeadline() {
	d := this.deadliner
	this.deadliner = nil
	if nil != d {
		d.Cancel()
	}
}

func (this *Socket) SetReadDeadline(t time.Time) error {
	return this.SetDeadline(t)
}

func (this *Socket) SetWriteDeadline(t time.Time) error {
	return this.SetDeadline(t)
}
