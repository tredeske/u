package uconfig

import (
	"net"
	"strconv"
	"strings"

	"github.com/tredeske/u/uerr"
)

// Ensure addr is correctly filled in with default host/port if missing.
//
// addr may be any of these:
//
//	HOST:PORT
//	""
//	:
//	HOST
//	HOST:
//	:PORT
//	[HOST]:PORT
//	[IPv6]:PORT
//
// HOST may be an IP (v4 or v6), hostname, or fqdn.
//
// The square brackets can only be used when PORT is also provided.  And they must
// be used when HOST is an IPv6 address.
func EnsureAddr(defaultHost, defaultPort, addr string) (rv string, err error) {
	const (
		errHost = uerr.Const("host is not a valid hostname or IP")
		errPort = uerr.Const("port out of range or not a number")
	)
	if 0 == len(addr) || ":" == addr {
		rv = defaultHost + ":" + defaultPort
		return
	} else if ip := net.ParseIP(addr); nil != ip { // it's just an IP
		if nil != ip.To4() { // it's an ipv4
			rv = addr + ":" + defaultPort
		} else if '[' == addr[0] { // [ipv6]
			rv = addr + ":" + defaultPort
		} else { // naked ipv6
			rv = "[" + addr + "]:" + defaultPort
		}
		return
	}
	rv = addr

	//
	// SplitHostPort will consider any of these valid:
	// host:
	// :port
	// host:port
	// [host]:port
	// [host]:
	//
	h, p, err := net.SplitHostPort(addr)
	if err != nil { // could be no ':' or ':port'
		if -1 == strings.IndexByte(addr, ':') {
			rv += ":" + defaultPort
		} else {
			return
		}
		h, p, err = net.SplitHostPort(rv)
		if err != nil {
			return
		}
	}
	if 0 == len(h) {
		h = defaultHost
		rv = defaultHost + rv
	}
	if nil == net.ParseIP(h) && !ValidHostname(h) {
		err = errHost
		return
	}
	if 0 == len(p) {
		p = defaultPort
		rv += defaultPort
	}
	//
	// SplitHostPort does not validate that port is a number or in valid range
	//
	port, err := strconv.Atoi(p)
	if err != nil || 0 > port || 65535 < port {
		err = errPort
		return
	}
	return
}
