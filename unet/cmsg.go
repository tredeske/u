package unet

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"unsafe"
)

// Get the target (destination) IP from the Cmsghdr.
//
// when recvmsg or recvmmsg used, Control and Controllen from Msghdr has this
//
// See TestCmsghdr
func CmsghdrAsIp(cmsgB []byte, ipB net.IP) (rv net.IP, err error) {
	if syscall.SizeofSockaddrInet6 > len(cmsgB) {
		err = errors.New("Cmsghdr buffer too small")
		return
	}
	//
	// 1st struct is the 16 byte Cmshdr
	//
	cmsg := (*syscall.Cmsghdr)(unsafe.Pointer(&cmsgB[0]))

	if cmsg.Level == syscall.IPPROTO_IP { // IPv4

		if cmsg.Type != syscall.IP_PKTINFO {
			err = fmt.Errorf("Bad ipv4 cmsghdr: %#v, %#v", cmsg, cmsgB)
			return
		}
		//
		// next struct:
		// pktInfo := (*syscall.Inet4Pktinfo)(unsafe.Pointer(&cmsgB[16]))
		//
		if len(ipB) != 4 {
			if cap(ipB) < 4 {
				ipB = make([]byte, 4)
			} else {
				ipB = ipB[:4]
			}
		}
		copy(ipB, cmsgB[24:28])
		rv = ipB

	} else if cmsg.Level == syscall.IPPROTO_IPV6 {

		if 32 > len(cmsgB) {
			err = errors.New("Cmsghdr buffer too small")
			return
		} else if cmsg.Type != syscall.IPV6_PKTINFO {
			err = fmt.Errorf("Bad ipv6 cmsghdr: %#v, %#v", cmsg, cmsgB)
			return
		}
		//
		// next struct:
		// pktInfo := (*syscall.Inet6Pktinfo)(unsafe.Pointer(&cmsgB[16]))
		//
		if len(ipB) != 16 {
			if cap(ipB) < 16 {
				ipB = make([]byte, 16)
			} else {
				ipB = ipB[:16]
			}
		}
		copy(ipB, cmsgB[16:32])
		rv = ipB

	} else {
		err = fmt.Errorf("Cmsg not ipv4 nor ipv6: %#v", cmsg)
	}
	return
}
