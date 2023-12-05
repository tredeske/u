package unet

import (
	"testing"
	"time"

	"github.com/tredeske/u/ulog"
	"github.com/tredeske/u/usync"
)

func TestMtu(t *testing.T) {
	host := "127.0.0.1"
	port := uint16(19332)
	var srcAddr, dstAddr Address
	err := srcAddr.FromHostPort(host, port)
	if err != nil {
		t.Fatalf("resolving src: %s", err)
	}
	err = dstAddr.FromHostPort(host, port+1)
	if err != nil {
		t.Fatalf("resolving dst: %s", err)
	}

	ulog.Printf(`

GIVEN default probe parameters
 WHEN probe MTU from %s to %s
 THEN largest MTU will be found

`,
		srcAddr, dstAddr)

	echoer := &MtuEchoer{Name: t.Name()}
	err = echoer.NewSock(dstAddr)
	if err != nil {
		t.Fatalf("echoer.NewSock: %s", err)
	}
	defer echoer.Close()

	errC := usync.NewChan[error](2)
	go func() {
		err := echoer.Echo(5 * time.Second)
		if err != nil {
			errC <- err
		}
	}()
	time.Sleep(100 * time.Millisecond)

	prober := &MtuProber{
		Name: t.Name(),
		BeforeSend: func(size uint16, space []byte) (pkt []byte, err error) {
			err, _ = errC.TryGet()
			if err != nil {
				return
			}
			return space[:size], nil
		},
		MtuMax: MtuMax,
	}
	err = prober.NewSock(srcAddr, dstAddr)
	if err != nil {
		t.Fatalf("prober.NewSock: %s", err)
	}
	defer prober.Close()

	pmtu, err := prober.Probe()
	if err != nil {
		t.Fatalf("prober.Probe: %s (%#v)", err, err)
	} else if 0 >= pmtu {
		t.Fatalf("pmtu not positive (%d)", pmtu)
	}

	ulog.Printf("PMTU: %d", pmtu)

	err, _ = errC.TryGet()
	if err != nil {
		t.Fatalf("echoer problem: %s", err)
	}
}
