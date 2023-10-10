package unet

import (
	"testing"
	"time"

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
