package unet

import (
	"fmt"
	"syscall"
	"testing"
	"time"

	"github.com/tredeske/u/ulog"
)

func TestUdpEndpoint(t *testing.T) {

	const port = 47999
	const txMessages = 32
	const msgSz = 1024
	const startId = byte(7)

	ulog.Printf(`
GIVEN: udp socket bound to port %d
 WHEN: udp socket connected to port %d
  AND: %d messages sent
 THEN: %d messaged are received
`, port, port, txMessages, txMessages)

	var rxMfd ManagedFd
	rxFd := -1
	rxSock := Socket{}
	err := rxSock.
		ResolveNearAddr("0.0.0.0", port).
		ConstructUdp().
		Bind().
		ManageFd(&rxMfd).
		//GiveMeTheFreakingFd(&rxFd).
		Error
	if err != nil {
		t.Fatalf("unable to set up txSock: %s", err)
	}
	defer rxMfd.Close()

	rxFd, valid := rxMfd.Get()
	if !valid {
		t.Fatalf("rx fd not valid!")
	}

	txFd := -1
	txSock := Socket{}
	var txMfd ManagedFd
	err = txSock.
		ResolveFarAddr("127.0.0.1", port).
		ConstructUdp().
		Connect().
		ManageFd(&txMfd).
		//GiveMeTheFreakingFd(&txFd).
		Error
	if err != nil {
		t.Fatalf("unable to set up txSock: %s", err)
	}
	defer txMfd.Close()

	txFd, valid = txMfd.Get()
	if !valid {
		t.Fatalf("tx fd not valid!")
	}

	rx := &UdpEndpoint{}
	rx.SetupVectors(txMessages/8, 1, rx.IovFiller(msgSz), rx.RecvNamer)

	tx := &UdpEndpoint{}
	pktId := startId
	tx.SetupVectors(txMessages, 1,
		func(iov []syscall.Iovec) {
			for i, N := 0, len(iov); i < N; i++ {
				b := make([]byte, msgSz)
				b[0] = pktId
				pktId++
				iov[i].Base = &b[0]
				iov[i].Len = msgSz
			}
		},
		nil)

	//
	// send the messages
	//
	retries, errno := tx.SendMMsgRetry(txFd, txMessages)
	if errno != 0 {
		t.Fatalf("unable to send: %s", errno)
	} else if 0 != retries {
		fmt.Printf("Retries: %d\n", retries)
	}

	//
	// receive the messages
	//
	var rxMessages, messages int
	pktId = startId
	for i := 0; i < txMessages && rxMessages != txMessages; i++ {
		fmt.Printf("recv %d\n", i)
		messages, errno = rx.RecvMMsg(rxFd)
		if errno != 0 {
			t.Fatalf("unable to recv: %s", err)
		} else if 0 >= messages {
			t.Fatalf("invalid messages value: %d", messages)
		}
		for m := 0; m < messages; m++ {
			if 0 != rx.Hdrs[m].Flags {
				t.Fatalf("Got a non-zero flag: %x", rx.Hdrs[m].Flags)
			} else if 0 == rx.Hdrs[m].NTransferred {
				t.Fatalf("Got %d messages, but message %d none tranferred",
					messages, m)
			} else if msgSz != rx.Hdrs[m].NTransferred {
				t.Fatalf("Message %d only had %d bytes", m, rx.Hdrs[m].NTransferred)
			} else if pktId != *rx.Iov[m].Base {
				t.Fatalf("Message %d expected %x, got %x", m, pktId,
					*rx.Iov[m].Base)
			}
			pktId++
			rx.Hdrs[m].NTransferred = 0
		}
		rxMessages += messages
	}
	if rxMessages != txMessages {
		t.Fatalf("rxMessages (%d) != txMessages (%d)", rxMessages, txMessages)
	}

	//
	// try a read shutdown on rx sock
	//
	ulog.Printf(`
GIVEN: udp rx socket open
 WHEN: shutdown read
 THEN: read detects shutdown
 `)
	go func() {
		time.Sleep(10 * time.Millisecond)
		rxMfd.ShutdownRead()
	}()
	messages, errno = rx.RecvMMsg(rxFd)
	if 0 != errno {
		t.Fatalf("Get error %d (%s)", errno, errno)
	}

	// when a shutdown is detected, we see messages set to 1 and NTransferred set
	// to zero for that message.
	if 0 != messages && !(1 == messages && 0 == rx.Hdrs[0].NTransferred) {
		t.Fatalf("should not get any messages. Got %#v", rx)
	}

	//
	// try a read shutdown on tx sock
	//
	ulog.Printf(`
GIVEN: udp tx socket open
 WHEN: shutdown read of tx sock
 THEN: write still works
 `)
	txMfd.ShutdownRead()
	retries, errno = tx.SendMMsgRetry(txFd, txMessages)
	if errno != 0 {
		t.Fatalf("unable to send: %s", errno)
	} else if 0 != retries {
		fmt.Printf("Retries: %d\n", retries)
	}
}
