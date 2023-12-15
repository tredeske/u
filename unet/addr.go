package unet

import (
	"encoding/hex"
	"net"
	"strconv"
	"syscall"
	"unsafe"
)

const (
	InvalidIpv4Str = "192.0.2.1" // RFC5737 - for testing, 192.0.2.0/24
	InvalidIpv6Str = "0100::1"   // RFC6666 - for testing, 0100::/64
)

// a struct that can hold either ipv4 or ipv6 address, plus port
//
// suitable for using as a map key
//
// the net.IP type is a slice, so cannot be used as a key in a map, etc.
//
// When net.IP stores an ipv4 addr, it either does it in a 4 byte slice (uncommon)
// or as an ip4-in-ip6 address in a 16 bytes slice.  That is:
//
//	::ffff:a.b.c.d
//
// So, the ipv4 address is in the last 4 bytes of the slice, preceded by ffff.
//
// We use the ip4-in-ip6 method to store all ipv4 as ipv6.
//
// for ipv6, we currently do not support the zone id, as that is primarily
// for link local addresses, which are not of interest to us.
type Address struct {
	addr1 uint64
	addr2 uint64
	plus  uint64
}

const (
	addrBit_  uint64 = 1 << 17
	portBit_  uint64 = 1 << 16
	portMask_ uint64 = 0xffff
	ipv4Mask_ uint64 = 0xffffffff
	ipv4Bits_ uint64 = 0xffff0000
)

func (this *Address) Clear() {
	this.addr1 = 0
	this.addr2 = 0
	this.plus = 0
}

func (this Address) IsIpv4() bool {
	// addr1 must be 0, while addr2 must be 0xffff0000 in low 32 bits
	return 0 == this.addr1 && ipv4Bits_ == (this.addr2&ipv4Mask_)
}

func (this Address) IsIpv6() bool { return this.IsIpSet() && !this.IsIpv4() }

// are both IP and port set?
func (this Address) IsSet() bool     { return this.IsIpSet() && this.IsPortSet() }
func (this Address) IsIpSet() bool   { return 0 != (this.plus & addrBit_) }
func (this Address) IsPortSet() bool { return 0 != (this.plus & portBit_) }
func (this Address) Port() uint16    { return uint16(this.plus & portMask_) }

// are both ip and port set as zeros?
// we also accept unset port as part of zero port
// this is not the same as unset!
func (this Address) IsZero() bool { return this.IsIpZero() && 0 == this.Port() }

// is either ip or port set as zeros?
// this is not the same as unset!
func (this Address) IsEitherZero() bool {
	return this.IsIpZero() || 0 == this.Port()
}

// is ip set to 0.0.0.0 or ::?
func (this Address) IsIpZero() bool {
	return 0 != (this.plus&addrBit_) && 0 == this.addr1 &&
		// see IsIpv4()
		(0 == this.addr2 || ipv4Bits_ == this.addr2)
}

// is ip set to 0.0.0.0?
func (this Address) IsIpv4Zero() bool {
	return 0 != (this.plus&addrBit_) && 0 == this.addr1 &&
		// see IsIpv4()
		ipv4Bits_ == this.addr2
}

// is ip set to ::?
func (this Address) IsIpv6Zero() bool {
	return 0 != (this.plus&addrBit_) && 0 == this.addr1 && 0 == this.addr2
}

// set ip to '0.0.0.0'
func (this *Address) SetIpv4Zero() {
	this.addr1 = 0
	this.addr2 = ipv4Bits_
	this.plus |= addrBit_
}

// set ip to '::'
func (this *Address) SetIpv6Zero() {
	this.addr1 = 0
	this.addr2 = 0
	this.plus |= addrBit_
}

func (this *Address) AsIp() (rv net.IP) {
	return (*[16]byte)(unsafe.Pointer(this))[:16:16]
}

func (this *Address) AsIpv4() (rv net.IP) {
	return (*[16]byte)(unsafe.Pointer(this))[12:16:16]
}

// populate provided sockaddr based on this
func (this *Address) ToSockaddr4(sa *syscall.SockaddrInet4) {
	sa.Port = int(this.Port())
	copy(sa.Addr[:], this.AsIpv4())
}

// populate provided sockaddr based on this
func (this *Address) ToSockaddr6(sa *syscall.SockaddrInet6) {
	sa.Port = int(this.Port())
	copy(sa.Addr[:], this.AsIp())
}

// allocate and populate a sockaddr based on this
func (this *Address) AsSockaddr() (rv syscall.Sockaddr) {
	if this.IsIpv4() || !this.IsIpSet() {
		sa := &syscall.SockaddrInet4{Port: int(this.Port())}
		copy(sa.Addr[:], this.AsIpv4())
		rv = sa
	} else {
		sa := &syscall.SockaddrInet6{Port: int(this.Port())}
		copy(sa.Addr[:], this.AsIp())
		rv = sa
	}
	return
}

// Pack addr into provided space, returning name and namelen that point into
// space.  space must be on heap.
//
// Use for syscall.Msghdr (sendmsg/recvmsg)
func (this *Address) AsNameBytes(space []byte) (name *byte, namelen uint32) {

	name = &space[0]
	if this.IsIpv4() || !this.IsIpSet() {
		if len(space) < syscall.SizeofSockaddrInet4 {
			panic("not enough space for IPv4 sockaddr")
		}
		rsa := (*syscall.RawSockaddrInet4)(unsafe.Pointer(name))
		namelen = syscall.SizeofSockaddrInet4
		rsa.Family = syscall.AF_INET
		rsa.Port = Htons(this.Port())
		copy(rsa.Addr[:], this.AsIpv4())
	} else {
		if len(space) < syscall.SizeofSockaddrInet6 {
			panic("not enough space for IPv6 sockaddr")
		}
		rsa := (*syscall.RawSockaddrInet6)(unsafe.Pointer(name))
		namelen = syscall.SizeofSockaddrInet6
		rsa.Family = syscall.AF_INET6
		rsa.Port = Htons(this.Port())
		copy(rsa.Addr[:], this.AsIp())
	}
	return
}

// return an allocated copy of the ip
func (this *Address) Ip() (rv net.IP) {
	rv = make(net.IP, 16)
	copy(rv, this.AsIp())
	return
}

// implement net.Addr
func (this Address) Network() string { return "" }

// implement net.Addr
func (this Address) String() string {
	return this.AsIp().String() + ":" + strconv.Itoa(int(this.plus&portMask_))
}

func (this *Address) SetPort(port uint16) {
	this.plus = (this.plus & (^portMask_)) | portBit_ | uint64(port)
}

func (this *Address) SetAddrFrom(that Address) {
	this.addr1 = that.addr1
	this.addr2 = that.addr2
	this.plus |= that.plus & addrBit_
}

// try to parse hostOrIp, and then try to lookup hostOrIp if that fails.
// so, if hostOrIp is an IP addr, no lookup will occur
func (this *Address) ResolveIp(hostOrIp string) (err error) {
	ip, err := ResolveIp(hostOrIp)
	if nil == err {
		this.SetIp(ip)
	}
	return
}

func (this *Address) SetIp(ip net.IP) {
	this.plus |= addrBit_
	if 4 == len(ip) { // ipv4: store as ipv4-in-ipv6
		this.addr1 = 0
		this.addr2 = (0xffff << 16) |
			(uint64(ip[3]) << 56) | (uint64(ip[2]) << 48) |
			(uint64(ip[1]) << 40) | (uint64(ip[0]) << 32)
	} else { // ipv6 or ipv4-in-ipv6
		if len(ip) != 16 {
			panic("ip must be either 4 or 16 in length!")
		}
		to := (*[16]byte)(unsafe.Pointer(this))[:16:16]
		copy(to, ip)
	}
}

func (this *Address) FromIpAndPort(ip net.IP, port uint16) {
	this.SetPort(port)
	this.SetIp(ip)
}

func (this *Address) FromHostPort(hostOrIp string, port uint16) (err error) {
	ip, err := ResolveIp(hostOrIp)
	if nil == err {
		this.SetIp(ip)
		this.SetPort(port)
	}
	return
}

/*
func (this *Address) FromCmsghdr(cmsgB []byte) (err error) {
	space := [16]byte{}
	ip, err := CmsghdrAsIp(cmsgB, net.IP(space[:]))
	if err != nil {
		return
	}
	this.SetIp(ip)
	return
}
*/

// see syscall.Msghdr (Name and Namelen fields)
func (this *Address) FromNameBytes(name *byte, namelen uint32) {
	if syscall.SizeofSockaddrInet4 == namelen { // ipv4: store as ipv4-in-ipv6
		actual := (*syscall.RawSockaddrInet4)(unsafe.Pointer(name))
		//if actual.Family != syscall.AF_INET {
		//	panic("family not set to IPv4")
		//}
		this.addr1 = 0
		this.addr2 = (0xffff << 16) |
			(uint64(actual.Addr[3]) << 56) | (uint64(actual.Addr[2]) << 48) |
			(uint64(actual.Addr[1]) << 40) | (uint64(actual.Addr[0]) << 32)
		this.plus |= addrBit_
		this.SetPort(Htons(actual.Port))

	} else if syscall.SizeofSockaddrInet6 == namelen { // ipv6
		actual := (*syscall.RawSockaddrInet6)(unsafe.Pointer(name))
		//if actual.Family != syscall.AF_INET6 {
		//	panic("family not set to IPv6")
		//}
		to := (*[16]byte)(unsafe.Pointer(this))[:16:16]
		copy(to, actual.Addr[:])
		this.plus |= addrBit_
		this.SetPort(Htons(actual.Port))

	} else {
		slice := (*[1024]byte)(unsafe.Pointer(name))[0:namelen:namelen]
		panic("should not happen - not ipv4 nor ipv6 addr: " + hex.Dump(slice))
	}
}

func IsSockaddrValid(sa syscall.Sockaddr) bool {
	switch actual := sa.(type) {
	case *syscall.SockaddrInet4:
		return nil != actual
	case *syscall.SockaddrInet6:
		return nil != actual
	}
	return false
}

func IsSockaddrZero(sa syscall.Sockaddr) bool {
	switch actual := sa.(type) {
	case *syscall.SockaddrInet4:
		return nil != actual && 0 == actual.Port &&
			0 == actual.Addr[3] && 0 == actual.Addr[2] &&
			0 == actual.Addr[1] && 0 == actual.Addr[0]
	case *syscall.SockaddrInet6:
		return nil != actual && 0 == actual.Port &&
			0 == actual.Addr[15] && 0 == actual.Addr[14] &&
			0 == actual.Addr[13] && 0 == actual.Addr[12] &&
			0 == actual.Addr[11] && 0 == actual.Addr[10] &&
			0 == actual.Addr[9] && 0 == actual.Addr[8] &&
			0 == actual.Addr[7] && 0 == actual.Addr[6] &&
			0 == actual.Addr[5] && 0 == actual.Addr[4] &&
			0 == actual.Addr[3] && 0 == actual.Addr[2] &&
			0 == actual.Addr[1] && 0 == actual.Addr[0]
	}
	return false
}

func IsSockaddrPortAndIpNotZero(sa syscall.Sockaddr) bool {
	switch actual := sa.(type) {
	case *syscall.SockaddrInet4:
		return nil != actual && 0 != actual.Port &&
			(0 != actual.Addr[3] || 0 != actual.Addr[2] ||
				0 != actual.Addr[1] || 0 != actual.Addr[0])
	case *syscall.SockaddrInet6:
		return nil != actual && 0 != actual.Port &&
			(0 != actual.Addr[15] || 0 != actual.Addr[14] ||
				0 != actual.Addr[13] || 0 != actual.Addr[12] ||
				0 != actual.Addr[11] || 0 != actual.Addr[10] ||
				0 != actual.Addr[9] || 0 != actual.Addr[8] ||
				0 != actual.Addr[7] || 0 != actual.Addr[6] ||
				0 != actual.Addr[5] || 0 != actual.Addr[4] ||
				0 != actual.Addr[3] || 0 != actual.Addr[2] ||
				0 != actual.Addr[1] || 0 != actual.Addr[0])
	}
	return false
}

func (this *Address) FromSockaddr(sa syscall.Sockaddr) {
	switch actual := sa.(type) {
	case *syscall.SockaddrInet4:
		if nil == actual {
			this.Clear()
		} else {
			this.SetIp(net.IP(actual.Addr[:]))
			this.SetPort(uint16(actual.Port))
		}
	case *syscall.SockaddrInet6:
		if nil == actual {
			this.Clear()
		} else {
			this.SetIp(net.IP(actual.Addr[:]))
			this.SetPort(uint16(actual.Port))
		}
	default:
		this.Clear()
	}
}
