package unet

import (
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"syscall"
	"time"
	"unsafe"

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
//	sock, err := NewSocket().
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

func (this *Socket) Log(msg string, args ...any) *Socket {
	if nil == this.Error {
		ulog.Printf(msg, args...)
	}
	return this
}

func (this *Socket) SetFarIpPort(ip net.IP, port int, unless ...bool) *Socket {
	return this.SetFarAddr(AsSockaddr(ip, port), unless...)
}

func (this *Socket) SetFarAddress(far Address, unless ...bool) *Socket {
	if this.canDo(unless) && !far.IsEitherZero() {
		return this.SetFarAddr(far.AsSockaddr(), unless...)
	}
	return this
}

func (this *Socket) SetFarAddr(far syscall.Sockaddr, unless ...bool) *Socket {
	if this.canDo(unless) && !IsSockaddrPortOrIpZero(far) {
		this.FarAddr = far
	}
	return this
}

func (this *Socket) ResolveFarAddr(host string, port int, unless ...bool) *Socket {
	if this.canDo(unless) {
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

// if near is not set, then will be set to 0.0.0.0:0
func (this *Socket) SetNearAddress(near Address, unless ...bool) *Socket {
	return this.SetNearAddr(near.AsSockaddr(), unless...)
}

func (this *Socket) SetNearAddr(near syscall.Sockaddr, unless ...bool) *Socket {
	if this.canDo(unless) {
		if nil == near {
			this.Error = errors.New("No near provided")
			return this
		}
		this.NearAddr = near
	}
	return this
}

func (this *Socket) ResolveNearAddr(host string, port int, unless ...bool) *Socket {
	if this.canDo(unless) {
		if 0 == len(host) {
			host = "0.0.0.0"
		}
		this.NearAddr, this.Error = ResolveSockaddr(host, port)
		this.closeIfError()
	}
	return this
}

// construct Socket from provided mfd, transferring state from mfd to this.
//
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

// construct a temporary Socket from mfd, not transferring state.
func (this *Socket) Temp(mfd ManagedFd) *Socket {
	if mfd.IsDisabledOrClosed() {
		this.Error = ErrFdDisabled
	} else {
		this.Fd = mfd
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
	const errNotSet = uerr.Const("src or dst addr must be set before construct")
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
		this.Error = errNotSet
	} else {
		family, this.Error = SockaddrFamily(addr)
		if this.Error != nil {
			this.Error = uerr.Chainf(this.Error, "getting sockaddr family")
		}
	}
	return
}

func (this *Socket) ShutdownRead() bool { return this.Fd.ShutdownRead() }

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
	if this.canDo(unless) && this.goodToGo() {
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
			this.Error = uerr.Chainf(err, "syscall.Getsockopt")
		} else {
			*value = v
		}
		this.closeIfError()
	}
	return this
}

func (this *Socket) SetOptInt(layer, key, value int, unless ...bool) *Socket {
	fd, good := this.goodFd()
	if good && this.canDo(unless) {
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
	if 0 < size && this.canDo(unless) {
		this.SetOptInt(syscall.IPPROTO_UDP, UDP_SEGMENT, size)
		if this.Error != nil {
			this.Error = uerr.Chainf(this.Error, "GSO requires kernel 4.18+")
		}
	}
	return this
}

func (this *Socket) canDo(unless []bool) bool {
	return (0 == len(unless) || !unless[0]) && nil == this.Error
}

func (this *Socket) IsIpv6() bool {
	const errUnknownIpVer = uerr.Const("Set near or far addr before SetOptMtuDisco")
	switch this.NearAddr.(type) {
	case *syscall.SockaddrInet4:
	case *syscall.SockaddrInet6:
		return true
	default:
		switch this.FarAddr.(type) {
		case *syscall.SockaddrInet4:
		case *syscall.SockaddrInet6:
			return true
		default:
			this.Error = errUnknownIpVer
		}
	}
	return false
}

// MTU Discovery option
//
// https://man7.org/linux/man-pages/man7/ip.7.html
//
// /usr/include/bits/in.h
type MtuDisco int

const (
	MtuDiscoNone  MtuDisco = 0 // unset (leave as system default)
	MtuDiscoDo    MtuDisco = 1 // always set DF
	MtuDiscoDont  MtuDisco = 2 // do not set DF
	MtuDiscoProbe MtuDisco = 3 // set DF, ignore path MTU
	MtuDiscoWant  MtuDisco = 4 // fragment according to path MTU, or set DF
	MtuDiscoIntfc MtuDisco = 5 // always use intfc MTU, do not set DF, ignore icmp
	MtuDiscoOmit  MtuDisco = 6 // Like MtuDiscoIntfc, but all pkts to be fragmented
)

func (this *Socket) SetOptMtuDiscover(disco MtuDisco, unless ...bool) *Socket {
	if MtuDiscoNone != disco && this.canDo(unless) {
		level := syscall.IPPROTO_IP
		opt := syscall.IP_MTU_DISCOVER
		var val int
		if !this.IsIpv6() {
			switch disco {
			case MtuDiscoDo:
				val = syscall.IP_PMTUDISC_DO
			case MtuDiscoDont:
				val = syscall.IP_PMTUDISC_DONT
			case MtuDiscoProbe:
				val = syscall.IP_PMTUDISC_PROBE
			case MtuDiscoWant:
				val = syscall.IP_PMTUDISC_WANT
			case MtuDiscoIntfc:
				val = unix.IP_PMTUDISC_INTERFACE
			case MtuDiscoOmit:
				val = unix.IP_PMTUDISC_OMIT
			default:
				panic("unknown path mtu disco")
			}
		} else { // ipv6
			level = syscall.IPPROTO_IPV6
			opt = syscall.IPV6_MTU_DISCOVER
			switch disco {
			case MtuDiscoDo:
				val = syscall.IPV6_PMTUDISC_DO
			case MtuDiscoDont:
				val = syscall.IPV6_PMTUDISC_DONT
			case MtuDiscoProbe:
				val = syscall.IPV6_PMTUDISC_PROBE
			case MtuDiscoWant:
				val = syscall.IPV6_PMTUDISC_WANT
			case MtuDiscoIntfc:
				val = unix.IPV6_PMTUDISC_INTERFACE
			case MtuDiscoOmit:
				val = unix.IPV6_PMTUDISC_OMIT
			default:
				panic("unknown path mtu disco")
			}
		}
		this.SetOptInt(level, opt, val)
		if this.Error != nil {
			this.Error = uerr.Chainf(this.Error, "IP_MTU_DISCOVER")
		}
	}
	return this
}

// If socket is connected, can get the MTU with this, which will either be the
// MTU of the interface, or the path MTU (PMTU) discovered from ICMP and cached
// in the kernel.
//
// Especially handy after EMSGSIZE.
func (this *Socket) GetOptMtu(mtu *int, unless ...bool) *Socket {
	if this.canDo(unless) {
		level := syscall.IPPROTO_IP
		opt := syscall.IP_MTU
		if this.IsIpv6() {
			level = syscall.IPPROTO_IPV6
			opt = syscall.IPV6_MTU
		}
		this.GetOptInt(level, opt, mtu)
	}
	return this
}

func (this *Socket) IpOverhead() (rv int) {
	rv = IP_OVERHEAD
	if this.IsIpv6() {
		rv = IP6_OVERHEAD
	}
	return
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
	if 0 < timeout && this.canDo(unless) {
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
	if 0 < timeout && this.canDo(unless) {
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
	if 0 < size && this.canDo(unless) {
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
	if 0 < size && this.canDo(unless) {
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
	if this.canDo(unless) && this.goodToGo() {
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
	if val, enabled := tristateEnabled(tristate); enabled {
		switch this.NearAddr.(type) {
		case *syscall.SockaddrInet4:
			return this.SetOptInt(syscall.IPPROTO_IP, syscall.IP_PKTINFO, val)
		case *syscall.SockaddrInet6:
			this.SetOptInt(syscall.IPPROTO_IPV6, syscall.IPV6_V6ONLY, 0)
			return this.SetOptInt(syscall.IPPROTO_IPV6, syscall.IPV6_RECVPKTINFO, val)
		default:
			this.Error = errors.New("set near addr before set recv_pkt_info")
		}
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
	if good && this.canDo(unless) {
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

func (this *Socket) GetFarAddress(rv *Address) *Socket {
	if nil == this.FarAddr {
		this.GetPeerName()
	}
	if nil != this.FarAddr {
		rv.FromSockaddr(this.FarAddr)
	}
	return this
}

func (this *Socket) GetSockName(unless ...bool) *Socket {
	fd, good := this.goodFd()
	if good && this.canDo(unless) {
		var addr syscall.Sockaddr
		addr, this.Error = syscall.Getsockname(fd)
		this.closeIfError()
		if nil == this.Error {
			this.NearAddr = addr
		}
	}
	return this
}

func (this *Socket) GetPeerName(unless ...bool) *Socket {
	fd, good := this.goodFd()
	if good && this.canDo(unless) {
		var addr syscall.Sockaddr
		addr, this.Error = syscall.Getpeername(fd)
		this.closeIfError()
		if nil == this.Error {
			this.FarAddr = addr
		}
	}
	return this
}

func (this *Socket) Listen(depth int, unless ...bool) *Socket {
	fd, good := this.goodFd()
	if good && this.canDo(unless) {
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
	if good && this.canDo(unless) {
		if nil == this.FarAddr {
			this.Error = errors.New("FarAddr must be set before Connect")
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
func (this *Socket) Done() (s *Socket, err error) {
	this.CancelDeadline()
	this.closeIfError()
	return this, this.Error
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

func (this *Socket) Send(buff []byte, flags int) (err error) {
	fd, good := this.goodFd()
	if good {
		err = unix.Send(fd, buff, flags)
	} else {
		err = this.Error
	}
	return
}

func (this *Socket) SendTo(
	buff []byte, flags int, to syscall.Sockaddr,
) (
	err error,
) {
	fd, good := this.goodFd()
	if good {
		if nil == to {
			to = this.FarAddr
		}
		err = syscall.Sendto(fd, buff, flags, to)
	} else {
		err = this.Error
	}
	return
}

func (this *Socket) SendMsg(
	msghdr *syscall.Msghdr, flags int,
) (
	nsent int, err error,
) {
	fd, good := this.goodFd()
	if good {
		rv1, _, errno := syscall.Syscall(syscall.SYS_SENDMSG,
			uintptr(fd), uintptr(unsafe.Pointer(msghdr)), uintptr(flags))
		nsent = int(rv1)
		if 0 != errno {
			err = errno
		}
	} else {
		err = this.Error
	}
	return
}

// wrapper for go syscall, which require alloc of from on each recv
// useful for simple, non-performant cases
func (this *Socket) RecvFrom(
	buff []byte, flags int,
) (
	nread int, from syscall.Sockaddr, err error,
) {
	fd, good := this.goodFd()
	if good {
		nread, from, err = syscall.Recvfrom(fd, buff, flags)
	} else {
		err = this.Error
	}
	return
}

func (this *Socket) RecvMsg(
	msghdr *syscall.Msghdr,
	flags int,
) (
	nread int, err error,
) {
	fd, good := this.goodFd()
	if good {
		rv1, _, errno := syscall.Syscall(syscall.SYS_RECVMSG,
			uintptr(fd), uintptr(unsafe.Pointer(msghdr)), uintptr(flags))
		nread = int(rv1)
		if 0 != errno {
			err = errno
		}
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
