package unet

import (
	"sync/atomic"
	"syscall"
)

// Manages a socket file descriptor (fd)
//
// When a goroutine is using an fd, it may be blocked on it.  When it is time
// for that goroutine to die, another goroutine will need to tell it.  The
// safe way to do that is to shutdown the fd, which will unblock the goroutine,
// and then that goroutine can close the fd.
//
// This allows that activity to be safely performed.
//
// On Linux, fds are limited to 32 bits (see epoll interfaces).
// OS limits push that down considerably.
//
// NOTES:
//   - When a TCP socket is shutdown, reads will return 0 bytes, so the reader
//     needs to check IsDisabled upon getting 0 bytes.
//   - When a TCP listen socket is shutdown, the goroutine waiting for connections
//     will get an error (in the accept).  IsDisabled should be checked.
//   - When a UDP socket is shutdown, recvmmsg will return 1 message, but the
//     first message will be 0 bytes.  IsDisabled needs to be checked in this case.
type ManagedFd uint64

const (
	fdMask_      = 0x0000_ffff_ffff_ffff
	fdCountMask_ = 0x0fff_0000_0000_0000
	fdCountOne_  = 0x0001_0000_0000_0000
	openBit_     = 0x4000_0000_0000_0000
	disableBit_  = 0x8000_0000_0000_0000
)

func (this *ManagedFd) load() uint64 {
	return atomic.LoadUint64((*uint64)(this))
}

func (this *ManagedFd) store(newV uint64) {
	atomic.StoreUint64((*uint64)(this), newV)
}

func (this *ManagedFd) cas(oldV, newV uint64) bool {
	return atomic.CompareAndSwapUint64((*uint64)(this), oldV, newV)
}

// Transfer state from other ManagedFd to this
//
// On success, from will be cleared and this will contain the state.
//
// # On failure, both this and from will remain unchanged
//
// Failure is caused when this already has state
func (this *ManagedFd) From(from *ManagedFd) (ok bool) {
	ok = this.cas(0, from.load())
	if ok {
		from.store(0)
	}
	return
}

// set the file descriptor to be managed.
//
// this will return false if the ManagedFd is disabled or already set
func (this *ManagedFd) Set(fd int) (actuallySet bool) {
	if 0 > fd {
		panic("fd must be non-negative")
	} else if fdMask_ < fd {
		panic("fd too large")
	}
	return this.cas(0, uint64(fd)|openBit_)
}

// clear all state - only use when you are *sure*
func (this *ManagedFd) Clear() {
	this.store(0)
}

func (this *ManagedFd) Get() (fd int, valid bool) {
	fd = -1
	v := this.load()
	valid = openBit_ == (v & (disableBit_ | openBit_))
	if valid {
		fd = int(v & fdMask_)
	}
	return
}

func (this *ManagedFd) Eject() (fd int, valid bool) {
	fd = -1
	v := this.load()
	if 0 != (v & openBit_) {
		// try to clear the fd, but preserve the disable bit
		// if that fails, try again assuming they changed the disable it
		if this.cas(v, v&disableBit_) || this.cas(v|disableBit_, v&disableBit_) {
			fd = int(v & fdMask_)
			valid = true
		} else { // someone changed it.  try once more.
			v = this.load()
			if 0 != (v&openBit_) &&
				this.cas(v, v&disableBit_) {
				fd = int(v & fdMask_)
				valid = true
			}
		}
	}
	return
}

// replace the fd (if any) with newFd.  this will also undisable.
//
// oldFd will be -1 if no valid fd was there, or if replacement failed
func (this *ManagedFd) Replace(newFd int) (oldFd int, replaced bool) {
	oldFd = -1
	if 0 > newFd {
		panic("negative fd")
	}
	v := this.load()
	if this.cas(v, uint64(newFd)|openBit_) {
		replaced = true
		if 0 != (v & openBit_) {
			oldFd = int(v & fdMask_)
		}
	}
	return
}

// replace the fd (if any) with newFd.  this will also undisable.
//
// if a valid fd was set, it will be closed
func (this *ManagedFd) ReplaceAndClose(newFd int) (replaced bool) {
	if 0 > newFd {
		panic("negative fd")
	}
	v := this.load()
	if this.cas(v, uint64(newFd)|openBit_) {
		replaced = true
		if 0 != (v & openBit_) {
			fd := int(v & fdMask_)
			syscall.Close(fd)
		}
	}
	return
}

// disabled or open or previously used
func (this *ManagedFd) IsSet() bool {
	return 0 != this.load()
}

func (this *ManagedFd) IsClosed() (closed bool) {
	return 0 == (this.load() & openBit_)
}

func (this *ManagedFd) IsDisabled() (disabled bool) {
	return 0 != (this.load() & disableBit_)
}

func (this *ManagedFd) IsDisabledOrClosed() (disabled, closed bool) {
	v := this.load()
	return 0 != (v & disableBit_), 0 == (v & openBit_)
}

func (this *ManagedFd) Close() (closed bool, err error) {
retry:
	v := this.load()
	if 0 != (v & openBit_) {
		// try to clear the fd, but preserve the disable bit
		// if that fails, try again assuming they changed the disable it
		if this.cas(v, v&disableBit_) || this.cas(v|disableBit_, v&disableBit_) {
			err = syscall.Close(int(v & fdMask_))
			closed = true
		} else {
			goto retry
		}
	}
	return
}

// disable the fd
//
// this is commonly used when a goroutine may be blocking on an fd, and another
// goroutine is trying to tell it that it is time to die.  this provides that
// notification to the blocked goroutine, but keeps the fd open.  this prevents
// the race condition where one goroutine closes a fd, and then a new connection
// is made that gets the same fd, and then the other goroutine that was using
// the fd now has an fd to the wrong thing
func (this *ManagedFd) Disable() (disabled bool) {
	v := this.load()
	if openBit_ == (v&(openBit_|disableBit_)) && this.cas(v, v|disableBit_) {
		syscall.Shutdown(int(v&fdMask_), syscall.SHUT_RDWR)
		disabled = true
	}
	return
}

// add a reference count to this if it is valid, returning fd if valid
func (this *ManagedFd) Acquire() (fd int, valid bool) {
	fd = -1
retry:
	v := this.load()
	valid = openBit_ == (v & (disableBit_ | openBit_))
	if valid {
		if v&fdCountMask_ == fdCountMask_ {
			panic("fd acquire overflow")
		}
		if !this.cas(v, v+fdCountOne_) {
			goto retry
		}
		fd = int(v & fdMask_)
	}
	return
}

// remove a reference count to this, returning status and remaining refs
func (this *ManagedFd) Release() (disabled bool, count int) {
retry:
	v := this.load()
	if 0 != v&fdCountMask_ {
		w := v - fdCountOne_
		if !this.cas(v, w) {
			goto retry
		}
		v = w
	}
	return 0 != (v & disableBit_), int(v & fdCountMask_ >> 48)
}

// remove a ref to this, disabling if not disabled, closing if count zero
func (this *ManagedFd) ReleaseAndDisableAndMaybeClose() (count int) {
	var disabled bool
	disabled, count = this.Release()
	if !disabled {
		this.Disable()
	}
	if 0 == count {
		this.Close()
	}
	return
}
