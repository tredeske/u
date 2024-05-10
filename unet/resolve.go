package unet

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"syscall"
	"time"
	"unsafe"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"
)

// resolve host to an ipv4 or ipv6 addr
func ResolveIp(hostOrIp string, timeout ...time.Duration) (rv net.IP, err error) {
	rv = net.ParseIP(hostOrIp)
	if nil == rv {
		var ips []net.IP
		ips, err = ResolveIps(hostOrIp, timeout...)
		if err != nil {
			return
		}
		rv = ips[0]
	}
	return
}

func ResolveIps(hostOrIp string, timeout ...time.Duration) (rv []net.IP, err error) {
	ip := net.ParseIP(hostOrIp)
	if nil != ip {
		return []net.IP{ip}, nil
	}
	til := 7 * time.Second
	if 0 != len(timeout) {
		til = timeout[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), til)

	rv, err = net.DefaultResolver.LookupIP(ctx, "ip", hostOrIp)
	cancel()
	if err != nil {
		return
	} else if 0 == len(rv) {
		err = fmt.Errorf("No addresses found for host %s", hostOrIp)
	}
	return
}

func ResolveAddrs(
	hostOrIp string,
	timeout ...time.Duration,
) (
	rv []Address,
	err error,
) {
	ip := net.ParseIP(hostOrIp)
	if nil != ip {
		rv = make([]Address, 1)
		rv[0].SetIp(ip)
		return // success
	}

	til := 7 * time.Second
	if 0 != len(timeout) {
		til = timeout[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), til)

	ips, err := net.DefaultResolver.LookupIP(ctx, "ip", hostOrIp)
	cancel()
	if err != nil {
		return
	} else if 0 == len(ips) {
		err = fmt.Errorf("No addresses found for host %s", hostOrIp)
		return
	}
	rv = make([]Address, len(ips))
	for i, ip := range ips {
		rv[i].SetIp(ip)
	}
	return
}

func ResolveNames(ip net.IP, timeout ...time.Duration) (rv []string, err error) {
	til := 7 * time.Second
	if 0 != len(timeout) {
		til = timeout[0]
	}
	ctx, cancel := context.WithTimeout(context.Background(), til)

	rv, err = net.DefaultResolver.LookupAddr(ctx, ip.String())
	cancel()
	if err != nil {
		return
	}
	return
}

func ResolveTcp(
	hostOrIp string,
	port int,
	timeout ...time.Duration,
) (
	rv *net.TCPAddr,
	err error,
) {
	rv = &net.TCPAddr{
		Port: port,
	}
	rv.IP, err = ResolveIp(hostOrIp, timeout...)
	return
}

// if hostOrIp is not an ip addr and is not resolvable, then error
//
// conforms to uconfig.Validator
func ValidIp(hostOrIp string) (err error) {
	if 0 == len(hostOrIp) {
		err = errors.New("no host or ip provided")
		return
	}
	_, err = ResolveIp(hostOrIp)
	return
}

// resolve host/port to a Sockaddr for ipv4 or ipv6
func ResolveSockaddr(
	host string,
	port int,
	timeout ...time.Duration,
) (
	rv syscall.Sockaddr,
	err error,
) {

	ipAddr, err := ResolveIp(host, timeout...)
	if err != nil {
		err = uerr.Chainf(err, "resolving %s", host)
		return
	}
	rv = AsSockaddr(ipAddr, port)
	return
}

// set ip/port to a Sockaddr for ipv4 or ipv6
func AsSockaddr(ip net.IP, port int) (rv syscall.Sockaddr) {
	ip4Addr := ip.To4() // is non-nil for ip4, nil for ip6
	if nil != ip4Addr {
		ip4 := &syscall.SockaddrInet4{
			Port: port,
		}
		copy(ip4.Addr[:], ip4Addr)
		rv = ip4
	} else {
		ip6 := &syscall.SockaddrInet6{
			Port: port,
		}
		copy(ip6.Addr[:], ip)
		rv = ip6
	}
	return
}

// get socket family of sockaddr
func SockaddrFamily(sa syscall.Sockaddr) (rv int, err error) {
	switch sa.(type) {
	case *syscall.SockaddrInet4:
		rv = syscall.AF_INET
	case *syscall.SockaddrInet6:
		rv = syscall.AF_INET6
	default:
		rv = -1
		err = errors.New("Invalid/unknown sockaddr type")
	}
	return
}

// convert uint16 from one byte order to the other
func Htons(v uint16) (rv uint16) {
	return (v >> 8) | (v << 8)
}

// convert uint32 from one byte order to the other
func Htonl(v uint32) (rv uint32) {
	return (v >> 24) | (v << 24) | ((v >> 8) & 0xff00) | ((v << 8) & 0xff0000)
}

// Suitable for creating Name and Namelen of syscall.Msghdr
//
// space is necessary to reserve space for the sockaddr type.  you must
// ensure that space is allocated on the heap and is sized appropriately
// (syscall.SizeofSockaddrInet6)
func RawSockaddrAsNameBytes(
	sa syscall.Sockaddr,
	//space *syscall.RawSockaddrAny,
	space []byte,
) (
	name *byte,
	namelen uint32,
	err error,
) {
	if len(space) < syscall.SizeofSockaddrInet6 {
		panic("space for raw sockaddr too small")
	}

	//
	// this is strange, but how you have to do it.
	// we get a pointer to the space,
	// then cast it to a pointer to a larger byte array,
	// then get a slice of (size:cap) of that.
	//
	//const SZ = syscall.SizeofSockaddrAny
	//slice := (*[SZ]byte)(unsafe.Pointer(space))[:SZ:SZ]
	//name = &slice[0]
	name = &space[0]

	//
	// the family and addr are already in network byte order, but the port
	// is not
	//
	switch actual := sa.(type) {
	case *syscall.SockaddrInet4:
		rsa := (*syscall.RawSockaddrInet4)(unsafe.Pointer(&space[0]))
		namelen = syscall.SizeofSockaddrInet4
		rsa.Family = syscall.AF_INET
		rsa.Port = Htons(uint16(actual.Port))
		copy(rsa.Addr[:], actual.Addr[:])

	case *syscall.SockaddrInet6:
		rsa := (*syscall.RawSockaddrInet6)(unsafe.Pointer(&space[0]))
		namelen = syscall.SizeofSockaddrInet6
		rsa.Family = syscall.AF_INET6
		rsa.Port = Htons(uint16(actual.Port))
		copy(rsa.Addr[:], actual.Addr[:])

		ulog.TODO("IPv6 Flowinfo and Scope_id")

	default:
		name = nil
		err = errors.New("Invalid/unknown sockaddr type (not ipv4 or ipv6)")
	}
	return
}

func SockaddrIP(sa syscall.Sockaddr) net.IP {
	switch actual := sa.(type) {
	case *syscall.SockaddrInet4:
		return net.IP(actual.Addr[:])
	case *syscall.SockaddrInet6:
		return net.IP(actual.Addr[:])
	}
	panic("should not happen - unknown sockaddr type")
}

func SockaddrPort(sa syscall.Sockaddr) int {
	switch actual := sa.(type) {
	case *syscall.SockaddrInet4:
		return actual.Port
	case *syscall.SockaddrInet6:
		return actual.Port
	}
	panic("should not happen - unknown sockaddr type")
}

func SockaddrHostPort(sa syscall.Sockaddr) string {
	switch actual := sa.(type) {
	case *syscall.SockaddrInet4:
		return fmt.Sprintf("%s:%d", net.IP(actual.Addr[:]), actual.Port)
	case *syscall.SockaddrInet6:
		return fmt.Sprintf("[%s]:%d", net.IP(actual.Addr[:]), actual.Port)
	}
	panic("should not happen - unknown sockaddr type")
}

// Suitable for decoding Name and Namelen of syscall.Msghdr
func NameBytesAsString(name *byte, namelen uint32) (rv string) {
	if nil == name || 0 == namelen {
		rv = fmt.Sprintf("nil sockaddr: %p, %d", name, namelen)
	} else if syscall.SizeofSockaddrInet4 == namelen {
		actual := (*syscall.RawSockaddrInet4)(unsafe.Pointer(name))
		rv = fmt.Sprintf("%s:%d", net.IP(actual.Addr[:]), Htons(actual.Port))
	} else if syscall.SizeofSockaddrInet6 == namelen {
		actual := (*syscall.RawSockaddrInet6)(unsafe.Pointer(name))
		rv = fmt.Sprintf("[%s]:%d", net.IP(actual.Addr[:]), Htons(actual.Port))
	} else {
		const SZ = syscall.SizeofSockaddrAny
		slice := (*[SZ]byte)(unsafe.Pointer(name))[:namelen:namelen]
		rv = fmt.Sprintf("unknown sockaddr: %#v", slice)
	}
	return
}

// Suitable for decoding Name and Namelen of syscall.Msghdr
//
// NOTE: ip is set to the internal bytes of the name buffer
func NameBytesAsIpAndPort(name *byte, namelen uint32, ip *net.IP) (port int) {
	family := uint16(*name)
	if syscall.AF_INET == family {
		if syscall.SizeofSockaddrInet4 > namelen {
			panic("expecting sockaddr for ipv4, but it is too small")
		}
		actual := (*syscall.RawSockaddrInet4)(unsafe.Pointer(name))
		port = int(Htons(actual.Port))
		*ip = actual.Addr[:]
	} else if syscall.AF_INET6 == family {
		if syscall.SizeofSockaddrInet6 > namelen {
			panic("expecting sockaddr for ipv6, but it is too small")
		}
		actual := (*syscall.RawSockaddrInet6)(unsafe.Pointer(name))
		port = int(Htons(actual.Port))
		*ip = actual.Addr[:]
	} else {
		slice := (*[1024]byte)(unsafe.Pointer(name))[0:namelen:namelen]
		panic("should not happen - not ipv4 nor ipv6 addr: " + hex.Dump(slice))
	}
	return
}

// Suitable for decoding Name and Namelen of syscall.Msghdr
func NameBytesAsSockaddr(name *byte, namelen uint32) (rv syscall.Sockaddr, err error) {
	if nil == name || 0 == namelen {
		err = fmt.Errorf("nil sockaddr: %p, %d", name, namelen)
	} else if syscall.SizeofSockaddrInet4 == namelen {
		actual := (*syscall.RawSockaddrInet4)(unsafe.Pointer(name))
		ip4 := syscall.SockaddrInet4{
			Port: int(Htons(actual.Port)),
		}
		copy(ip4.Addr[:], actual.Addr[:])
		rv = &ip4
	} else if syscall.SizeofSockaddrInet6 == namelen {
		actual := (*syscall.RawSockaddrInet6)(unsafe.Pointer(name))
		ip6 := syscall.SockaddrInet6{
			Port: int(Htons(actual.Port)),
		}
		copy(ip6.Addr[:], actual.Addr[:])
		rv = &ip6
	} else {
		const SZ = syscall.SizeofSockaddrAny
		slice := (*[SZ]byte)(unsafe.Pointer(name))[:namelen:namelen]
		err = fmt.Errorf("unknown sockaddr: %#v", slice)
	}
	return
}
