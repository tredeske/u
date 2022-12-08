package unet

import (
	"fmt"
	"net"
	"syscall"
	"testing"
	"time"
	"unsafe"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"
)

func TestSocketUdp(t *testing.T) {
	sock := Socket{}
	err := sock.Bind().Error
	if nil == err {
		t.Fatalf("Bind without resolving src addr should fail")
	}

	sock = Socket{}
	err = sock.Connect().Error
	if nil == err {
		t.Fatalf("Connect without resolving dst addr should fail")
	}

	sock = Socket{}
	err = sock.
		ResolveNearAddr("localhost", 5000).
		ConstructUdp().
		SetOptRcvBuf(65536).
		Bind().
		Error
	if err != nil {
		t.Fatalf("Should be no error: %s", err)
	}

	sock = Socket{}
	err = sock.
		ResolveFarAddr("localhost", 5000).
		ConstructUdp().
		Connect().
		Error
	if err != nil {
		t.Fatalf("Should be no error: %s", err)
	}
}

func TestSocketTcp(t *testing.T) {

	ulog.Println(`
GIVEN: tcp listener
 WHEN: client connects
  AND: client sends data
 THEN: listener accepts connection
  AND: receiver gets data
  `)

	const (
		host = "127.0.0.1"
		port = 5000
	)
	resultC := make(chan error)
	dataString := "the quick brown fox"
	data := []byte(dataString)
	timeout := 5 * time.Second
	deadline := time.Now().Add(timeout)

	listener := Socket{}
	err := listener.
		ResolveNearAddr(host, port).
		ConstructTcp().
		SetTimeout(timeout).
		SetOptReuseAddr().
		Bind().
		Listen(7).
		Error
	if err != nil {
		t.Fatalf("Should be no error: %s", err)
	}

	sender := Socket{}
	defer sender.Fd.Disable()
	go func() {
		err := sender.
			ResolveFarAddr(host, port).
			ConstructTcp().
			SetTimeout(timeout).
			Connect().
			Error
		if err != nil {
			resultC <- err
			return
		}

		_, err = sender.Write(data)
		if err != nil {
			resultC <- err
			return
		}

		err = sender.Close()
		if err != nil {
			resultC <- err
			return
		}
		close(resultC)
	}()

	receiver := Socket{}
	err = listener.Accept(&receiver)
	if err != nil {
		t.Fatalf("Unable to accept: %s", err)
	}
	receiver.SetDeadline(deadline)

	buff := [512]byte{}
	nread, err := receiver.Read(buff[:])
	if err != nil {
		t.Fatalf("Unable to read: %s", err)
	} else if dataString != string(buff[:nread]) {
		t.Fatalf("Did not get expected data")
	}

	err = receiver.Close()
	if err != nil {
		t.Fatalf("Unable to close: %s", err)
	}

	err = <-resultC
	if err != nil {
		t.Fatalf("Sender failed: %s", err)
	}
}

func TestCmsghdr(t *testing.T) {

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
		// now receive it and decode cmsghdr
		//

		dfd, ok := dst.Fd.Get()
		if !ok {
			t.Fatalf("unable to get valid dst fd")
		}
		data, srcPort, srcIp, dstIp, srcAddr, err := recvWithCmsghdr(dfd, dstIp)
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
	rv1, _, errno := syscall.Syscall6(
		syscall.SYS_SENDMSG,
		uintptr(fd),
		uintptr(unsafe.Pointer(&hdr)),
		0, 0, 0, 0)

	if 0 != errno {
		err = uerr.ChainfCode(errno, int(errno), "syscall sendmmsg (%#v)",
			errno)
	} else if 0 >= int(rv1) {
		err = fmt.Errorf("got back rv=%d from SYS_SENDMSG", int(rv1))
	}
	return
}

func recvWithCmsghdr(
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
	rv1, _, errno := syscall.Syscall6(
		syscall.SYS_RECVMSG,
		uintptr(fd),
		uintptr(unsafe.Pointer(&hdr)),
		0, 0, 0, 0)

	if 0 != errno {
		err = uerr.ChainfCode(errno, int(errno), "syscall recvmsg (%#v)", errno)
		return
	} else if 0 >= int(rv1) {
		err = fmt.Errorf("got back rv=%d from SYS_RECVMSG", int(rv1))
		return
	} else if 0 != hdr.Flags {
		err = fmt.Errorf("hdr.Flags not zero: %x", hdr.Flags)
		return
	}
	srcPort = NameBytesAsIpAndPort(hdr.Name, hdr.Namelen, &src)
	srcAddr.FromNameBytes(hdr.Name, hdr.Namelen)
	data = string(dataB[:rv1])
	to, err = CmsghdrAsIp(cmsgB[:hdr.Controllen], dstIp)
	return
}

func TestDscpTos(t *testing.T) {
	code, err := LookupDscpTos("foo")
	if nil == err {
		t.Fatalf("should fail")
	} else if 0 != code {
		t.Fatalf("should be 0, is %x", code)
	}

	code, err = LookupDscpTos("")
	if err != nil {
		t.Fatalf("should return normal code, but got err: %s", err)
	} else if 0 != code {
		t.Fatalf("should return normal code, but got %d", code)
	}

	code, err = LookupDscpTos("AF11")
	if err != nil {
		t.Fatalf("should return AF11 code, but got err: %s", err)
	} else if 0 == code {
		t.Fatalf("got 0 code back for AF11!")
	}

	for _, s := range []string{
		"AF42", "144", "0x90", "0o220", "0b10010000",
	} {
		code, err = LookupDscpTos(s)
		if err != nil {
			t.Fatalf("should return AF42 code, but got err: %s", err)
		} else if 0x90 != code {
			t.Fatalf("Did not get proper code for AF42, got %d", code)
		}
	}
}
