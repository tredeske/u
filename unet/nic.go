package unet

import (
	"fmt"
	"net"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"
)

// get a list of all IP addresses of all network interfaces
func AllLocalIps() (rv []net.IP, err error) {
	return FindLocalIps(nil, nil)
}

// use with FindLocalIps
func AllowIp4(ip net.IP) bool {
	return nil != ip.To4()
}

// use with FindLocalIps
func AllowIp6(ip net.IP) bool {
	return nil != ip.To16()
}

// only allow normal global unicast ips.  so, no loopback.
//
// use with FindLocalIps
func AllowUnicastIps(ip net.IP) bool {
	return ip.IsGlobalUnicast()
}

// deny loopback ips
//
// use with FindLocalIps
func DenyLoopbackIps(ip net.IP) bool {
	return !ip.IsLoopback()
}

// deny private ips, such as 10., 192.168., 172
//
// use with FindLocalIps
func DenyPrivateIps(ip net.IP) bool {
	return !ip.IsPrivate()
}

// deny multicast ips
//
// use with FindLocalIps
func DenyMulticastIps(ip net.IP) bool {
	return !ip.IsMulticast()
}

// construct a multi-step filter
//
// use with FindLocalIps
func FilterIps(filter ...func(net.IP) bool) func(net.IP) bool {
	return func(ip net.IP) bool {
		for _, f := range filter {
			if !f(ip) {
				return false
			}
		}
		return true
	}
}

// get a list of the filtered IP addresses of the filtered network interfaces
//
// setting either filter to nil means to accept all
func FindLocalIps(
	allowIntfc func(net.Interface) bool,
	allowIp func(net.IP) bool,
) (
	rv []net.IP,
	err error,
) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return
	}
	rv = make([]net.IP, 0, len(interfaces))
	for _, in := range interfaces {
		if 0 == net.FlagUp&in.Flags {
			continue
		}
		if nil != allowIntfc && !allowIntfc(in) {
			continue
		}
		var addrs []net.Addr
		addrs, err = in.Addrs()
		if err != nil {
			return
		} else if 0 == len(addrs) {
			continue
		}
		for i := range addrs {
			addr, ok := addrs[i].(*net.IPNet)
			if !ok {
				ulog.Warnf("Did not get back expected addr type for %s: %T",
					in.Name, addrs[i])
				continue
			}
			ip := addr.IP
			if ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
				//
				// we are only interested in global addresses.
				// in ipv6, every interface gets a link local address by default,
				// and these start with fe80::/10.
				// in ipv4, link local addresses are not common.
				//
				continue
			}
			exists := false
			for _, existing := range rv {
				if ip.Equal(existing) {
					exists = true
					break
				}
			}
			if !exists && (nil == allowIp || allowIp(ip)) {
				rv = append(rv, ip)
			}
		}
	}
	return
}

// create a filter to match specific hosts
func AllowIps(hosts []string) (filter func(net.IP) bool, err error) {
	m := make(map[Address]struct{})
	anyIpv4 := false
	anyIpv6 := false
	for _, host := range hosts {
		var resolved []net.IP
		resolved, err = ResolveIps(host)
		if err != nil {
			err = uerr.Chainf(err, "Unable to resolve IPs for "+host)
			return
		}
		for _, ip := range resolved {
			if ip.IsUnspecified() { // 0.0.0.0 or ::
				if nil != ip.To4() {
					anyIpv4 = true
				} else {
					anyIpv6 = true
				}
			}
			addr := Address{}
			addr.SetIp(ip)
			m[addr] = struct{}{}
		}
	}
	if 0 == len(m) {
		err = fmt.Errorf("Unable to lookup IPs from %#v", hosts)
		return
	}
	filter = func(ip net.IP) bool {
		addr := Address{}
		addr.SetIp(ip)
		if _, found := m[addr]; found {
			return true
		} else if anyIpv4 && nil != ip.To4() {
			return true
		} else if anyIpv6 && nil != ip.To16() {
			return true
		}
		return false
	}
	return
}
