package usftp

import (
	"os"
)

// toFileMode converts sftp filemode bits to the os.FileMode specification
func toFileMode(mode uint32) os.FileMode {
	var fm = os.FileMode(mode & 0777)

	switch FileMode(mode) & ModeType {
	case ModeDevice:
		fm |= os.ModeDevice
	case ModeCharDevice:
		fm |= os.ModeDevice | os.ModeCharDevice
	case ModeDir:
		fm |= os.ModeDir
	case ModeNamedPipe:
		fm |= os.ModeNamedPipe
	case ModeSymlink:
		fm |= os.ModeSymlink
	case ModeRegular:
		// nothing to do
	case ModeSocket:
		fm |= os.ModeSocket
	}

	if FileMode(mode)&ModeSetUID != 0 {
		fm |= os.ModeSetuid
	}
	if FileMode(mode)&ModeSetGID != 0 {
		fm |= os.ModeSetgid
	}
	if FileMode(mode)&ModeSticky != 0 {
		fm |= os.ModeSticky
	}

	return fm
}

// fromFileMode converts from the os.FileMode specification to sftp filemode bits
func fromFileMode(mode os.FileMode) uint32 {
	ret := FileMode(mode & os.ModePerm)

	switch mode & os.ModeType {
	case os.ModeDevice | os.ModeCharDevice:
		ret |= ModeCharDevice
	case os.ModeDevice:
		ret |= ModeDevice
	case os.ModeDir:
		ret |= ModeDir
	case os.ModeNamedPipe:
		ret |= ModeNamedPipe
	case os.ModeSymlink:
		ret |= ModeSymlink
	case 0:
		ret |= ModeRegular
	case os.ModeSocket:
		ret |= ModeSocket
	}

	if mode&os.ModeSetuid != 0 {
		ret |= ModeSetUID
	}
	if mode&os.ModeSetgid != 0 {
		ret |= ModeSetGID
	}
	if mode&os.ModeSticky != 0 {
		ret |= ModeSticky
	}

	return uint32(ret)
}

const (
	s_ISUID = uint32(ModeSetUID)
	s_ISGID = uint32(ModeSetGID)
	s_ISVTX = uint32(ModeSticky)
)

// S_IFMT is a legacy export, and was brought in to support GOOS environments whose sysconfig.S_IFMT may be different from the value used internally by SFTP standards.
// There should be no reason why you need to import it, or use it, but unexporting it could cause code to break in a way that cannot be readily fixed.
// As such, we continue to export this value as the value used in the SFTP standard.
//
// Deprecated: Remove use of this value, and avoid any future use as well.
// There is no alternative provided, you should never need to access this value.
const S_IFMT = uint32(ModeType)
