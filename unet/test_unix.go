package unet

import "testing"

func TestUnixSocketPair(t *testing.T) {
	pair, err := NewSocketPair()
	if err != nil {
		t.Fatalf("socketpair: %s", err)
	} else if nil == pair[0] {
		t.Fatalf("socketpair 0 is nil!")
	} else if nil == pair[1] {
		t.Fatalf("socketpair 1 is nil!")
	}

	expect := "the quick brown fox"
	nwrote, err := pair[0].Write([]byte(expect))
	if err != nil {
		t.Fatalf("write: %s", err)
	} else if len(expect) != nwrote {
		t.Fatalf("expected to write %d, wrote %d", len(expect), nwrote)
	}

	recvbuff := [48]byte{}
	nread, err := pair[1].Read(recvbuff[:])
	if err != nil {
		t.Fatalf("read: %s", err)
	} else if nread != nwrote {
		t.Fatalf("expected to read %d, got %d", nwrote, nread)
	} else if expect != string(recvbuff[:nread]) {
		t.Fatalf("did not get expected bytes")
	}

	err = pair[0].Close()
	if err != nil {
		t.Fatalf("Unable to close sock 0: %s", err)
	}
	err = pair[1].Close()
	if err != nil {
		t.Fatalf("Unable to close sock 1: %s", err)
	}
}
