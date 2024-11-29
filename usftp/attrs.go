package usftp

// ssh_FXP_ATTRS support
// see https://filezilla-project.org/specs/draft-ietf-secsh-filexfer-02.txt#section-5

import (
	"os"
	"time"
)

const (
	sshFileXferAttrSize        = 0x00000001
	sshFileXferAttrUIDGID      = 0x00000002
	sshFileXferAttrPermissions = 0x00000004
	sshFileXferAttrACmodTime   = 0x00000008
	sshFileXferAttrExtended    = 0x80000000

	sshFileXferAttrAll = sshFileXferAttrSize | sshFileXferAttrUIDGID | sshFileXferAttrPermissions |
		sshFileXferAttrACmodTime | sshFileXferAttrExtended
)

// fileInfo is an artificial type designed to satisfy os.FileInfo.
type fileInfo struct {
	name string
	stat *FileStat
}

// Name returns the base name of the file.
func (fi *fileInfo) Name() string { return fi.name }

// Size returns the length in bytes for regular files; system-dependent for others.
func (fi *fileInfo) Size() int64 { return int64(fi.stat.Size) }

// Mode returns file mode bits.
func (fi *fileInfo) Mode() os.FileMode { return fi.stat.OsFileMode() }

// ModTime returns the last modification time of the file.
func (fi *fileInfo) ModTime() time.Time { return fi.stat.ModTime() }

// IsDir returns true if the file is a directory.
func (fi *fileInfo) IsDir() bool { return fi.Mode().IsDir() }

func (fi *fileInfo) Sys() interface{} { return fi.stat }

// FileStat holds the original unmarshalled values from a call to READDIR or
// *STAT. It is exported for the purposes of accessing the raw values via
// os.FileInfo.Sys(). It is also used server side to store the unmarshalled
// values for SetStat.
type FileStat struct {
	Size     uint64
	Mode     uint32
	Mtime    uint32
	Atime    uint32
	UID      uint32
	GID      uint32
	Extended []StatExtended
}

// returns the FileMode, containing type and permission bits
func (fs *FileStat) FileMode() FileMode {
	return FileMode(fs.Mode)
}

// returns the Type bits of the FileMode
func (fs *FileStat) FileType() FileMode {
	return FileMode(fs.Mode) & ModeType
}

// returns true if the mode describes a regular file.
func (fs *FileStat) IsRegular() bool {
	return FileMode(fs.Mode)&ModeType == ModeRegular
}

// returns true if the mode describes a directory
func (fs *FileStat) IsDir() bool {
	return FileMode(fs.Mode)&ModeType == ModeDir
}

// ModTime returns the Mtime SFTP file attribute converted to a time.Time
func (fs *FileStat) ModTime() time.Time {
	return time.Unix(int64(fs.Mtime), 0)
}

// AccessTime returns the Atime SFTP file attribute converted to a time.Time
func (fs *FileStat) AccessTime() time.Time {
	return time.Unix(int64(fs.Atime), 0)
}

// returns the Mode SFTP file attribute converted to an os.FileMode
func (fs *FileStat) OsFileMode() os.FileMode {
	return toFileMode(fs.Mode)
}

// StatExtended contains additional, extended information for a FileStat.
type StatExtended struct {
	ExtType string
	ExtData string
}

// convert a FileStat and filename to a go os.FileInfo
func FileInfoFromStat(stat *FileStat, name string) os.FileInfo {
	return &fileInfo{
		name: name,
		stat: stat,
	}
}

// FileInfoUidGid extends os.FileInfo and adds callbacks for Uid and Gid retrieval,
// as an alternative to *syscall.Stat_t objects on unix systems.
type FileInfoUidGid interface {
	os.FileInfo
	Uid() uint32
	Gid() uint32
}

// FileInfoUidGid extends os.FileInfo and adds a callbacks for extended data retrieval.
type FileInfoExtendedData interface {
	os.FileInfo
	Extended() []StatExtended
}

func fileStatFromInfo(fi os.FileInfo) (uint32, *FileStat) {
	mtime := fi.ModTime().Unix()
	atime := mtime
	var flags uint32 = sshFileXferAttrSize |
		sshFileXferAttrPermissions |
		sshFileXferAttrACmodTime

	fileStat := &FileStat{
		Size:  uint64(fi.Size()),
		Mode:  fromFileMode(fi.Mode()),
		Mtime: uint32(mtime),
		Atime: uint32(atime),
	}

	// os specific file stat decoding
	fileStatFromInfoOs(fi, &flags, fileStat)

	// The call above will include the sshFileXferAttrUIDGID in case
	// the os.FileInfo can be casted to *syscall.Stat_t on unix.
	// If fi implements FileInfoUidGid, retrieve Uid, Gid from it instead.
	if fiExt, ok := fi.(FileInfoUidGid); ok {
		flags |= sshFileXferAttrUIDGID
		fileStat.UID = fiExt.Uid()
		fileStat.GID = fiExt.Gid()
	}

	// if fi implements FileInfoExtendedData, retrieve extended data from it
	if fiExt, ok := fi.(FileInfoExtendedData); ok {
		fileStat.Extended = fiExt.Extended()
		if len(fileStat.Extended) > 0 {
			flags |= sshFileXferAttrExtended
		}
	}

	return flags, fileStat
}

// FileMode represents a file’s mode and permission bits.
// The bits are defined according to POSIX standards,
// and may not apply to the OS being built for.
type FileMode uint32

// Permission flags, defined here to avoid potential inconsistencies in individual OS implementations.
const (
	ModePerm       FileMode = 0o0777 // S_IRWXU | S_IRWXG | S_IRWXO
	ModeUserRead   FileMode = 0o0400 // S_IRUSR
	ModeUserWrite  FileMode = 0o0200 // S_IWUSR
	ModeUserExec   FileMode = 0o0100 // S_IXUSR
	ModeGroupRead  FileMode = 0o0040 // S_IRGRP
	ModeGroupWrite FileMode = 0o0020 // S_IWGRP
	ModeGroupExec  FileMode = 0o0010 // S_IXGRP
	ModeOtherRead  FileMode = 0o0004 // S_IROTH
	ModeOtherWrite FileMode = 0o0002 // S_IWOTH
	ModeOtherExec  FileMode = 0o0001 // S_IXOTH

	ModeSetUID FileMode = 0o4000 // S_ISUID
	ModeSetGID FileMode = 0o2000 // S_ISGID
	ModeSticky FileMode = 0o1000 // S_ISVTX

	ModeType       FileMode = 0xF000 // S_IFMT
	ModeNamedPipe  FileMode = 0x1000 // S_IFIFO
	ModeCharDevice FileMode = 0x2000 // S_IFCHR
	ModeDir        FileMode = 0x4000 // S_IFDIR
	ModeDevice     FileMode = 0x6000 // S_IFBLK
	ModeRegular    FileMode = 0x8000 // S_IFREG
	ModeSymlink    FileMode = 0xA000 // S_IFLNK
	ModeSocket     FileMode = 0xC000 // S_IFSOCK
)

// IsDir reports whether m describes a directory.
// That is, it tests for m.Type() == ModeDir.
func (m FileMode) IsDir() bool {
	return (m & ModeType) == ModeDir
}

// IsRegular reports whether m describes a regular file.
// That is, it tests for m.Type() == ModeRegular
func (m FileMode) IsRegular() bool {
	return (m & ModeType) == ModeRegular
}

// Perm returns the POSIX permission bits in m (m & ModePerm).
func (m FileMode) Perm() FileMode {
	return (m & ModePerm)
}

// Type returns the type bits in m (m & ModeType).
func (m FileMode) Type() FileMode {
	return (m & ModeType)
}
