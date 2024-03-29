package unet

import (
	"fmt"
	"syscall"
	"testing"
)

func TestAddress(t *testing.T) {

	const PORT = 5

	for _, testCase := range []struct {
		name    string
		version string
		addr    string
	}{
		{
			name:    "ipv6-local",
			version: "ipv6",
			addr:    "::1",
		}, {
			name:    "ipv6-any",
			version: "ipv6",
			addr:    "::",
		}, {
			name:    "ipv4-local",
			version: "ipv4",
			addr:    "127.0.0.1",
		}, {
			name:    "ipv4-any",
			version: "ipv4",
			addr:    "0.0.0.0",
		},
	} {
		t.Run(t.Name()+"-"+testCase.name, func(t *testing.T) {
			addr := Address{}

			fmt.Printf(`
GIVEN %s addr %s
 WHEN resolve IP
 THEN IP resolved
 `, testCase.version, testCase.addr)
			ip, err := ResolveIp(testCase.addr)
			if err != nil {
				t.Fatalf("could not resolve %s: %s", testCase.addr, err)
			}

			fmt.Printf(`
GIVEN %s IP %s
 WHEN create Address from IP
 THEN conversion successful
 `, testCase.version, ip)
			addr.FromIpAndPort(ip, PORT)
			if !addr.IsIpSet() {
				t.Fatalf("IP not set!")
			} else if !addr.IsPortSet() {
				t.Fatalf("port not set!")
			} else if "ipv4" == testCase.version && !addr.IsIpv4() {
				t.Fatalf("should be ipv4: %#v", addr)
			} else if "ipv4" == testCase.version && addr.IsIpv6() {
				t.Fatalf("should not be ipv6: %#v", addr)
			} else if "ipv6" == testCase.version && !addr.IsIpv6() {
				t.Fatalf("should be ipv6: %#v", addr)
			} else if "ipv6" == testCase.version && addr.IsIpv4() {
				t.Fatalf("should not be ipv4: %#v", addr)
			} else if PORT != addr.Port() {
				t.Fatalf("port should be %d: %#v", PORT, addr)
			} else if !ip.Equal(addr.AsIp()) {
				t.Fatalf("should be equal: %#v and %#v", ip, addr)
			}

			fmt.Printf(`
GIVEN %s IP %s
 WHEN create sockaddr from Address
  AND create new Address back from sockaddr
 THEN recovered Address matches original Address
 `, testCase.version, ip)
			sockaddr := addr.AsSockaddr()
			addr2 := Address{}
			addr2.FromSockaddr(sockaddr)
			if addr != addr2 {
				t.Fatalf("(sockaddr) should be equal: %#v and %#v", addr, addr2)
			} else if !IsSockaddrValid(sockaddr) {
				t.Fatalf("sockaddr not valid!")
			} else if IsSockaddrZero(sockaddr) { // port always set...
				t.Fatalf("sockaddr should not be zero!")
			}
			if "::" == testCase.addr || "0.0.0.0" == testCase.addr {
				if IsSockaddrZero(sockaddr) {
					t.Fatalf("sockaddr port and ip should NOT be set! %#v", sockaddr)
				}
				if !addr.IsIpZero() {
					t.Fatalf("addr should be zero! %#v", addr)
				}
			} else {
				if IsSockaddrPortOrIpZero(sockaddr) {
					t.Fatalf("sockaddr port and ip should be set! %#v", sockaddr)
				}
				if addr.IsIpZero() {
					t.Fatalf("addr should NOT be zero! %#v %t", addr,
						ipv4Bits_ == (addr.addr2&ipv4Mask_))
				}
			}

			fmt.Printf(`
GIVEN %s IP %s
 WHEN create name bytes from Address
  AND create new Address back from name bytes
 THEN recovered Address matches original Address
 `, testCase.version, ip)
			space := make([]byte, syscall.SizeofSockaddrInet6)
			name, namelen := addr.AsNameBytes(space)
			addr3 := Address{}
			addr3.FromNameBytes(name, namelen)
			if addr != addr3 {
				t.Fatalf("(name bytes) should be equal: %#v and %#v", addr, addr3)
			}
		})
	}

	/*
			addr := Address{}

			ip, err := ResolveIp("::1")
			if err != nil {
				t.Fatalf("could not resolve ::1: %s", err)
			}
			fmt.Printf(`
		GIVEN ipv6 addr %s
		 WHEN create Address
		 THEN conversion successful
		 `, ip)
			addr.FromIpAndPort(ip, 5)
			if addr.IsIpv4() {
				t.Fatalf("should not be ipv4: %#v", addr)
			} else if !addr.IsIpv6() {
				t.Fatalf("should be ipv6: %#v", addr)
			} else if 5 != addr.Port() {
				t.Fatalf("port should be 5: %#v", addr)
			} else if !ip.Equal(addr.AsIp()) {
				t.Fatalf("equal: %#v and %#v", ip, addr)
			}

			ip, err = ResolveIp("127.0.0.1")
			if err != nil {
				t.Fatalf("could not resolve 127.0.0.1: %s", err)
			}
			fmt.Printf(`
		GIVEN ipv4 addr %s
		 WHEN create Address
		 THEN conversion successful
		 `, ip)
			addr.FromIpAndPort(ip, 5)
			fmt.Printf("IPv4: %#v\n", addr)
			if !addr.IsIpv4() {
				t.Fatalf("should be ipv4: %#v", addr)
			} else if addr.IsIpv6() {
				t.Fatalf("should not be ipv6: %#v", addr)
			} else if 5 != addr.Port() {
				t.Fatalf("port should be 5: %#v", addr)
			} else if !ip.Equal(addr.AsIp()) {
				t.Fatalf("equal: %#v and %#v", ip, addr)
			}
	*/
}

func TestSockaddrZero(t *testing.T) {
	if !IsSockaddrZero(nil) {
		t.Fatalf("nil should be detected as sockaddr zero!")
	} else if !IsSockaddrPortOrIpZero(nil) {
		t.Fatalf("nil should be detected as sockaddr either zero!")
	} else if IsSockaddrValid(nil) {
		t.Fatalf("nil sockaddr should be invalid")
	}

	var sa syscall.Sockaddr

	if !IsSockaddrZero(sa) {
		t.Fatalf("unset sockaddr should be zero!")
	} else if !IsSockaddrPortOrIpZero(sa) {
		t.Fatalf("unset should be detected as sockaddr either zero!")
	} else if IsSockaddrValid(sa) {
		t.Fatalf("unset sockaddr should be invalid")
	}

	var sa4 *syscall.SockaddrInet4
	sa = sa4
	if !IsSockaddrZero(sa) {
		t.Fatalf("nil sockaddr should not be zero!")
	} else if !IsSockaddrPortOrIpZero(sa) {
		t.Fatalf("nil sockaddr should be detected as sockaddr either zero!")
	} else if IsSockaddrValid(sa) {
		t.Fatalf("nil sockaddr should be invalid")
	}

	var saZero syscall.SockaddrInet4
	sa = &saZero
	if !IsSockaddrZero(sa) {
		t.Fatalf("zero sockaddr should be detected!")
	} else if !IsSockaddrPortOrIpZero(sa) {
		t.Fatalf("zero sockaddr should be detected as sockaddr either zero!")
	} else if !IsSockaddrValid(sa) {
		t.Fatalf("zero sockaddr should be valid")
	}

	var saZero6 syscall.SockaddrInet6
	sa = &saZero6
	if !IsSockaddrZero(sa) {
		t.Fatalf("zero6 sockaddr should be detected!")
	} else if !IsSockaddrPortOrIpZero(sa) {
		t.Fatalf("zero6 sockaddr should be detected as sockaddr either zero!")
	} else if !IsSockaddrValid(sa) {
		t.Fatalf("zero6 sockaddr should be valid")
	}

}
