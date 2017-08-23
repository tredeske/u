package uio

import (
	"bytes"
	"net"
	"os"
	"testing"
)

func TestFileCreateExistsRemove(t *testing.T) {

	file := "./FileCreateTest.txt"
	err := FileCreate(file,

		func(f *os.File) (err error) {
			_, err = f.Write([]byte("just a test"))
			return
		})

	if err != nil {
		t.Fatalf("Unable to create %s: %s", file, err)
	}

	if !FileExists(file) {
		t.Fatalf("File %s does not exist!")
	}

	err = FileRemoveAll(file)
	if err != nil {
		t.Fatalf("Unable to rm %s: %s", file, err)
	}

	if FileExists(file) {
		t.Fatalf("File %s still exists!")
	}
}

func TestFdsOpen(t *testing.T) {

	fds, err := FdsOpen(20)
	if err != nil {
		t.Fatalf("Unable to determine open files: %s", err)
	}

	initFds := len(fds)

	//
	// open 2 fds for pipe
	//
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Unable to determine open pipe: %s", err)
	}

	//
	// this will open another 2.  the extra is to the name resolver
	//
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Unable to listen: %s", err)
	}

	fds, err = FdsOpen(20)
	if err != nil {
		t.Fatalf("Unable to determine open files: %s", err)
	}

	pipeFds := len(fds)
	if pipeFds != initFds+4 {
		t.Fatalf("Should only be 4 additional fds.  Instead %d -> %d",
			initFds, pipeFds)
	}

	r.Close()
	w.Close()
	l.Close() // will not close conn to resolver, so we get an extra one

	fds, err = FdsOpen(20)
	if err != nil {
		t.Fatalf("Unable to determine open files: %s", err)
	}

	if len(fds) != initFds+1 {
		t.Fatalf("Too many fds: %d vs %d", len(fds), initFds+1)
	}
}

func TestCopy(t *testing.T) {
	var srcBB, dstBB bytes.Buffer

	text := "this is a test"
	srcBB.WriteString(text)

	amount := int64(srcBB.Len())

	nwrote, err := CopyTo(&dstBB, &srcBB, 0) // not checking amount
	if err != nil {
		t.Fatalf("Copy failed: %s", err)
	} else if nwrote != amount {
		t.Fatalf("Should have copied %d, but instead got %d", amount, nwrote)
	} else if text != dstBB.String() {
		t.Fatalf("Received invalid string: '%s' should be '%s'",
			dstBB.String(), text)
	}

	srcBB.Reset()
	srcBB.WriteString(text)
	dstBB.Reset()
	nwrote, err = CopyTo(&dstBB, &srcBB, amount)
	if err != nil {
		t.Fatalf("Copy failed: %s", err)
	} else if nwrote != amount {
		t.Fatalf("Should have copied %d, but instead got %d", amount, nwrote)
	} else if text != dstBB.String() {
		t.Fatalf("Received invalid string: '%s' should be '%s'",
			dstBB.String(), text)
	}
}
