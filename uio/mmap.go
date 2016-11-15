package uio

import (
	"os"
	"syscall"
)

// a memory map of file contents
type MMap []byte

// unmap the memory
func (this MMap) Close() error {
	return syscall.Munmap(this)
}

// map the named file into memory, optimized for sequential read of contents.
// note: you must call Close() when done with map
func MapFile(file string) (bytes MMap, stat os.FileInfo, err error) {
	f, err := os.Open(file)
	if err != nil {
		return
	}
	defer f.Close() // close does not cause unmap

	stat, err = f.Stat()
	if err != nil {
		return
	}

	bytes, err = syscall.Mmap(int(f.Fd()), 0, int(stat.Size()), syscall.PROT_READ,
		syscall.MAP_PRIVATE)
	if err != nil {
		return
	}
	err = syscall.Madvise(bytes, syscall.MADV_WILLNEED|syscall.MADV_SEQUENTIAL)
	if err != nil {
		bytes.Close()
		bytes = nil
		return
	}
	return
}
