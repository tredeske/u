package unet

import (
	"net"
	"syscall"
	"unsafe"

	"github.com/tredeske/u/uerr"
	"golang.org/x/sys/unix"
)

const (
	cmsgHdrLen_         = 16
	sockExtendedErrLen_ = 16
)

// helper to setup msghdr for cmsghdrs before calling recvmsg
func CmsgSetupMsghdr(msghdr *syscall.Msghdr, cmsgbuff []byte) {
	if cmsgHdrLen_ > len(cmsgbuff) {
		panic("cannot even fit even 1 cmsghdr in this buff")
	}
	msghdr.Control = &cmsgbuff[0]
	msghdr.Controllen = uint64(len(cmsgbuff))
}

// a lens to examine Cmsghdr structs from recvmsg
type CmsgLens struct {
	rem       []byte
	cmsgLen   uint64 // current
	cmsgLevel int32  // current
	cmsgType  int32  // current
}

func (this *CmsgLens) Level() int32 { return this.cmsgLevel }
func (this *CmsgLens) Type() int32  { return this.cmsgType }
func (this *CmsgLens) Msg() []byte  { return this.rem[:int(this.cmsgLen)] }

// use to check for first Cmsghdr
//
// analog to CMSG_FIRSTHDR()
func (this *CmsgLens) First(msghdr *syscall.Msghdr) (ok bool) {
	if cmsgHdrLen_ > msghdr.Controllen { // none
		return false
	}
	cmsg := (*syscall.Cmsghdr)(unsafe.Pointer(msghdr.Control))
	this.cmsgLen = cmsg.Len - cmsgHdrLen_
	this.cmsgLevel = cmsg.Level
	this.cmsgType = cmsg.Type
	sz := int(msghdr.Controllen)
	this.rem = (*[1 << 28]byte)(unsafe.Pointer(msghdr.Control))[cmsgHdrLen_:sz:sz]
	return true
}

// use for subsequent Cmsghdrs
//
// analog to CMSG_NXTHDR()
func (this *CmsgLens) Next() (ok bool) {
	this.rem = this.rem[int(this.cmsgLen):]
	if cmsgHdrLen_ > len(this.rem) { // no more left
		this.cmsgLen = 0
		return false
	}
	cmsg := (*syscall.Cmsghdr)(unsafe.Pointer(&this.rem[0]))
	this.cmsgLen = cmsg.Len - cmsgHdrLen_
	this.cmsgLevel = cmsg.Level
	this.cmsgType = cmsg.Type
	this.rem = this.rem[cmsgHdrLen_:]
	return true
}

func (this *CmsgLens) IsIpError() bool {
	return this.cmsgType == syscall.IP_RECVERR ||
		this.cmsgType == syscall.IPV6_RECVERR
}

// if cmsg is extended error, get it
//
// if rv.Errno == syscall.EMSGSIZE, then rv.Info contains MTU to use
//
// see ip(7) of the linux man pages for more info about IP_RECVERR
//
// see /usr/include/linux/errqueue.h
func (this *CmsgLens) IpError() (rv unix.SockExtendedErr, ok bool, err error) {
	const badCmsg = uerr.Const("Cmsghdr too small for sock_extended_err")
	if !this.IsIpError() {
		//(this.cmsgLevel != syscall.IPPROTO_IP &&
		//	this.cmsgLevel != syscall.IPPROTO_IPV6) {
		return
	}
	cmsgB := this.Msg()
	if sockExtendedErrLen_ > len(cmsgB) {
		err = badCmsg
		return
	}
	return *(*unix.SockExtendedErr)(unsafe.Pointer(&cmsgB[0])), true, nil
}

// use after IpError() is successful to get source of error.
//
// refer to SO_EE_OFFENDER() in ip(7) of linux man pages
func (this *CmsgLens) IpErrorOffender(rv *Address) (ok bool) {
	if this.cmsgType != syscall.IP_RECVERR &&
		this.cmsgType != syscall.IPV6_RECVERR {
		return false
	}
	cmsgB := this.Msg()[cmsgHdrLen_:]
	namelen := uint32(syscall.SizeofSockaddrInet4)

	if this.cmsgLevel == syscall.IPPROTO_IPV6 {
		namelen = uint32(syscall.SizeofSockaddrInet6)
	} else if this.cmsgLevel != syscall.IPPROTO_IP {
		return false
	}
	if namelen > uint32(len(cmsgB)) {
		return false
	}
	sa := *(*syscall.RawSockaddrInet4)(unsafe.Pointer(&cmsgB[0]))
	if syscall.AF_UNSPEC == sa.Family {
		return false
	}
	rv.FromNameBytes(&cmsgB[0], namelen)
	return true
}

// Get the target (destination) IP from the Cmsghdr.
//
// When recvmsg or recvmmsg used, Control and Controllen from Msghdr has this.
//
// See TestCmsghdr.
//
// See ip(7) of the linux man pages for more info about IP_PKTINFO.
func (this *CmsgLens) PktInfo(ipB net.IP) (rv net.IP, ok bool, err error) {

	const errTooSmall = uerr.Const("Cmsghdr buffer too small")
	cmsgB := this.Msg()
	if this.cmsgLevel == syscall.IPPROTO_IP { // IPv4

		if this.cmsgType != syscall.IP_PKTINFO {
			return
		} else if 12 > len(cmsgB) {
			err = errTooSmall
			return
		}
		if len(ipB) != 4 {
			if cap(ipB) < 4 {
				ipB = make([]byte, 4)
			} else {
				ipB = ipB[:4]
			}
		}
		copy(ipB, cmsgB[8:12])
		rv = ipB
		ok = true

	} else if this.cmsgLevel == syscall.IPPROTO_IPV6 {

		if this.cmsgType != syscall.IPV6_PKTINFO {
			return
		} else if 16 > len(cmsgB) {
			err = errTooSmall
			return
		}
		if len(ipB) != 16 {
			if cap(ipB) < 16 {
				ipB = make([]byte, 16)
			} else {
				ipB = ipB[:16]
			}
		}
		copy(ipB, cmsgB[:16])
		rv = ipB
		ok = true
	}
	return
}
