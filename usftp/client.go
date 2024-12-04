package usftp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/tredeske/u/uerr"
	"golang.org/x/crypto/ssh"
)

// A ClientOption is a function which applies configuration to a Client.
type ClientOption func(*Client) error

// Set the maximum size (bytes) of the payload.
//
// The larger the payload, the more efficient the transport.
//
// The default is 32768 (32KiB), which all compliant SFTP servers must support.
// - OpenSsh supports 255KiB (version 8.7 was used for the test)
//
// If you get the error "failed to send packet header: EOF" when copying a
// large file, try lowering this number.
func WithMaxPacket(size int) ClientOption {
	return func(c *Client) error {
		if size < 8192 {
			return errors.New("maxPacket must be greater or equal to 8192")
		}
		c.maxPacket = size
		return nil
	}
}

// Cause an error if File.ReadFrom would use the slow path.  Default is false.
//
// refer to [File.ReadFrom]
func PreventSlowReadFrom(prevent bool) ClientOption {
	return func(c *Client) error {
		c.preventSlowReadFrom = prevent
		return nil
	}
}

// Client represents an SFTP session on a *ssh.ClientConn SSH connection.
// Multiple Clients can be active on a single SSH connection, and a Client
// may be called concurrently from multiple Goroutines.
//
// Client implements the github.com/kr/fs.FileSystem interface.
type Client struct {
	conn clientConn_

	respPool sync.Pool // of resp chans

	ext map[string]string // Extensions (name -> data).

	maxPacket int // max packet size read or written.

	preventSlowReadFrom bool
}

// Create a new SFTP client on the SSH conn
func NewClient(conn *ssh.Client, opts ...ClientOption) (*Client, error) {
	s, err := conn.NewSession()
	if err != nil {
		return nil, err
	}
	if err := s.RequestSubsystem("sftp"); err != nil {
		return nil, err
	}
	pw, err := s.StdinPipe()
	if err != nil {
		return nil, err
	}
	pr, err := s.StdoutPipe()
	if err != nil {
		return nil, err
	}

	return NewClientPipe(pr, pw, opts...)
}

// Create a new SFTP client with the Reader and a WriteCloser.
// This can be used for connecting to an SFTP server over TCP/TLS or by using
// the system's ssh client program (e.g. via exec.Command).
func NewClientPipe(
	rd io.Reader,
	wr io.WriteCloser,
	opts ...ClientOption,
) (
	client *Client,
	err error,
) {
	client = &Client{
		maxPacket: 1 << 15, // 32768, min supported as per RFC
	}
	client.respPool.New = client.newResponder

	defer func() {
		if err != nil {
			wr.Close()
		}
	}()

	for _, opt := range opts {
		err = opt(client)
		if err != nil {
			return
		}
	}

	client.conn.Construct(rd, wr, client.maxPacket)

	client.ext, err = client.conn.Start()

	return
}

type errResponder_ struct {
	c      chan error
	client *Client
}

func (r *errResponder_) onError(err error) { r.c <- err }
func (r *errResponder_) await() (err error) {
	err = <-r.c
	r.client.respPool.Put(r)
	return
}

func (c *Client) newResponder() any {
	return &errResponder_{
		c:      make(chan error, 1),
		client: c,
	}
}
func (c *Client) responder() *errResponder_ {
	return c.respPool.Get().(*errResponder_)
}

// https://filezilla-project.org/specs/draft-ietf-secsh-filexfer-02.txt
const sftpProtocolVersion = 3

// Check whether the server supports a named extension.
//
// The first return value is the extension data reported by the server
// (typically a version number).
func (c *Client) HasExtension(name string) (string, bool) {
	data, ok := c.ext[name]
	return data, ok
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// Walk returns a new Walker rooted at root.
//func (c *Client) Walk(root string) *fs.Walker {
//	return fs.WalkFS(root, c)
//}

type ReadDirFilter func(fileN string, attrs *FileStat) (allow, stop bool)

type ReadDirLimit struct {
	N int
}

func (rdl *ReadDirLimit) Filter(fileN string, attrs *FileStat) (allow, stop bool) {
	if 0 != rdl.N {
		rdl.N--
		return true, false
	}
	return false, true
}

// ReadDir get a list of Files in dirN.
func (c *Client) ReadDir(
	dirN string,
	timeout time.Duration, // if positive, limit time to read dir
	filter ReadDirFilter, // if not nil, filter entries
) (
	entries []*File,
	err error,
) {
	var deadline time.Time
	if 0 < timeout {
		deadline = time.Now().Add(timeout)
	}

	handle, err := c.opendir(timeout, dirN)
	if err != nil {
		return
	}
	defer c.closeHandleAsync(handle, nil, nil)

	if 0 < timeout && time.Now().After(deadline) {
		return
	}

	responder := c.responder()

	var readdirF func(id, length uint32, typ uint8) (err error)
	readdirF = func(id, length uint32, typ uint8) (err error) {
		done := false
		defer func() {
			if !done && nil == err &&
				(0 >= timeout || !time.Now().After(deadline)) {
				err = c.conn.RequestSingle(
					&sshFxpReaddirPacket{Handle: handle},
					sshFxpName, true,
					readdirF,
					responder.onError)
			}
			if done || nil != err {
				responder.onError(err)
			}
		}()
		switch typ {
		case sshFxpName:
			err = c.conn.ensure(int(length))
			if err != nil {
				return
			}
			allow := true
			count, buff := unmarshalUint32(c.conn.buff)
			for i := uint32(0); i < count && !done; i++ {
				var fileN string
				fileN, buff = unmarshalString(buff)
				_, buff = unmarshalString(buff) // discard longname
				var attrs *FileStat
				attrs, buff, err = unmarshalAttrs(buff)
				if err != nil {
					return
				}
				if nil != filter {
					allow, done = filter(fileN, attrs)
				}
				if fileN == "." || fileN == ".." || !allow {
					continue
				}
				entries = append(entries, &File{
					c:     c,
					pathN: path.Join(dirN, fileN),
					attrs: *attrs,
				})
			}
		case sshFxpStatus:
			err = maybeError(c.conn.buff) // may be nil
			if 0 != len(entries) || io.EOF == err {
				err = nil // ignore any unmarshaled error if we have entries
			}
			done = true
		default:
			panic("impossible!")
		}
		return
	}

	err = c.conn.RequestSingle(
		&sshFxpReaddirPacket{Handle: handle},
		sshFxpName, true,
		readdirF,
		responder.onError)
	if err != nil {
		return
	}
	err = responder.await()
	return
}

func (c *Client) opendir(
	timeout time.Duration,
	dirN string,
) (
	handle string,
	err error,
) {
	err = c.invokeExpect(
		&sshFxpOpendirPacket{Path: dirN},
		sshFxpHandle,
		func() error {
			handle, _ = unmarshalString(c.conn.buff)
			return nil
		})
	return
}

// response to an async call
type AsyncResponse struct {
	Request any   // request info provided by caller
	Error   error // result (nil == success), failure (not nil)
}

// async call expecting a status response
func (c *Client) asyncExpectStatus(
	pkt idAwarePkt_,
	onStatus func(error), // if not nil, call before dispatching to respC
	request any, // any request data to be returned with response - may be nil
	respC chan *AsyncResponse, // if nil, then toss any response
) (err error) {
	return c.asyncExpect(pkt, 0, nil, onStatus, request, respC)
}

// async call expecting a single response, either the expectType or status
func (c *Client) asyncExpect(
	pkt idAwarePkt_,
	expectType uint8,
	onExpect func() (err error),
	onStatus func(error),
	request any, // any request data to be returned with response
	respC chan *AsyncResponse, // if nil, then toss any response
) error {
	const errUnexpected = uerr.Const("Unexpected packet type 0")

	resp := &AsyncResponse{Request: request}
	return c.conn.RequestSingle(
		pkt, expectType, true,
		func(id, length uint32, typ uint8) error {
			defer func() {
				if nil != onStatus {
					onStatus(resp.Error)
				}
				if nil != respC {
					respC <- resp
				}
			}()
			resp.Error = c.conn.ensure(int(length))
			if resp.Error != nil {
				return nil
			}
			switch typ {
			case expectType:
				if nil != onExpect {
					resp.Error = onExpect()
				} else {
					resp.Error = errUnexpected
				}
			case sshFxpStatus:
				resp.Error = maybeError(c.conn.buff) // may be nil
			default:
				panic("impossible!")
			}
			return nil
		},
		func(err error) {
			resp.Error = err
			respC <- resp
		})
}

// perform invocation expecting a single response, either the expectType or status
func (c *Client) invokeExpect(
	pkt idAwarePkt_,
	expectType uint8,
	onExpect func() error,
) (err error) {
	responder := c.responder()
	err = c.conn.RequestSingle(
		pkt, expectType, false,
		func(id, length uint32, typ uint8) (err error) {
			err = c.conn.ensure(int(length))
			if err != nil {
				return
			}
			switch typ {
			case expectType:
				err = onExpect()
			case sshFxpStatus:
				err = maybeError(c.conn.buff) // may be nil
			default:
				panic("impossible!")
			}
			return nil
		},
		responder.onError)
	if err != nil {
		return
	}
	err = responder.await()
	return
}

// invoke when expected resp is just a status
func (c *Client) invokeExpectStatus(pkt idAwarePkt_) (err error) {
	responder := c.responder()
	err = c.conn.RequestSingle(
		pkt, sshFxpStatus, false,
		func(id, length uint32, typ uint8) (err error) {
			err = c.conn.ensure(int(length))
			if err != nil {
				return
			}
			switch typ {
			case sshFxpStatus:
				err = maybeError(c.conn.buff) // may be nil
			default:
				panic("impossible!")
			}
			return nil
		},
		responder.onError)
	if err != nil {
		return
	}
	err = responder.await()
	return
}

// returns a FileStat describing the file specified by pathN
// If pathN is a symbolic link, the returned FileStat describes the actual file.
// FileInfoFromStat can be used to convert this to a go os.FileInfo
func (c *Client) Stat(pathN string) (fs *FileStat, err error) {
	return c.stat(pathN)
}

// returns a FileStat describing the file specified by pathN.
// If pathN is a symbolic link, the returned FileStat describes the link, not the
// actual file.
func (c *Client) Lstat(pathN string) (attrs *FileStat, err error) {
	err = c.invokeExpect(
		&sshFxpLstatPacket{Path: pathN},
		sshFxpAttrs,
		func() (err error) {
			attrs, _, err = unmarshalAttrs(c.conn.buff)
			return
		})
	return
}

// ReadLink reads the target of a symbolic link.
func (c *Client) ReadLink(pathN string) (target string, err error) {
	err = c.invokeExpect(
		&sshFxpReadlinkPacket{Path: pathN},
		sshFxpName,
		func() (err error) {
			count, buff := unmarshalUint32(c.conn.buff)
			if count != 1 {
				err = unexpectedCount(1, count)
			} else {
				target, _ = unmarshalString(buff) // ignore dummy attributes
			}
			return
		})
	return
}

// Link creates a hard link at 'newname', pointing at the same inode as 'oldname'
func (c *Client) Link(oldname, newname string) error {
	return c.invokeExpectStatus(
		&sshFxpHardlinkPacket{
			Oldpath: oldname,
			Newpath: newname,
		})
}

// Symlink creates a symbolic link at 'newname', pointing at target 'oldname'
func (c *Client) Symlink(oldname, newname string) error {
	return c.invokeExpectStatus(
		&sshFxpSymlinkPacket{
			Linkpath:   newname,
			Targetpath: oldname,
		})
}

func (c *Client) fsetstat(handle string, flags uint32, attrs any) error {
	return c.invokeExpectStatus(
		&sshFxpFsetstatPacket{
			Handle: handle,
			Flags:  flags,
			Attrs:  attrs,
		})
}

// allow for changing of various parts of the file descriptor.
func (c *Client) setstat(pathN string, flags uint32, attrs any) error {
	return c.invokeExpectStatus(
		&sshFxpSetstatPacket{
			Path:  pathN,
			Flags: flags,
			Attrs: attrs,
		})
}

// Chtimes changes the access and modification times of the named file.
func (c *Client) Chtimes(pathN string, atime time.Time, mtime time.Time) error {
	type times struct {
		Atime uint32
		Mtime uint32
	}
	attrs := times{uint32(atime.Unix()), uint32(mtime.Unix())}
	return c.setstat(pathN, sshFileXferAttrACmodTime, attrs)
}

// Chown changes the user and group owners of the named file.
func (c *Client) Chown(pathN string, uid, gid int) error {
	type owner struct {
		UID uint32
		GID uint32
	}
	attrs := owner{uint32(uid), uint32(gid)}
	return c.setstat(pathN, sshFileXferAttrUIDGID, attrs)
}

// Chmod changes the permissions of the named file.
//
// Chmod does not apply a umask, because even retrieving the umask is not
// possible in a portable way without causing a race condition. Callers
// should mask off umask bits, if desired.
func (c *Client) Chmod(pathN string, mode os.FileMode) error {
	return c.setstat(pathN, sshFileXferAttrPermissions, toChmodPerm(mode))
}

// Truncate sets the size of the named file. Although it may be safely assumed
// that if the size is less than its current size it will be truncated to fit,
// the SFTP protocol does not specify what behavior the server should do when setting
// size greater than the current size.
func (c *Client) Truncate(path string, size int64) error {
	return c.setstat(path, sshFileXferAttrSize, uint64(size))
}

// SetExtendedData sets extended attributes of the named file. It uses the
// SSH_FILEXFER_ATTR_EXTENDED flag in the setstat request.
//
// This flag provides a general extension mechanism for vendor-specific extensions.
// Names of the attributes should be a string of the format "name@domain", where "domain"
// is a valid, registered domain name and "name" identifies the method. Server
// implementations SHOULD ignore extended data fields that they do not understand.
func (c *Client) SetExtendedData(path string, extended []StatExtended) error {
	attrs := &FileStat{
		Extended: extended,
	}
	return c.setstat(path, sshFileXferAttrExtended, attrs)
}

// Create creates the named file mode 0666 (before umask), truncating it if it
// already exists. If successful, methods on the returned File can be used for
// I/O; the associated file descriptor has mode O_RDWR. If you need more
// control over the flags/mode used to open the file see client.OpenFile.
//
// Note that some SFTP servers (eg. AWS Transfer) do not support opening files
// read/write at the same time. For those services you will need to use
// `client.OpenFile(os.O_WRONLY|os.O_CREATE|os.O_TRUNC)`.
func (c *Client) Create(pathN string) (*File, error) {
	return c.open(&File{c: c, pathN: pathN},
		toPflags(os.O_RDWR|os.O_CREATE|os.O_TRUNC))
}

// Open file at pathN for reading.
func (c *Client) OpenRead(pathN string) (*File, error) {
	return c.open(&File{c: c, pathN: pathN}, toPflags(os.O_RDONLY))
}

// Open file at path using the specified flags
func (c *Client) Open(pathN string, flags int) (*File, error) {
	return c.open(&File{c: c, pathN: pathN}, toPflags(flags))
}

func (c *Client) open(f *File, pflags uint32) (rv *File, err error) {
	err = c.invokeExpect(
		&sshFxpOpenPacket{
			Path:   f.pathN,
			Pflags: pflags,
		},
		sshFxpHandle,
		func() error {
			f.handle, _ = unmarshalString(c.conn.buff)
			rv = f
			return nil
		})
	if err != nil {
		err = uerr.Chainf(err, "open %s", f.pathN)
	}
	return
}

func (c *Client) openAsync(
	f *File, pflags uint32, req any, respC chan *AsyncResponse,
) (
	err error,
) {
	err = c.asyncExpect(
		&sshFxpOpenPacket{
			Path:   f.pathN,
			Pflags: pflags,
		},
		sshFxpHandle,
		func() error {
			f.handle, _ = unmarshalString(c.conn.buff)
			return nil
		}, nil, req, respC)
	if err != nil {
		err = uerr.Chainf(err, "openAsync %s", f.pathN)
	}
	return
}

// close a handle handle previously returned in the response
// to SSH_FXP_OPEN or SSH_FXP_OPENDIR. The handle becomes invalid
// immediately after this request has been sent.
func (c *Client) closeHandleAsync(
	handle string,
	req any, // may be nil
	respC chan *AsyncResponse, // my be nil
) error {
	return c.asyncExpectStatus(&sshFxpClosePacket{Handle: handle}, nil, req, respC)
}

// synchronous - waits for any error
func (c *Client) closeHandle(handle string) error {
	return c.invokeExpectStatus(&sshFxpClosePacket{Handle: handle})
}

func (c *Client) stat(path string) (attr *FileStat, err error) {
	err = c.invokeExpect(
		&sshFxpStatPacket{Path: path},
		sshFxpAttrs,
		func() (err error) {
			attr, _, err = unmarshalAttrs(c.conn.buff)
			return
		})
	return
}

func (c *Client) fstat(handle string) (attr *FileStat, err error) {
	err = c.invokeExpect(
		&sshFxpFstatPacket{Handle: handle},
		sshFxpAttrs,
		func() (err error) {
			attr, _, err = unmarshalAttrs(c.conn.buff)
			return
		})
	return
}

// get VFS (file system) statistics from a remote host.
//
// Implement the statvfs@openssh.com SSH_FXP_EXTENDED feature from
// http://www.opensource.apple.com/source/OpenSSH/OpenSSH-175/openssh/PROTOCOL?txt.
func (c *Client) StatVFS(pathN string) (rv *StatVFS, err error) {
	err = c.invokeExpect(
		&sshFxpStatvfsPacket{Path: pathN},
		sshFxpExtendedReply,
		func() (err error) {
			rv = &StatVFS{}
			err = binary.Read(bytes.NewReader(c.conn.buff), binary.BigEndian, rv)
			if err != nil {
				rv = nil
				err = errors.New("can not parse StatVFS reply")
			}
			return
		})
	return
}

// Remove removes the specified file or directory. An error will be returned if no
// file or directory with the specified path exists, or if the specified directory
// is not empty.
func (c *Client) Remove(pathN string) error {
	err := c.removeFile(pathN)
	// some servers, *cough* osx *cough*, return EPERM, not ENODIR.
	// serv-u returns ssh_FX_FILE_IS_A_DIRECTORY
	// EPERM is converted to os.ErrPermission so it is not a StatusError
	if err, ok := err.(*StatusError); ok {
		switch err.Code {
		case sshFxFailure, sshFxFileIsADirectory:
			return c.RemoveDirectory(pathN)
		}
	}
	if os.IsPermission(err) {
		return c.RemoveDirectory(pathN)
	}
	return err
}

func (c *Client) removeFile(pathN string) error {
	return c.invokeExpectStatus(&sshFxpRemovePacket{Filename: pathN})
}

func (c *Client) RemoveAsync(
	pathN string, req any, respC chan *AsyncResponse,
) error {
	return c.asyncExpectStatus(
		&sshFxpRemovePacket{Filename: pathN},
		nil, req, respC)
}

// RemoveDirectory removes a directory path.
func (c *Client) RemoveDirectory(pathN string) error {
	return c.invokeExpectStatus(&sshFxpRmdirPacket{Path: pathN})
}

// Rename renames a file.
func (c *Client) Rename(oldN, newN string) error {
	return c.invokeExpectStatus(
		&sshFxpRenamePacket{
			Oldpath: oldN,
			Newpath: newN,
		})
}

func (c *Client) RenameAsync(
	oldN, newN string,
	req any, respC chan *AsyncResponse,
) (err error) {
	return c.asyncExpectStatus(
		&sshFxpRenamePacket{
			Oldpath: oldN,
			Newpath: newN,
		}, nil, req, respC)
}

// PosixRename renames a file using the posix-rename@openssh.com extension
// which will replace newname if it already exists.
func (c *Client) PosixRename(oldN, newN string) error {
	return c.invokeExpectStatus(
		&sshFxpPosixRenamePacket{
			Oldpath: oldN,
			Newpath: newN,
		})
}

// PosixRename renames a file using the posix-rename@openssh.com extension
// which will replace newname if it already exists.
func (c *Client) PosixRenameAsync(
	oldN, newN string,
	req any, respC chan *AsyncResponse,
) (err error) {
	return c.asyncExpectStatus(
		&sshFxpPosixRenamePacket{
			Oldpath: oldN,
			Newpath: newN,
		}, nil, req, respC)
}

// Request server to canonicalize pathN to an absolute path.
//
// This is useful for converting path names containing ".." components,
// or relative pathnames without a leading slash into absolute paths.
func (c *Client) RealPath(pathN string) (canonN string, err error) {
	err = c.invokeExpect(
		&sshFxpRealpathPacket{Path: pathN},
		sshFxpName,
		func() (err error) {
			count, buff := unmarshalUint32(c.conn.buff)
			if count != 1 {
				err = unexpectedCount(1, count)
				return
			}
			canonN, _ = unmarshalString(buff) // ignore attributes
			return
		})
	return
}

// Getwd returns the current working directory of the server. Operations
// involving relative paths will be based at this location.
func (c *Client) Getwd() (string, error) {
	return c.RealPath(".")
}

// Mkdir creates the specified directory. An error will be returned if a file or
// directory with the specified path already exists, or if the directory's
// parent folder does not exist (the method cannot create complete paths).
func (c *Client) Mkdir(path string) error {
	return c.invokeExpectStatus(&sshFxpMkdirPacket{Path: path})
}

/*
// MkdirAll creates a directory named path, along with any necessary parents,
// and returns nil, or else returns an error.
// If path is already a directory, MkdirAll does nothing and returns nil.
// If path contains a regular file, an error is returned
func (c *Client) MkdirAll(path string) error {
	// Most of this code mimics https://golang.org/src/os/path.go?s=514:561#L13
	// Fast path: if we can tell whether path is a directory or file, stop with success or error.
	dir, err := c.Stat(path)
	if err == nil {
		if dir.IsDir() {
			return nil
		}
		return &os.PathError{Op: "mkdir", Path: path, Err: syscall.ENOTDIR}
	}

	// Slow path: make sure parent exists and then call Mkdir for path.
	i := len(path)
	for i > 0 && path[i-1] == '/' { // Skip trailing path separator.
		i--
	}

	j := i
	for j > 0 && path[j-1] != '/' { // Scan backward over element.
		j--
	}

	if j > 1 {
		// Create parent
		err = c.MkdirAll(path[0 : j-1])
		if err != nil {
			return err
		}
	}

	// Parent now exists; invoke Mkdir and use its result.
	err = c.Mkdir(path)
	if err != nil {
		// Handle arguments like "foo/." by
		// double-checking that directory doesn't exist.
		dir, err1 := c.Lstat(path)
		if err1 == nil && dir.IsDir() {
			return nil
		}
		return err
	}
	return nil
}

// RemoveAll delete files recursively in the directory and Recursively delete subdirectories.
// An error will be returned if no file or directory with the specified path exists
func (c *Client) RemoveAll(path string) error {

	// Get the file/directory information
	fi, err := c.Stat(path)
	if err != nil {
		return err
	}

	if fi.IsDir() {
		// Delete files recursively in the directory
		files, err := c.ReadDir(path)
		if err != nil {
			return err
		}

		for _, file := range files {
			if file.IsDir() {
				// Recursively delete subdirectories
				err = c.RemoveAll(path + "/" + file.Name())
				if err != nil {
					return err
				}
			} else {
				// Delete individual files
				err = c.Remove(path + "/" + file.Name())
				if err != nil {
					return err
				}
			}
		}

	}

	return c.Remove(path)

}
*/

// Provide access to a remote file.
//
// Files obtained via Client.ReadDir are not in an open state.  They must be opened
// first.  These Files do have populated attributes.
//
// Files obtained via Client.Open calls are open, but do not have populated
// attributes until Stat() is called.
//
// Calls that change the offset (Read/ReadFrom/Write/WriteTo/Seek) need to be
// externally coordinated or synchronized.  This is no different than dealing
// with any other kind of file, as concurrent reads and writes will result in
// gibberish otherwise.
//
// Likewise, Open/Close needs to also be externally coordinated or synchronized
// with other i/o ops.
type File struct {
	c      *Client
	pathN  string
	handle string   // empty if not open
	offset int64    // current offset within remote file
	attrs  FileStat // if Mode bits not set, then not populated
}

const ErrOpenned = uerr.Const("file already openned")

// normally create with client.Open or client.ReadDir
func NewFile(client *Client, pathN string) *File {
	return &File{
		c:     client,
		pathN: pathN,
	}
}

func (f *File) IsOpen() bool { return 0 != len(f.handle) }

func (f *File) Client() *Client { return f.c }

// if File is not currently open, it is possible to change the Client
func (f *File) SetClient(c *Client) error {
	if 0 != len(f.handle) {
		return ErrOpenned
	}
	f.c = c
	return nil
}

// return cached FileStat, which may not be populated with file attributes.
//
// if Mode bits are zero, then it is not populated.
//
// it will be populated after a ReadDir, or a Stat call
func (f *File) FileStat() FileStat { return f.attrs }

// if attrs are populated, mod time in unix seconds
//
// it's only 32 bits, but it's unsigned so will not fail in 2038
func (f *File) ModTimeUnix() uint32 { return f.attrs.Mtime }

// careful - this creates a time.Time each invocation
func (f *File) ModTime() time.Time { return time.Unix(int64(f.attrs.Mtime), 0) }

// if attrs are populated, mode bits of file.  otherwise, bits are zero.
func (f *File) Mode() FileMode { return f.attrs.FileMode() }

// return true if attributes are populated
func (f *File) AttrsCached() bool { return 0 != f.attrs.Mode }

// if attrs are populated, size of the file
func (f *File) Size() uint64 { return f.attrs.Size }

// if attrs are populated, check if this is regular file
func (f *File) IsRegular() bool { return f.attrs.IsRegular() }

// if attrs are populated, check if this is a dir
func (f *File) IsDir() bool { return f.attrs.IsDir() }

// return the name of the file as presented to Open or Create.
func (f *File) Name() string { return f.pathN }

// change the name - useful after AsyncRename
func (f *File) SetName(newN string) { f.pathN = newN }

// return the base name of the file
func (f *File) BaseName() string { return path.Base(f.pathN) }

// Open the file for read.
//
// async safe
func (f *File) OpenRead() (err error) {
	if 0 != len(f.handle) {
		return ErrOpenned
	}
	_, err = f.c.open(f, toPflags(os.O_RDONLY))
	return
}

// Open the file for read, async.
//
// async safe
func (f *File) OpenReadAsync(request any, respC chan *AsyncResponse) (err error) {
	if 0 != len(f.handle) {
		return ErrOpenned
	}
	err = f.c.openAsync(f, toPflags(os.O_RDONLY), request, respC)
	return
}

// Open file using the specified flags
//
// async safe
func (f *File) Open(flags int) (err error) {
	if 0 != len(f.handle) {
		return ErrOpenned
	}
	_, err = f.c.open(f, toPflags(flags))
	return
}

// Open the file, async.
//
// async safe
func (f *File) OpenAsync(flags int, req any, respC chan *AsyncResponse) (err error) {
	if 0 != len(f.handle) {
		return ErrOpenned
	}
	err = f.c.openAsync(f, toPflags(flags), req, respC)
	return
}

// implement io.Closer
//
// close the File.
//
// syncronize access
func (f *File) Close() error {
	if 0 == len(f.handle) {
		return nil
	}
	handle := f.handle
	f.handle = ""
	return f.c.closeHandle(handle)
}

// close the File, async.
//
// syncronize access
func (f *File) CloseAsync(request any, respC chan *AsyncResponse) error {
	if 0 == len(f.handle) {
		return nil
	}
	handle := f.handle
	f.handle = ""
	return f.c.closeHandleAsync(handle, request, respC)
}

// remove the file.  it may remain open.
//
// async safe
func (f *File) Remove() (err error) {
	return f.c.Remove(f.pathN)
}

// remove the file, async.  it may remain open.
//
// async safe
func (f *File) RemoveAsync(req any, respC chan *AsyncResponse) (err error) {
	return f.c.RemoveAsync(f.pathN, req, respC)
}

// rename file.
//
// synchronize access
func (f *File) Rename(newN string) (err error) {
	err = f.c.Rename(f.pathN, newN)
	if err != nil {
		return
	}
	f.pathN = newN
	return
}

// Rename file, but only if it doesn't already exist.
//
// synchronize access
func (f *File) RenameAsync(newN string, req any, respC chan *AsyncResponse) error {
	return f.c.asyncExpectStatus(
		&sshFxpRenamePacket{
			Oldpath: f.pathN,
			Newpath: newN,
		},
		func(status error) {
			if nil == status { // success
				f.pathN = newN
			}
		},
		req, respC)
}

// rename file, even if newN already exists (replacing it).
//
// uses the posix-rename@openssh.com extension
//
// synchronize access
func (f *File) PosixRename(newN string) (err error) {
	err = f.c.PosixRename(f.pathN, newN)
	if err != nil {
		return
	}
	f.pathN = newN
	return
}

// rename file, async, even if newN already exists (replacing it).
//
// uses the posix-rename@openssh.com extension
//
// synchronize access
func (f *File) PosixRenameAsync(
	newN string, req any, respC chan *AsyncResponse,
) error {
	return f.c.asyncExpectStatus(
		&sshFxpPosixRenamePacket{
			Oldpath: f.pathN,
			Newpath: newN,
		},
		func(status error) {
			if nil == status { // success
				f.pathN = newN
			}
		},
		req, respC)
}

// implement io.WriterTo
//
// copy contents (from current offset to end) of file to w
//
// If file is not built from ReadDir, then Stat must be called on it before
// making this call to ensure the size is known.
//
// synchronize i/o ops
func (f *File) WriteTo(w io.Writer) (written int64, err error) {

	const errStat = uerr.Const("file has no attrs - run Stat prior to WriteTo")

	if 0 == f.attrs.Mode {
		err = errStat
		return
	}
	amount := int64(f.attrs.Size) - f.offset
	if amount <= 0 {
		return
	}

	pkt := sshFxpReadPacket{}
	chunkSz, lastChunkSz, req := f.buildReadReq(amount, f.offset, &pkt)
	conn := &f.c.conn
	responder := f.c.responder()
	req.onError = responder.onError
	expectPkts := req.expectPkts

	first := true
	var expectId uint32
	req.onResp = func(id, length uint32, typ uint8) (err error) {
		defer func() {
			if err != nil || 0 == expectPkts {
				expectPkts = 0 // ignore any remaining pkts
				responder.onError(err)
			}
		}()
		if 0 == expectPkts {
			return // ignore any pkts after error
		}
		expectPkts--

		//
		// detect out of order reads being returned by server
		//
		if first {
			first = false
			expectId = id
		} else if id != expectId {
			err = fmt.Errorf("WriteTo expecting pkt %d, got %d", expectId, id)
			return
		}
		expectId++

		switch typ {
		case sshFxpData:
			//
			// our next chunk of data
			//
			err = conn.ensure(4)
			if err != nil {
				return
			}
			dataSz, buff := unmarshalUint32(conn.buff)
			length -= 4
			if dataSz != length {
				err = fmt.Errorf("dataSz is %d, but remaining is %d!", dataSz,
					length)
				return
			} else if (0 != expectPkts && length != chunkSz) ||
				(0 == expectPkts && length != lastChunkSz) {
				exp := chunkSz
				if 0 == expectPkts {
					exp = lastChunkSz
				}
				err = fmt.Errorf(
					"only got %d of %d bytes - may need to adjust MaxPacket",
					length, exp)
				return
			}
			if 0 == length {
				return
			}
			//
			// use up any already read by conn
			//
			var nwrote int
			if 0 != len(buff) {
				if int(length) < len(buff) {
					buff = buff[:length]
				}
				nwrote, err = w.Write(buff)
				written += int64(nwrote)
				if err != nil || int(length) == len(buff) {
					return // success if done
				}
				length -= uint32(len(buff))
			}

			//
			// copy the rest from the conn to the w
			//
			buff = conn.backing[:]
			for 0 != length {
				if int(length) < len(buff) {
					buff = buff[:length]
				}
				_, err = io.ReadFull(conn.r, buff)
				if err != nil {
					return
				}
				nwrote, err = w.Write(buff)
				written += int64(nwrote)
				if err != nil {
					return
				}
				length -= uint32(len(buff))
			}

		case sshFxpStatus:
			err = maybeError(conn.buff) // may be nil
		default:
			panic("impossible!")
		}
		return
	}

	err = conn.Request(req)
	if err != nil {
		return
	}
	err = responder.await()
	if err != nil {
		return
	}
	f.offset += amount
	return
}

// when reading from sftp server, we have to obey the maxPacket limit.
//
// if we request more bytes that that limit, then it will just return a
// truncated amount.
//
// therefore, we split up requests larger than that into chunks using the
// nextPkt closure to manufacture reqs as needed by the conn writer.
func (f *File) buildReadReq(
	amount, offset int64,
	single *sshFxpReadPacket,
) (
	chunkSz, lastChunkSz uint32,
	req *clientReq_,
) {
	maxPkt := int64(f.c.maxPacket)
	expectPkts := amount / maxPkt
	if amount != expectPkts*maxPkt {
		if 0 == expectPkts {
			chunkSz = uint32(amount)
			lastChunkSz = chunkSz
		} else {
			chunkSz = uint32(maxPkt)
			lastChunkSz = uint32(amount - expectPkts*maxPkt)
		}
		expectPkts++
	}

	req = &clientReq_{
		expectType: sshFxpData,
		noAutoResp: true,
		expectPkts: uint32(expectPkts),
	}
	single.Handle = f.handle
	if 1 == expectPkts {
		single.Offset = uint64(offset)
		single.Len = chunkSz
		req.pkt = single
		req.expectPkts = 1
		return
	}

	req.nextPkt = func(id uint32) idAwarePkt_ {
		single.ID = id
		single.Offset = uint64(offset)
		offset += int64(chunkSz)
		expectPkts--
		if 0 == expectPkts {
			single.Len = lastChunkSz
		} else {
			single.Len = chunkSz
		}
		return single
	}
	return
}

// implement io.ReaderAt
//
// synchronize i/o ops
func (f *File) ReadAt(toBuff []byte, offset int64) (nread int, err error) {
	const errMissing = uerr.Const(
		"previous read was short, but this was not - missing data")

	if 0 == len(toBuff) {
		return
	}

	pkt := sshFxpReadPacket{}
	chunkSz, lastChunkSz, req := f.buildReadReq(int64(len(toBuff)), offset, &pkt)
	conn := &f.c.conn
	responder := f.c.responder()
	req.onError = responder.onError
	expectPkts := req.expectPkts //len(req.pkts)

	first := true
	var expectId uint32
	lastShort := false
	req.onResp = func(id, length uint32, typ uint8) (err error) {
		defer func() {
			if err != nil || 0 == expectPkts {
				expectPkts = 0 // ignore any others after error
				responder.onError(err)
			}
		}()
		if 0 == expectPkts {
			return // ignore any pkts after error
		}
		expectPkts--

		//
		// detect out of order reads being returned by server
		//
		if first {
			first = false
			expectId = id
		} else if id != expectId {
			err = fmt.Errorf("WriteTo expecting pkt %d, got %d", expectId, id)
			return
		}
		expectId++

		switch typ {
		case sshFxpData:
			//
			// our next chunk of data
			//
			// which could be less than requested (even 0) if we're at the EOF
			//
			err = conn.ensure(4)
			if err != nil {
				return
			}
			dataSz, buff := unmarshalUint32(conn.buff)
			length -= 4
			if dataSz != length {
				err = fmt.Errorf("dataSz is %d, but remaining is %d!",
					dataSz, length)
				return
			} else if (0 != expectPkts && length != chunkSz) ||
				(0 == expectPkts && length != lastChunkSz) {
				if 0 == length {
					if 0 == nread {
						err = io.EOF
					}
					expectPkts = 0 // ignore any other pkts
					return
				} else if lastShort {
					exp := chunkSz
					if 0 == expectPkts {
						exp = lastChunkSz
					}
					err = fmt.Errorf(
						"only got %d of %d bytes - may need to adjust MaxPacket",
						length, exp)
					return
				}
				lastShort = true
			} else if lastShort {
				err = errMissing
				return
			}
			if 0 == length {
				return
			}
			//
			// use up any already read by conn
			//
			if 0 != len(buff) {
				if int(length) < len(buff) {
					buff = buff[:length]
				}
				ncopied := copy(toBuff, buff)
				nread += ncopied
				if ncopied == len(toBuff) {
					return // success
				}
				toBuff = toBuff[ncopied:]
				length -= uint32(ncopied)
			}

			//
			// copy the rest from the conn to the w
			//
			buff = toBuff
			for 0 != length {
				if int(length) < len(buff) {
					buff = buff[:length]
				}
				var ncopied int
				ncopied, err = io.ReadFull(conn.r, buff)
				nread += ncopied
				if err != nil || ncopied == len(toBuff) {
					return
				}
				toBuff = toBuff[ncopied:]
				length -= uint32(ncopied)
			}

		case sshFxpStatus:
			err = maybeError(conn.buff) // may be nil
		default:
			panic("impossible!")
		}
		return
	}

	err = conn.Request(req)
	if err != nil {
		return
	}
	err = responder.await()
	return
}

// implement io.Reader
//
// Reads up to len(b) bytes from the File. It returns the number of bytes
// read and an error, if any. When Read encounters an error or EOF condition after
// successfully reading n > 0 bytes, it returns the number of bytes read.
//
// The read may be broken up into chunks supported by the server.
//
// If transfering to an io.Writer, use WriteTo for best performance.  io.Copy
// will do this automatically.
//
// synchronize i/o ops
func (f *File) Read(b []byte) (nread int, err error) {
	nread, err = f.ReadAt(b, f.offset)
	f.offset += int64(nread)
	return
}

// Stat returns the attributes about the file.  If the file is open, then fstat
// is used, otherwise, stat is used.  The attributes cached in this File will
// be updated.  To avoid a round trip with the server, use the already cached
// FileStat.
//
// synchronize i/o ops
func (f *File) Stat() (attrs *FileStat, err error) {

	if 0 == len(f.handle) {
		attrs, err = f.c.stat(f.pathN)
	} else {
		attrs, err = f.c.fstat(f.handle)
	}
	if err != nil {
		return
	}
	f.attrs = *attrs
	return
}

// implement io.ReaderFrom
//
// This is fast as long as r (the io.Reader) has one of these methods:
//
//	Len()  int
//	Size() int64
//	Stat() (os.FileInfo, error)
//
// or is an instance of [io.LimitedReader].  The following are known to match:
//   - bytes.Buffer
//   - bytes.Reader
//   - strings.Reader
//   - os.File
//   - io.LimitedReader
//
// Otherwise, this call will be slow, since we won't know the amount that is
// to be transferred and will need to make one i/o at a time.
//
// If you need to prevent the slow path from occuring, use the
// PreventSlowReadFrom ClientOption.
//
// synchronize i/o ops
func (f *File) ReadFrom(r io.Reader) (ncopied int64, err error) {

	if 0 == len(f.handle) {
		return 0, os.ErrClosed
	}

	//
	// we need to know the amount we'll be reading up front, as we need to be
	// able to tell the conn.writer and conn.reader how many packets to expect,
	// so that we can pump data to the sftp server while getting back acks.
	//
	remain, limited := surmiseRemaining(r)
	if !limited {
		return f.readFromSlow(r)
	} else if 0 == remain {
		return 0, nil
	}

	maxPacket := f.c.maxPacket
	expectPkts := remain / int64(maxPacket)
	if remain != expectPkts*int64(maxPacket) {
		expectPkts++
	}

	responder := f.c.responder()
	req := &clientReq_{
		expectType: sshFxpStatus,
		noAutoResp: true,
		onError:    responder.onError,
		expectPkts: uint32(expectPkts),
	}
	pkt := sshFxpWritePacket{Handle: f.handle}

	conn := &f.c.conn
	eof := false
	var sendErr error
	req.nextPkt = func(id uint32) idAwarePkt_ {
		// must only be accessed from conn.writer context
		dataBuff := conn.GetBuffForReadFrom()
		pkt.ID = id
		if eof { // need to flush remaining expected pkts
			pkt.Offset = uint64(f.offset)
			pkt.Length = 0
			pkt.Data = dataBuff[:0]
			return &pkt
		}
		amount, err := r.Read(dataBuff)
		if err != nil {
			if io.EOF != err {
				sendErr = err
			}
			eof = true
		}
		if amount > maxPacket {
			panic("impossible! - dataBuff somehow larger than maxPacket")
		}
		ncopied += int64(amount)
		pkt.Offset = uint64(f.offset)
		f.offset += int64(amount)
		pkt.Length = uint32(amount)
		pkt.Data = dataBuff[:amount]
		return &pkt
	}

	req.onResp = func(id, length uint32, typ uint8) (err error) {
		expectPkts--
		if 0 > expectPkts {
			panic("got back too many packets for ReadFrom!")
		}
		switch typ {
		case sshFxpStatus:
			err = maybeError(conn.buff) // may be nil
		default:
			panic("impossible!")
		}
		if 0 == expectPkts { // all done
			responder.onError(err)
		}
		return
	}

	err = conn.Request(req)
	if err != nil {
		return
	}
	err = responder.await()
	if err != nil {
		return
	}
	if nil == err {
		err = sendErr
	}
	return
}

// this is slow because we have to wait for round trip of each write.
// however, if the reader only has a buffer or two worth of data, then ok.
func (f *File) readFromSlow(r io.Reader) (ncopied int64, err error) {

	if f.c.preventSlowReadFrom {
		return 0, errors.New("attempt to use File.ReadFrom with slow Reader")
	}

	maxPacket := f.c.maxPacket
	conn := &f.c.conn
	pkt := sshFxpWritePacket{Handle: f.handle}
	eof := false
	var sendErr error

	for !eof {
		responder := f.c.responder()
		req := &clientReq_{
			expectType: sshFxpStatus,
			noAutoResp: true,
			onError:    responder.onError,
			expectPkts: 1,
		}

		req.nextPkt = func(id uint32) idAwarePkt_ {
			// must only be accessed from conn.writer context
			dataBuff := conn.GetBuffForReadFrom()
			pkt.ID = id
			amount, err := r.Read(dataBuff)
			if err != nil {
				if io.EOF != err {
					sendErr = err
				}
				eof = true
			}
			if amount > maxPacket {
				panic("impossible! - dataBuff somehow larger than maxPacket")
			}
			ncopied += int64(amount)
			pkt.Offset = uint64(f.offset)
			f.offset += int64(amount)
			pkt.Length = uint32(amount)
			pkt.Data = dataBuff[:amount]
			return &pkt
		}

		req.onResp = func(id, length uint32, typ uint8) (err error) {
			switch typ {
			case sshFxpStatus:
				err = maybeError(conn.buff) // may be nil
			default:
				panic("impossible!")
			}
			responder.onError(err)
			return
		}

		err = conn.Request(req)
		if err != nil {
			return
		}
		err = responder.await()
		if err != nil {
			return
		}
	}
	if nil == err {
		err = sendErr
	}
	return
}

func surmiseRemaining(r io.Reader) (remain int64, limited bool) {
	//
	// we need to know the amount we'll be reading up front, as we need to be
	// able to tell the conn.writer and conn.reader how many packets to expect,
	// so that we can pump data to the sftp server while getting back acks.
	//
	switch r := r.(type) {
	case interface{ Len() int }:
		limited = true
		remain = int64(r.Len())

	case interface{ Size() int64 }:
		limited = true
		remain = r.Size()

	case *io.LimitedReader:
		remain, limited = surmiseRemaining(r.R) // need to dig deeper
		if !limited || remain > r.N {
			limited = true
			remain = r.N
		}

	case interface{ Stat() (os.FileInfo, error) }:
		limited = true
		info, err := r.Stat()
		if err == nil {
			remain = info.Size()
		}
	}
	return
}

// implement io.Writer
//
// synchronize i/o ops
func (f *File) Write(b []byte) (nwrote int, err error) {

	if 0 == len(f.handle) {
		return 0, os.ErrClosed
	}
	nwrote, err = f.WriteAt(b, f.offset)
	f.offset += int64(nwrote)
	return
}

// implement io.WriterAt
//
// synchronize i/o ops
func (f *File) WriteAt(b []byte, offset int64) (written int, err error) {

	if 0 == len(f.handle) {
		return 0, os.ErrClosed
	} else if 0 == len(b) {
		return
	}

	responder := f.c.responder()

	maxPacket := f.c.maxPacket
	expectPkts := len(b) / maxPacket
	if len(b) != expectPkts*maxPacket {
		expectPkts++
	}

	req := &clientReq_{
		expectType: sshFxpStatus,
		noAutoResp: true,
		onError:    responder.onError,
		expectPkts: uint32(expectPkts),
	}
	pkt := sshFxpWritePacket{Handle: f.handle}

	req.nextPkt = func(id uint32) idAwarePkt_ {
		pkt.ID = id
		amount := len(b)
		if 0 == amount {
			return nil
		} else if amount > maxPacket {
			amount = maxPacket
		}
		written += amount
		pkt.Offset = uint64(offset)
		offset += int64(amount)
		pkt.Length = uint32(amount)
		pkt.Data = b
		b = b[amount:]
		return &pkt
	}

	conn := &f.c.conn

	req.onResp = func(id, length uint32, typ uint8) (err error) {
		expectPkts--
		if 0 > expectPkts {
			return errors.New("got back too many packets for write!")
		}
		switch typ {
		case sshFxpStatus:
			err = maybeError(conn.buff) // may be nil
		default:
			panic("impossible!")
		}
		if 0 == expectPkts { // all done
			responder.onError(err)
		}
		return
	}

	err = conn.Request(req)
	if err != nil {
		return
	}
	err = responder.await()
	return
}

// implement io.Seeker
//
// Set the offset for the next Read or Write. Return the next offset.
//
// Seeking before or after the end of the file is undefined.
//
// Seeking relative to the end will call Stat if file has no cached attributes,
// otherwise, it will use the cached attributes.
//
// synchronize i/o ops
func (f *File) Seek(offset int64, whence int) (int64, error) {
	if 0 == len(f.handle) {
		return 0, os.ErrClosed
	}

	switch whence {
	case io.SeekStart:
	case io.SeekCurrent:
		offset += f.offset
	case io.SeekEnd:
		if 0 == f.attrs.Mode {
			_, err := f.Stat()
			if err != nil {
				return f.offset, err
			}
		}
		offset += int64(f.attrs.Size)
	default:
		return f.offset, unimplementedSeekWhence(whence)
	}

	if offset < 0 {
		return f.offset, os.ErrInvalid
	}

	f.offset = offset
	return f.offset, nil
}

// Change the uid/gid of the current file.
//
// async safe
func (f *File) Chown(uid, gid int) error {
	fs := &FileStat{
		UID: uint32(uid),
		GID: uint32(gid),
	}
	if 0 == len(f.handle) {
		return f.c.setstat(f.pathN, sshFileXferAttrUIDGID, fs)
	} else {
		return f.c.fsetstat(f.handle, sshFileXferAttrUIDGID, fs)
	}
}

// Change the permissions of the current file.
//
// See Client.Chmod for details.
func (f *File) Chmod(mode os.FileMode) error {
	if 0 == len(f.handle) {
		return f.c.setstat(f.pathN, sshFileXferAttrPermissions, toChmodPerm(mode))
	} else {
		return f.c.fsetstat(f.handle, sshFileXferAttrPermissions, toChmodPerm(mode))
	}
}

// SetExtendedData sets extended attributes of the current file. It uses the
// SSH_FILEXFER_ATTR_EXTENDED flag in the setstat request.
//
// This flag provides a general extension mechanism for vendor-specific extensions.
// Names of the attributes should be a string of the format "name@domain", where "domain"
// is a valid, registered domain name and "name" identifies the method. Server
// implementations SHOULD ignore extended data fields that they do not understand.
//
// async safe
func (f *File) SetExtendedData(path string, extended []StatExtended) error {
	attrs := &FileStat{Extended: extended}
	if 0 == len(f.handle) {
		return f.c.setstat(f.pathN, sshFileXferAttrExtended, attrs)
	} else {
		return f.c.fsetstat(f.handle, sshFileXferAttrExtended, attrs)
	}
}

// Truncate sets the size of the current file. Although it may be safely assumed
// that if the size is less than its current size it will be truncated to fit,
// the SFTP protocol does not specify what behavior the server should do when setting
// size greater than the current size.
//
// async safe
func (f *File) Truncate(size int64) error {

	if 0 == len(f.handle) {
		return f.c.setstat(f.pathN, sshFileXferAttrSize, uint64(size))
	} else {
		return f.c.fsetstat(f.handle, sshFileXferAttrSize, uint64(size))
	}
}

// Request a flush of the contents of a File to stable storage.
//
// Sync requires the server to support the fsync@openssh.com extension.
//
// async safe
func (f *File) Sync() error {
	if 0 == len(f.handle) {
		return os.ErrClosed
	}
	return f.c.invokeExpectStatus(&sshFxpFsyncPacket{Handle: f.handle})
}

// Asynchronously request a flush of the contents of a File to stable storage.
//
// Requires the server to support the fsync@openssh.com extension.
//
// async safe
func (f *File) SyncAsync(req any, respC chan *AsyncResponse) error {
	if 0 == len(f.handle) {
		return os.ErrClosed
	}
	return f.c.asyncExpectStatus(
		&sshFxpFsyncPacket{Handle: f.handle}, nil, req, respC)
}

// convert ssh/sftp status/errors into stdlib errors, or to nil if not an error
func maybeError(buff []byte) error {
	err := unmarshalStatus(buff).(*StatusError)
	switch err.Code {
	case sshFxEOF:
		return io.EOF
	case sshFxNoSuchFile:
		return os.ErrNotExist
	case sshFxPermissionDenied:
		return os.ErrPermission
	case sshFxOk:
		return nil
	default:
		return err
	}
}

// flags converts the flags passed to OpenFile into ssh flags.
// Unsupported flags are ignored.
func toPflags(f int) uint32 {
	var out uint32
	switch f & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR) {
	case os.O_RDONLY:
		out |= sshFxfRead
	case os.O_WRONLY:
		out |= sshFxfWrite
	case os.O_RDWR:
		out |= sshFxfRead | sshFxfWrite
	}
	if f&os.O_APPEND == os.O_APPEND {
		out |= sshFxfAppend
	}
	if f&os.O_CREATE == os.O_CREATE {
		out |= sshFxfCreat
	}
	if f&os.O_TRUNC == os.O_TRUNC {
		out |= sshFxfTrunc
	}
	if f&os.O_EXCL == os.O_EXCL {
		out |= sshFxfExcl
	}
	return out
}

// toChmodPerm converts Go permission bits to POSIX permission bits.
//
// This differs from fromFileMode in that we preserve the POSIX versions of
// setuid, setgid and sticky in m, because we've historically supported those
// bits, and we mask off any non-permission bits.
func toChmodPerm(m os.FileMode) (perm uint32) {
	const mask = os.ModePerm | os.FileMode(s_ISUID|s_ISGID|s_ISVTX)
	perm = uint32(m & mask)

	if m&os.ModeSetuid != 0 {
		perm |= s_ISUID
	}
	if m&os.ModeSetgid != 0 {
		perm |= s_ISGID
	}
	if m&os.ModeSticky != 0 {
		perm |= s_ISVTX
	}

	return perm
}
