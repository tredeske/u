package unet

import (
	"syscall"
	"testing"
)

func TestAcquireReleaseFd(t *testing.T) {
	var mfd ManagedFd

	pipeFds := [2]int{}
	err := syscall.Pipe(pipeFds[:])
	if err != nil {
		t.Fatalf("Unable to create pipe: %s", err)
	} else if !isOpen(pipeFds[0]) {
		t.Fatalf("pipe fd 0 not open!")
	}

	defer func() {
		syscall.Close(pipeFds[1])
	}()

	ok := mfd.Set(pipeFds[0])
	if !ok {
		t.Fatalf("Unable to set fd")
	}
	fd, valid := mfd.Get()
	if !valid {
		t.Fatalf("fd should be valid")
	}

	mfd.Acquire()
	mfd.Acquire()
	count := mfd.ReleaseAndDisableAndMaybeClose()
	if 1 != count {
		t.Fatalf("should only be 1 ref left")
	} else if !mfd.IsDisabled() {
		t.Fatalf("should be disabled after release")
	} else if mfd.IsClosed() {
		t.Fatalf("should not be marked closed yet!")
	} else if !isOpen(fd) {
		t.Fatalf("pipe fd 0 not open anymore!")
	}

	count = mfd.ReleaseAndDisableAndMaybeClose()
	if 0 != count {
		t.Fatalf("should only be 1 ref left")
	} else if !mfd.IsDisabled() {
		t.Fatalf("should be disabled after release")
	} else if !mfd.IsClosed() {
		t.Fatalf("should be marked closed!")
	} else if isOpen(fd) {
		t.Fatalf("pipe fd still open!")
	}
}

func TestManagedFd(t *testing.T) {
	var mfd ManagedFd

	pipeFds := [2]int{}
	err := syscall.Pipe(pipeFds[:])
	if err != nil {
		t.Fatalf("Unable to create pipe: %s", err)
	} else if !isOpen(pipeFds[0]) {
		t.Fatalf("pipe fd 0 not open!")
	}

	//
	// try to get non-existing fd
	//
	fd, valid := mfd.Get()
	if valid {
		t.Fatalf("fd should not be valid")
	} else if 0 <= fd {
		t.Fatalf("fd should not be valid")
	} else if mfd.IsDisabled() {
		t.Fatalf("mfd should not be disabled")
	} else if !mfd.IsClosed() {
		t.Fatalf("mfd should be closed")
	}

	//
	// try to eject non-existing fd
	//
	fd, valid = mfd.Eject()
	if valid {
		t.Fatalf("fd should not be valid")
	} else if 0 <= fd {
		t.Fatalf("fd should not be valid")
	}

	//
	// try to close non-existing fd
	//
	ok, _ := mfd.Close()
	if ok {
		t.Fatalf("should not have been able to close")
	}

	//
	// set an fd
	//
	ok = mfd.Set(pipeFds[0])
	if !ok {
		t.Fatalf("Unable to set fd")
	}
	fd, valid = mfd.Get()
	if !valid {
		t.Fatalf("fd should be valid")
	} else if 0 > fd {
		t.Fatalf("fd should be valid")
	} else if mfd.IsDisabled() {
		t.Fatalf("mfd should not be disabled")
	} else if mfd.IsClosed() {
		t.Fatalf("mfd should NOT be closed")
	} else if pipeFds[0] != fd {
		t.Fatalf("did not get back correct fd")
	}

	//
	// eject existing fd
	//
	fd, valid = mfd.Eject()
	if !valid {
		t.Fatalf("fd should be valid")
	} else if 0 > fd {
		t.Fatalf("fd should be valid")
	} else if mfd.IsDisabled() {
		t.Fatalf("mfd should not be disabled")
	} else if !mfd.IsClosed() {
		t.Fatalf("mfd should be closed")
	}

	//
	// set it again
	//
	ok = mfd.Set(pipeFds[0])
	if !ok {
		t.Fatalf("Unable to set fd")
	}

	//
	// replace and close
	//
	ok = mfd.ReplaceAndClose(pipeFds[1])
	if !ok {
		t.Fatalf("Unable to replace fd")
	} else if isOpen(pipeFds[0]) {
		t.Fatalf("did not close pipe 0")
	} else if !isOpen(pipeFds[1]) {
		t.Fatalf("pipe 1 closed!")
	}

	//
	// close existing fd
	//
	ok, _ = mfd.Close()
	if !ok {
		t.Fatalf("should have closed")
	} else if isOpen(pipeFds[1]) {
		t.Fatalf("did not close pipe 0")
	}

}

func isOpen(fd int) bool {
	stat := syscall.Stat_t{}
	statErr := syscall.Fstat(fd, &stat)
	if statErr != nil {
		errno, isAnErrno := statErr.(syscall.Errno)
		if isAnErrno && errno == syscall.EBADF {
			return false
		}
	}
	return true
}
