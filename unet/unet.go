package unet

import (
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// must match layout of C struct mmsghdr
type MMsghdr struct {
	syscall.Msghdr         // what to send
	NTransferred   uint32  // returns number of bytes actually sent/received
	Pad_cgo_2      [4]byte // alignment
}

// prep for sending/receiving data
func (hdr *MMsghdr) Prep(iov *syscall.Iovec, iovlen int) {
	hdr.Iov = iov
	hdr.Iovlen = uint64(iovlen)
	hdr.NTransferred = 0
}

func MMsghdrsAsString(hdrs []MMsghdr) string {
	strBuff := [16]byte{}
	b := strings.Builder{}
	b.Grow(256)
	b.WriteString("[]MMsghdr{ len=")
	b.Write(strconv.AppendInt(strBuff[:], int64(len(hdrs)), 10))
	for i := range hdrs {
		hdr := &hdrs[i]
		b.WriteString("\n\t")
		b.Write(strconv.AppendInt(strBuff[:], int64(i), 10))
		b.WriteString(": iovlen=")
		iovlen := int(hdr.Iovlen)
		b.Write(strconv.AppendInt(strBuff[:], int64(iovlen), 10))
		b.WriteString(", name=")
		b.WriteString(NameBytesAsString(hdr.Name, hdr.Namelen))
		b.WriteString(" {")
		iovs := (*[1 << 16]syscall.Iovec)(unsafe.Pointer(hdr.Iov))[:iovlen:iovlen]
		bytes := 0
		for j, iov := range iovs {
			if 0 != j {
				b.WriteString(", ")
			}
			bytes += int(iov.Len)
			b.Write(strconv.AppendInt(strBuff[:], int64(iov.Len), 10))
		}
		b.WriteString("}, bytes=")
		b.Write(strconv.AppendInt(strBuff[:], int64(bytes), 10))
	}
	b.WriteString("\n}")
	return b.String()
}

// some useful values
const (
	IP_OVERHEAD     = 20
	IP6_OVERHEAD    = 40 // does not include extension headers
	UDP_OVERHEAD    = 8
	IPUDP_OVERHEAD  = IP_OVERHEAD + UDP_OVERHEAD
	IP6UDP_OVERHEAD = IP6_OVERHEAD + UDP_OVERHEAD
	UDP_MAX         = 65535 - IPUDP_OVERHEAD
	UDP_MAX_128     = 65408 // max mult of 128
	JUMBO_MAX_128   = 8960  // max mult of 128 when MTU is 9001
)

// from golang.org/x/sys/unix
const (
	SYS_RECVMMSG uintptr = 299
	SYS_SENDMMSG uintptr = 307
	//MSG_WAITFORONE uintptr = 0x10000
)

// does the error mean that the socket is closed?
//
// this occurs when another goroutine in the same process has closed it
// to tell the guy using it to go away, like when the component is
// being turned off.
//
// in some situations errors.Is(syscall.ECONNRESET) is also useful
func MeansClosed(err error) bool {
	return nil != err && (errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed))
}
