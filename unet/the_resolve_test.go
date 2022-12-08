package unet

import (
	"bytes"
	"reflect"
	"strconv"
	"strings"
	"syscall"
	"testing"

	"github.com/tredeske/u/ulog"
)

var Space_ [32]byte

func TestHtons(t *testing.T) {
	const (
		ORIG    uint16 = 0x0123
		MUST_BE uint16 = 0x2301
	)
	var value uint16 = ORIG
	result := Htons(value)
	if MUST_BE != result {
		t.Fatalf("Htons produced %x instead of %x", result, MUST_BE)
	}
}

func TestResolve(t *testing.T) {

	const PORT = 5000
	const IPV4 = 0
	const IPV6 = 1
	resolved := [2]bool{}

	for _, dat := range []struct {
		host   string
		result []byte
	}{
		{
			host:   "127.0.0.1",
			result: []byte{127, 0, 0, 1},
		},
		{
			host:   "localhost",
			result: []byte{}, // don't care, as could be ipv4 or ipv6
		},
		{
			host:   "::1",
			result: []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1},
		},
	} {

		//
		// resolve
		//
		sa, err := ResolveSockaddr(dat.host, PORT)
		if err != nil {
			t.Fatalf("resolve error: %s", err)
		}

		//
		// round trip convert sa -> raw -> refined
		//
		name, namelen, err := RawSockaddrAsNameBytes(sa, Space_[:])
		if err != nil {
			t.Fatalf("Unable to create RawSockaddr: %s", err)
		} else if nil == name || 0 == namelen {
			t.Fatalf("Nil or empty RawSockaddr")
		}

		asString := NameBytesAsString(name, namelen)
		if !strings.Contains(asString, strconv.Itoa(PORT)) {
			t.Fatalf("Unable to get name as string. got: %s", asString)
		} else if !strings.Contains(asString, strconv.Itoa(SockaddrPort(sa))) {
			t.Fatalf("Unable to get name as string. got: %s", asString)
		} else if !strings.Contains(asString, SockaddrIP(sa).String()) {
			t.Fatalf("Unable to get name as string. got: %s", asString)
		}

		var refined syscall.Sockaddr
		refined, err = NameBytesAsSockaddr(name, namelen)
		if err != nil {
			t.Fatalf("Unable to recreate Sockaddr: %s", err)
		}

		//
		// verify the resolved sockaddr
		//
		switch actual := sa.(type) {

		case *syscall.SockaddrInet4:
			resolved[IPV4] = true
			if PORT != actual.Port {
				t.Fatalf("ipv4 port not set")

			} else if 0 != len(dat.result) &&
				!bytes.Equal(actual.Addr[:], dat.result) {

				t.Fatalf("ipv4 addr mismatch.  %#v should be %#v",
					actual.Addr, dat.result)
			}
			family, err := SockaddrFamily(sa)
			if err != nil {
				t.Fatalf("Unable to get family: %s", err)
			} else if syscall.AF_INET != family {
				t.Fatalf("ipv4 addr did not resolve to AF_INET")
			}

			actualRefined, ok := refined.(*syscall.SockaddrInet4)
			if !ok {
				t.Fatalf("%s should be a SockaddrInet4. is %s",
					asString, reflect.TypeOf(refined))
			} else if !reflect.DeepEqual(sa, actualRefined) {
				t.Fatalf("should be equal: %#v, %#v", sa, actualRefined)
			}

		case *syscall.SockaddrInet6:
			resolved[IPV6] = true
			if PORT != actual.Port {
				t.Fatalf("ipv6 port not set")

			} else if 0 != len(dat.result) &&
				!bytes.Equal(actual.Addr[:], dat.result) {

				t.Fatalf("ipv4 addr mismatch.  %#v should be %#v",
					actual.Addr, dat.result)
			}
			family, err := SockaddrFamily(sa)
			if err != nil {
				t.Fatalf("Unable to get family: %s", err)
			} else if syscall.AF_INET6 != family {
				t.Fatalf("ipv6 addr did not resolve to AF_INET6")
			}

			actualRefined, ok := refined.(*syscall.SockaddrInet6)
			if !ok {
				t.Fatalf("%s should be a SockaddrInet6. is %s",
					asString, reflect.TypeOf(refined))
			} else if !reflect.DeepEqual(sa, actualRefined) {
				t.Fatalf("should be equal: %#v, %#v", sa, actualRefined)
			}

		default:
			t.Fatalf("Unknown sockaddr: %#v", sa)
		}
	}

	if !resolved[IPV4] {
		t.Fatalf("Did not verify IPV4")
	} else if !resolved[IPV6] {
		t.Fatalf("Did not verify IPV6")
	}
}

func TestAllLocalIps(t *testing.T) {

	ips, err := AllLocalIps()
	if err != nil {
		t.Fatalf("Unable to get local IPs: %s", err)
	} else if 0 == len(ips) {
		t.Fatalf("No local IPs found!")
	}
	for i := range ips {
		ulog.Printf("IP: %s, unicast=%t, multicast=%t, private=%t",
			ips[i], ips[i].IsGlobalUnicast(), ips[i].IsMulticast(),
			ips[i].IsPrivate())
	}

	ips, err = FindLocalIps(nil, AllowIp4)
	if err != nil {
		t.Fatalf("Unable to get local IPs: %s", err)
	} else if 0 == len(ips) {
		t.Fatalf("No local IPs found!")
	}
	for i := range ips {
		if nil == ips[i].To4() {
			t.Fatalf("Not an IPv4 addr: %s", ips[i])
		}
		ulog.Printf("IPv4: %s", ips[i])
	}
}
