package unet

import (
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"syscall"
	"unsafe"
)

// a struct that can hold either ipv4 or ipv6 address, plus port
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
	addrBit_  uint64 = 1 << 63
	portMask_ uint64 = 0xffff
)

func (this *Address) Clear() {
	this.addr1 = 0
	this.addr2 = 0
	this.plus = 0
}

func (this Address) IsIpv4() bool {
	// addr1 must be 0, while addr2 must be 0xffff0000 in low 32 bits
	return 0 == this.addr1 && 0xffff0000 == (this.addr2&0xffffffff)
}

func (this Address) IsIpv6() bool    { return !this.IsIpv4() }
func (this Address) IsSet() bool     { return 0 != this.plus }
func (this Address) IsIpSet() bool   { return 0 != (this.plus & addrBit_) }
func (this Address) IsPortSet() bool { return 0 != (this.plus & portMask_) }
func (this Address) Port() uint16    { return uint16(this.plus & portMask_) }

func (this *Address) AsIp() (rv net.IP) {
	return (*[16]byte)(unsafe.Pointer(this))[:16:16]
}

func (this *Address) AsIpV4() (rv net.IP) {
	return this.AsIp()[12:16]
}

func (this *Address) AsSockaddr() (rv syscall.Sockaddr) {
	return AsSockaddr(this.AsIp(), int(this.Port()))
}

// Pack addr into provided space, returning name and namelen that point into
// space.  space must be on heap.
//
// Use for syscall.Msghdr (sendmsg/recvmsg)
func (this *Address) AsNameBytes(space []byte) (name *byte, namelen uint32) {

	var err error
	name, namelen, err = RawSockaddrAsNameBytes(
		AsSockaddr(this.AsIp(), int(this.Port())), space)
	if err != nil {
		panic(fmt.Sprintf("Impossible!  Failed sockaddr conversion: %s", err))
	}
	return
}

// return a copy of the ip
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
	this.plus = (this.plus & (^portMask_)) | uint64(port)
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

func (this *Address) FromCmsghdr(cmsgB []byte) (err error) {
	space := [16]byte{}
	ip, err := CmsghdrAsIp(cmsgB, net.IP(space[:]))
	if err != nil {
		return
	}
	this.SetIp(ip)
	return
}

// see syscall.Msghdr (Name and Namelen fields)
func (this *Address) FromNameBytes(name *byte, namelen uint32) {
	if syscall.SizeofSockaddrInet4 == namelen { // ipv4: store as ipv4-in-ipv6
		if syscall.SizeofSockaddrInet4 > namelen {
			panic("expecting sockaddr for ipv4, but it is too small")
		}
		actual := (*syscall.RawSockaddrInet4)(unsafe.Pointer(name))
		this.addr1 = 0
		this.addr2 = (0xffff << 16) |
			(uint64(actual.Addr[3]) << 56) | (uint64(actual.Addr[2]) << 48) |
			(uint64(actual.Addr[1]) << 40) | (uint64(actual.Addr[0]) << 32)
		//this.plus = uint64(Htons(actual.Port))
		this.plus |= addrBit_
		this.SetPort(Htons(actual.Port))

	} else if syscall.SizeofSockaddrInet6 == namelen { // ipv6
		if syscall.SizeofSockaddrInet6 > namelen {
			panic("expecting sockaddr for ipv6, but it is too small")
		}
		actual := (*syscall.RawSockaddrInet6)(unsafe.Pointer(name))
		to := (*[16]byte)(unsafe.Pointer(this))[:16:16]
		copy(to, actual.Addr[:])
		//this.plus = uint64(Htons(actual.Port))
		this.plus |= addrBit_
		this.SetPort(Htons(actual.Port))

	} else {
		slice := (*[1024]byte)(unsafe.Pointer(name))[0:namelen:namelen]
		panic("should not happen - not ipv4 nor ipv6 addr: " + hex.Dump(slice))
	}
}

func (this *Address) FromSockaddr(sa syscall.Sockaddr) {
	switch actual := sa.(type) {
	case *syscall.SockaddrInet4:
		this.SetIp(net.IP(actual.Addr[:]))
		this.SetPort(uint16(actual.Port))
	case *syscall.SockaddrInet6:
		this.SetIp(net.IP(actual.Addr[:]))
		this.SetPort(uint16(actual.Port))
	default:
		panic("should not happen - unknown sockaddr type")
	}
}
