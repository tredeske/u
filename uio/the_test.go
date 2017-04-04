package uio

import (
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
