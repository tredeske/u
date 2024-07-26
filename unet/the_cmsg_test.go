package unet

import (
	"errors"
	"fmt"
	"net"
	"syscall"
	"testing"
	"unsafe"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"
	"golang.org/x/sys/unix"
)

/*
doing this:

ip route add unreachable 192.0.2.1

adds our invalid ipv4 addr as unreachable.

This causes sendmsg to return host unreachable error, probably because the OS
knows the dst is unreachable w/o sending the pkt.

what we need is for the pkt to actually be sent, resulting in an icmp back,
which will be detected in cmsg from recvmsg

TODO: figure out a decent way to simulate this, perhaps with podman-compose.

	func TestCmsgIpError(t *testing.T) {
		dstPort := 65535
		for _, testcase := range []struct {
			dstHost  string
			bindHost string
		}{
			{
				dstHost:  InvalidIpv4Str,
				bindHost: "0.0.0.0",
			}, {
				dstHost:  InvalidIpv6Str,
				bindHost: "::",
			},
		} {
			dstHost := testcase.dstHost
			var dstAddr Address
			dstAddr.ResolveIp(dstHost)
			dstAddr.SetPort(uint16(dstPort))

			var fd int
			sock := Socket{}
			err := sock.
				ResolveNearAddr(testcase.bindHost, 0).
				ResolveFarAddr(dstHost, dstPort).
				ConstructUdp().
				SetOptReusePort().
				Bind().
				//Connect().
				GiveMeTheFreakingFd(&fd).
				Error
			defer sock.Close()
			if err != nil {
				t.Fatalf("Unable to create socket: %s", err)
			}

			nameB := make([]byte, syscall.SizeofSockaddrAny)
			dataB := [64]byte{}

			iov := syscall.Iovec{
				Base: &dataB[0],
				Len:  uint64(len(dataB)),
			}
			msghdr := syscall.Msghdr{
				Iov:    &iov,
				Iovlen: 1,
			}
			msghdr.Name, msghdr.Namelen = dstAddr.AsNameBytes(nameB)

			msg := uintptr(unsafe.Pointer(&msghdr))

			nsent, errno := SendMsg(uintptr(fd), msg, 0)
			if 0 != errno {
				t.Fatalf("syscall sendmsg %d, %s", errno, errno)
			} else if 0 >= nsent {
				t.Fatalf("got back rv=%d from SYS_SENDMSG", nsent)
			}

			cmsgB := [1024]byte{}
			//var nread int
			//nread, errno = RecvMsg(uintptr(fd), msg, syscall.MSG_ERRQUEUE)
			times := 0
			msghdr.Name = &make([]byte, syscall.SizeofSockaddrAny)[0]
			msghdr.Namelen = syscall.SizeofSockaddrAny
		again:
			iov.Base = &dataB[0]
			iov.Len = uint64(len(dataB))
			msghdr.Iov = &iov
			msghdr.Iovlen = 1
			msghdr.Control = &cmsgB[0]
			msghdr.Controllen = uint64(len(cmsgB))
			_, errno = RecvMsg(uintptr(fd), msg, syscall.MSG_ERRQUEUE)
			if 0 != errno {
				if syscall.EAGAIN == errno {
					times++
					if times > 25 {
						t.Fatalf("%s: too many EAGAINs", dstHost)
					}
					time.Sleep(100 * time.Millisecond)
					goto again
				}
				t.Fatalf("%s: syscall recvmsg %d, %s", dstHost, errno, errno)
				//} else if 0 >= nsent {
				//	t.Fatalf("got back rv=%d from SYS_SENDMSG", nsent)
			}
			if 0 == msghdr.Controllen {
				t.Fatalf("%s: did not get back cmsghdr.  got %#v", dstHost, msghdr)
			}

		}
	}
*/
func TestCmsgMtuDisco(t *testing.T) {

	for i, testcase := range []struct {
		host string
		bind bool
	}{
		{
			host: InvalidIpv4Str,
			bind: false,
		}, {
			host: "0.0.0.0",
			bind: true,
		}, {
			host: InvalidIpv6Str,
			bind: false,
		}, {
			host: "::",
			bind: true,
		},
	} {
		s, err := NewSocket().
			ResolveNearAddr(testcase.host, 0, !testcase.bind).
			ResolveFarAddr(testcase.host, 65535, testcase.bind).
			ConstructUdp().
			SetOptMtuDiscover(MtuDiscoProbe).
			Done()
		s.Close()
		if err != nil {
			t.Fatalf("%d: unable to construct ipv4 socket with disco: %s", i, err)
		}
	}
}

func TestCmsgPktInfo(t *testing.T) {

	dstPort := 33557

	var dstIp net.IP

	for _, testcase := range []struct {
		dstHost    string
		listenHost string
	}{
		{
			dstHost:    "127.0.0.1",
			listenHost: "0.0.0.0",
		}, {
			dstHost:    "::1",
			listenHost: "::",
		},
	} {
		dst := Socket{}
		err := dst.
			ResolveNearAddr(testcase.listenHost, dstPort).
			ConstructUdp().
			SetOptReusePort().
			SetOptRecvPktInfo(). // for cmsghdr
			Bind().
			Error
		if err != nil {
			t.Fatalf("Unable to bind: %s", err)
		}

		src := Socket{}
		err = src.
			ResolveFarAddr(testcase.dstHost, dstPort).
			ConstructUdp().
			SetOptReusePort().
			Connect().
			GetSockName().
			Error
		if err != nil {
			t.Fatalf("Unable to connect: %s", err)
		}
		expectIp := SockaddrIP(src.FarAddr)
		isIPv4 := nil != expectIp.To4()
		expectSrcIp := SockaddrIP(src.NearAddr)
		expectSrcPort := SockaddrPort(src.NearAddr)

		sfd, ok := src.Fd.Get()
		if !ok {
			t.Fatalf("Unable to get valid fd")
		}
		err = send(sfd, t.Name())
		if err != nil {
			t.Fatalf("Unable to send: %s", err)
		}

		//
		// now receive it and decode cmsghdr pktinfo
		//

		dfd, ok := dst.Fd.Get()
		if !ok {
			t.Fatalf("unable to get valid dst fd")
		}
		data, srcPort, srcIp, dstIp, srcAddr, err := recvWithPktinfo(dfd, dstIp)
		if err != nil {
			t.Fatalf("Unable to recv: %s", err)
		} else if data != t.Name() {
			t.Fatalf("Got back '%s' %d instead of '%s'", data, len(data), t.Name())
		} else if !expectIp.Equal(dstIp) {
			t.Fatalf("dstIp (%s) does not match src (%s)", dstIp, expectIp)
		} else if isIPv4 && nil == dstIp.To4() {
			t.Fatalf("dst address should be ipv4")
		} else if !isIPv4 && nil != dstIp.To4() {
			t.Fatalf("dst address should be ipv6")
		} else if expectSrcPort != srcPort {
			t.Fatalf("src port should be %d but is %d", expectSrcPort, srcPort)
		} else if !expectSrcIp.Equal(srcIp) {
			t.Fatalf("src ip should be %s but is %s", expectSrcIp, srcIp)

		} else if uint16(expectSrcPort) != srcAddr.Port() {
			t.Fatalf("srcAddr port should be %d but is %d", expectSrcPort,
				srcAddr.Port())
		} else if !expectSrcIp.Equal(srcAddr.AsIp()) {
			t.Fatalf("srcAddr ip should be %s but is %s / %#v", expectSrcIp,
				srcAddr.AsIp(), srcAddr)
		}
		ulog.Printf("dstIp: %s", dstIp)

		src.Close()
		dst.Close()
	}
}

func send(fd int, data string) (err error) {

	dataB := []byte(data)
	iov := syscall.Iovec{
		Base: &dataB[0],
		Len:  uint64(len(dataB)),
	}
	hdr := syscall.Msghdr{
		Iov:    &iov,
		Iovlen: 1,
	}

	nsent, errno := SendMsg(uintptr(fd), uintptr(unsafe.Pointer(&hdr)), 0)
	if 0 != errno {
		err = uerr.ChainfCode(errno, int(errno), "syscall sendmmsg (%#v)",
			errno)
	} else if 0 >= nsent {
		err = fmt.Errorf("got back rv=%d from SYS_SENDMSG", nsent)
	}
	return
}

func recvWithPktinfo(
	fd int, dstIp net.IP,
) (
	data string, srcPort int, src, to net.IP, srcAddr Address, err error,
) {

	dataB := [64]byte{}
	cmsgB := [48]byte{}

	iov := syscall.Iovec{
		Base: &dataB[0],
		Len:  uint64(len(dataB)),
	}
	hdr := syscall.Msghdr{
		Name:       &make([]byte, syscall.SizeofSockaddrAny)[0],
		Namelen:    syscall.SizeofSockaddrAny,
		Iov:        &iov,
		Iovlen:     1,
		Control:    &cmsgB[0],
		Controllen: uint64(len(cmsgB)),
	}

	nread, errno := RecvMsg(uintptr(fd), uintptr(unsafe.Pointer(&hdr)), 0)
	if 0 != errno {
		err = uerr.ChainfCode(errno, int(errno), "syscall recvmsg (%#v)", errno)
		return
	} else if 0 >= nread {
		err = fmt.Errorf("got back rv=%d from SYS_RECVMSG", nread)
		return
	} else if 0 != hdr.Flags {
		err = fmt.Errorf("hdr.Flags not zero: %x", hdr.Flags)
		return
	}
	srcPort = NameBytesAsIpAndPort(hdr.Name, hdr.Namelen, &src)
	srcAddr.FromNameBytes(hdr.Name, hdr.Namelen)
	data = string(dataB[:nread])

	if cmsgHdrLen_ != unsafe.Sizeof(syscall.Cmsghdr{}) {
		panic("incorrect cmsghdr length")
	} else if sockExtendedErrLen_ != unsafe.Sizeof(unix.SockExtendedErr{}) {
		panic("incorrect sock_extended_err length")
	}

	lens := CmsgLens{}
	if !lens.First(&hdr) {
		err = errors.New("No cmsghdr detected")
		return
	}
	var ok bool
	var ipErr unix.SockExtendedErr
	ipErr, ok, err = lens.IpError()
	if err != nil {
		return
	} else if ok {
		err = fmt.Errorf("should not have got an IP error: %#v", ipErr)
		return
	}

	to, ok, err = lens.PktInfo(dstIp)
	if err != nil {
		return
	} else if !ok {
		err = errors.New("Didn't get a pktinfo cmsg")
		return
	}

	var addr Address
	ok, err = addr.FromCmsgHdr(&lens)
	if err != nil {
		return
	} else if !ok {
		err = errors.New("Didn't get a pktinfo cmsg")
		return
	} else if !to.Equal(addr.AsIp()) {
		err = fmt.Errorf("%s != %s", to, addr.AsIp())
		return
	}

	if lens.Next() {
		err = errors.New("Should not be a next cmsghdr")
		return
	}
	return
}
