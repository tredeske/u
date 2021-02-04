//+build linux

package uio

import (
	"log"
	"os"
	"syscall"
)

//
//
//
func DumpFds() (err error) {
	files, err := DirFilenames("/proc/self/fd")
	if err != nil {
		return
	}
	for _, f := range files {
		f = "/proc/self/fd/" + f
		link, _ := os.Readlink(f)

		log.Printf("%s -> %s", f, link)
	}
	return
}

//
// Get the max allowed number of open files
//
func FdsMax() (currMax, allowedMax int, err error) {
	rlim := syscall.Rlimit{}
	err = syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlim)
	if err != nil {
		return
	}
	currMax = int(rlim.Cur)
	allowedMax = int(rlim.Max)
	return
}

//
// Return current valid file descriptors, up to maxFd.
//
// If maxFd is unspecified or non-positive, then determine from rlimit
//
func FdsOpen(maxFd ...int) (fds []int, err error) {
	max := -1
	if 0 != len(maxFd) {
		max = maxFd[0]
	}
	if 0 >= max {
		max, _, err = FdsMax()
		if err != nil {
			return
		}
	}
	stat := syscall.Stat_t{}
	for i := 0; i < max; i++ {
		statErr := syscall.Fstat(i, &stat)
		if statErr != nil {
			errno, isAnErrno := statErr.(syscall.Errno)
			if !isAnErrno || errno != syscall.EBADF {
				err = statErr
				break
			}
		} else {
			fds = append(fds, i)
		}
	}
	return
}
