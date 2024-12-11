package usftp

import (
	"bytes"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/tredeske/u/uerr"
	"golang.org/x/crypto/ssh"
)

// https://filezilla-project.org/specs/draft-ietf-secsh-filexfer-02.txt
const sftpProtocolVersion = 3

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
	return func(client *Client) error {
		if size < 8192 {
			return errors.New("maxPacket must be greater or equal to 8192")
		}
		client.maxPacket = size
		return nil
	}
}

// Cause an error if File.ReadFrom would use the slow path.  Default is false.
//
// refer to [File.ReadFrom]
func WithoutSlowReadFrom(without bool) ClientOption {
	return func(client *Client) error {
		client.withoutSlowReadFrom = without
		return nil
	}
}

// Add a func to receive async error notifications.  These can occur if the
// connection to the SFTP server is lost, for example.
//
// The func should not perform time consuming operations.  One or more invocations
// may occur on connection failure.
func WithErrorFunc(onError func(error)) ClientOption {
	return func(client *Client) error {
		client.onError = onError
		return nil
	}
}

// Client represents an SFTP session on a *ssh.ClientConn SSH connection.
// Multiple Clients can be active on a single SSH connection, and a Client
// may be called concurrently from multiple Goroutines.
type Client struct {
	conn clientConn_

	respPool sync.Pool // of resp chans

	ext map[string]string // Extensions (name -> data).

	maxPacket int // max packet size read or written.

	withoutSlowReadFrom bool

	onError func(error)
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

	for i := range opts {
		err = opts[i](client)
		if err != nil {
			return
		}
	}

	client.conn.Construct(rd, wr, client)
	client.ext, err = client.conn.Start()
	return
}

type errResponder_ struct {
	errC   chan error
	client *Client
}

func (er *errResponder_) onError(err error) { er.errC <- err }
func (er *errResponder_) await() (err error) {
	err = <-er.errC
	er.client.respPool.Put(er)
	return
}

func (client *Client) newResponder() any {
	return &errResponder_{
		errC:   make(chan error, 1),
		client: client,
	}
}

func (client *Client) responder() *errResponder_ {
	return client.respPool.Get().(*errResponder_)
}

func (client *Client) reportError(err error) {
	if nil != client.onError {
		client.onError(err)
	}
}

// Check whether the server supports a named extension.
//
// The first return value is the extension data reported by the server
// (typically a version number).
func (client *Client) HasExtension(name string) (string, bool) {
	data, ok := client.ext[name]
	return data, ok
}

// close connection to SFTP server and cease operation
func (client *Client) Close() error {
	return client.conn.Close()
}

// return fs.FS compliant facade
func (client *Client) FS() fs.FS {
	return &fsClient_{client: client}
}

// see fs.WalkDir
func (client *Client) WalkDir(root string, f fs.WalkDirFunc) error {
	return fs.WalkDir(client.FS(), root, f)
}

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
func (client *Client) ReadDir(dirN string) ([]*File, error) {
	return client.ReadDirLimit(dirN, 0, nil)
}

// ReadDir get a list of Files in dirN.
func (client *Client) ReadDirLimit(
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

	handle, err := client.opendir(timeout, dirN)
	if err != nil {
		return
	}
	defer client.closeHandleAsync(handle, nil, nil)

	if 0 < timeout && time.Now().After(deadline) {
		return
	}

	responder := client.responder()

	var readdirF func(id, length uint32, typ uint8) (err error)
	readdirF = func(id, length uint32, typ uint8) (err error) {
		done := false
		defer func() {
			if !done && nil == err &&
				(0 >= timeout || !time.Now().After(deadline)) {
				err = client.conn.RequestSingle(
					&sshFxpReaddirPacket{Handle: handle},
					sshFxpName, manualRespond_,
					readdirF,
					responder.onError)
			}
			if done || nil != err {
				responder.onError(err)
			}
		}()
		switch typ {
		case sshFxpName:
			err = client.conn.ensure(int(length))
			if err != nil {
				return
			}
			allow := true
			count, buff := unmarshalUint32(client.conn.buff)
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
					client: client,
					pathN:  path.Join(dirN, fileN),
					attrs:  *attrs,
				})
			}
		case sshFxpStatus:
			err = maybeError(client.conn.buff) // may be nil
			if 0 != len(entries) || io.EOF == err {
				err = nil // ignore any unmarshaled error if we have entries
			}
			done = true
		default:
			panic("impossible!")
		}
		return
	}

	err = client.conn.RequestSingle(
		&sshFxpReaddirPacket{Handle: handle},
		sshFxpName, manualRespond_,
		readdirF,
		responder.onError)
	if err != nil {
		return
	}
	err = responder.await()
	return
}

func (client *Client) opendir(
	timeout time.Duration,
	dirN string,
) (
	handle string,
	err error,
) {
	err = client.invokeExpect(
		&sshFxpOpendirPacket{Path: dirN},
		sshFxpHandle,
		func() error {
			handle, _ = unmarshalString(client.conn.buff)
			return nil
		})
	return
}

// Callback invoked upon completion of an async operation.
//
// This is useful to avoid waiting for responses before starting the next
// operation.  For example, a pipeline where a file is opened async, could feed
// a chan to a worker that then writes to the open file and async closes it, which
// then feeds a chan for a worker that gets the close disposition.
//
// Any work performed in the callback should be brief and non blocking, offloading
// any time consuming or blocking work to a separate goroutine.  This callback
// will be called in the event loop of the connection reader, so delaying return
// delays reading the next message.
//
// If nil, then the async operation is "fire and forget".  This is useful (for
// example) after closing a File that is open for reading, but dangerous (for
// example) after closing a file that is open for writing.
//
// The req is provided by the caller as "callback data".
//
// The error is the disposition of the async operation.  If nil, then the operation
// was successful.
type AsyncFunc func(req any, err error)

// async call expecting a status response
func (client *Client) asyncExpectStatus(
	pkt idAwarePkt_,
	onStatus func(error), // if not nil, call before dispatching to respC
	req any, // onComplete req
	onComplete AsyncFunc,
) (err error) {
	return client.asyncExpect(pkt, 0, nil, onStatus, req, onComplete)
}

// async call expecting a single response, either the expectType or status
func (client *Client) asyncExpect(
	pkt idAwarePkt_,
	expectType uint8,
	onExpect func() (err error),
	onStatus func(error),
	req any, // onComplete req
	onComplete AsyncFunc,
) error {
	const errUnexpected = uerr.Const("Unexpected packet type 0")

	return client.conn.RequestSingle(
		pkt, expectType, manualRespond_,
		func(id, length uint32, typ uint8) error {
			var respErr error
			defer func() {
				if nil != onStatus {
					onStatus(respErr)
				}
				if nil != onComplete {
					onComplete(req, respErr)
				}
			}()
			switch typ {
			case expectType:
				respErr = client.conn.ensure(int(length))
				if respErr != nil {
					return respErr
				}
				if nil != onExpect {
					respErr = onExpect()
				} else {
					respErr = errUnexpected
				}
			case sshFxpStatus:
				respErr = maybeError(client.conn.buff) // may be nil
			default:
				panic("impossible!")
			}
			return nil
		},
		func(err error) {
			if nil != onComplete {
				onComplete(req, err)
			}
		})
}

// perform invocation expecting a single response, either the expectType or status
func (client *Client) invokeExpect(
	pkt idAwarePkt_,
	expectType uint8,
	onExpect func() error,
) (err error) {
	responder := client.responder()
	err = client.conn.RequestSingle(
		pkt, expectType, autoRespond_,
		func(id, length uint32, typ uint8) (err error) {
			switch typ {
			case expectType:
				err = onExpect()
			case sshFxpStatus:
				err = maybeError(client.conn.buff) // may be nil
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
func (client *Client) invokeExpectStatus(pkt idAwarePkt_) (err error) {
	return client.invokeExpect(pkt, 0, nil) // there is no type 0
}

// Return a FileStat describing the file specified by pathN
// If pathN is a symbolic link, the returned FileStat describes the actual file.
// FileInfoFromStat can be used to convert this to a go os.FileInfo
func (client *Client) Stat(pathN string) (fs *FileStat, err error) {
	return client.stat(pathN)
}

// Return a FileStat describing the file specified by pathN.
// If pathN is a symbolic link, the returned FileStat describes the link, not the
// actual file.
func (client *Client) Lstat(pathN string) (attrs *FileStat, err error) {
	err = client.invokeExpect(
		&sshFxpLstatPacket{Path: pathN},
		sshFxpAttrs,
		func() (err error) {
			attrs, _, err = unmarshalAttrs(client.conn.buff)
			return
		})
	return
}

// Read the target of a symbolic link (resolve to actual file/dir).
func (client *Client) ReadLink(pathN string) (target string, err error) {
	err = client.invokeExpect(
		&sshFxpReadlinkPacket{Path: pathN},
		sshFxpName,
		func() (err error) {
			count, buff := unmarshalUint32(client.conn.buff)
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
func (client *Client) Link(oldname, newname string) error {
	return client.invokeExpectStatus(
		&sshFxpHardlinkPacket{
			Oldpath: oldname,
			Newpath: newname,
		})
}

// Symlink creates a symbolic link at 'newname', pointing at target 'oldname'
func (client *Client) Symlink(oldname, newname string) error {
	return client.invokeExpectStatus(
		&sshFxpSymlinkPacket{
			Linkpath:   newname,
			Targetpath: oldname,
		})
}

func (client *Client) fsetstat(handle string, flags uint32, attrs any) error {
	return client.invokeExpectStatus(
		&sshFxpFsetstatPacket{
			Handle: handle,
			Flags:  flags,
			Attrs:  attrs,
		})
}

// allow for changing of various parts of the file descriptor.
func (client *Client) setstat(pathN string, flags uint32, attrs any) error {
	return client.invokeExpectStatus(
		&sshFxpSetstatPacket{
			Path:  pathN,
			Flags: flags,
			Attrs: attrs,
		})
}

// Change the access and modification times of the named file.
func (client *Client) Chtimes(pathN string, atime time.Time, mtime time.Time) error {
	type times struct {
		Atime uint32
		Mtime uint32
	}
	attrs := times{uint32(atime.Unix()), uint32(mtime.Unix())}
	return client.setstat(pathN, sshFileXferAttrACmodTime, attrs)
}

// Chown changes the user and group owners of the named file.
func (client *Client) Chown(pathN string, uid, gid int) error {
	type owner struct {
		UID uint32
		GID uint32
	}
	attrs := owner{uint32(uid), uint32(gid)}
	return client.setstat(pathN, sshFileXferAttrUIDGID, attrs)
}

// Change the permissions of the named file.
//
// No umask will be applied. Because even retrieving the umask is not
// possible in a portable way without causing a race condition. Callers
// should mask off umask bits, if desired.
func (client *Client) Chmod(pathN string, mode os.FileMode) error {
	return client.setstat(pathN, sshFileXferAttrPermissions, toChmodPerm(mode))
}

// Set the size of the named file. Setting a size smaller than the current size
// causes file truncation.  Setting a size greater than the current size is
// not defined by SFTP - the server may grow the file or do something else.
func (client *Client) Truncate(path string, size int64) error {
	return client.setstat(path, sshFileXferAttrSize, uint64(size))
}

// Set extended attributes of the named file, using the
// SSH_FILEXFER_ATTR_EXTENDED flag in the setstat request.
//
// This flag provides a general extension mechanism for vendor-specific extensions.
// Names of the attributes should be a string of the format "name@domain", where
// "domain" is a valid, registered domain name and "name" identifies the method.
// Server implementations SHOULD ignore extended data fields that they do not
// understand.
func (client *Client) SetExtendedAttr(path string, extended []StatExtended) error {
	attrs := &FileStat{
		Extended: extended,
	}
	return client.setstat(path, sshFileXferAttrExtended, attrs)
}

// Create the named file mode 0666 (before umask), truncating it if it
// already exists. If successful, methods on the returned File can be used for
// I/O; the associated file descriptor has mode O_RDWR. If you need more
// control over the flags/mode used to open the file see client.OpenFile.
//
// Note that some SFTP servers (eg. AWS Transfer) do not support opening files
// read/write at the same time. For those services you will need to use
// `client.OpenFile(os.O_WRONLY|os.O_CREATE|os.O_TRUNC)`.
func (client *Client) Create(pathN string) (*File, error) {
	return client.open(
		&File{client: client, pathN: pathN},
		toPflags(os.O_RDWR|os.O_CREATE|os.O_TRUNC))
}

// Create the file, async.
func (client *Client) CreateAsync(
	pathN string, req any, onComplete AsyncFunc,
) (f *File, err error) {
	f = &File{client: client, pathN: pathN}
	return f, client.openAsync(f,
		toPflags(os.O_RDWR|os.O_CREATE|os.O_TRUNC),
		req, onComplete)
}

// Open file at pathN for reading.
func (client *Client) OpenRead(pathN string) (*File, error) {
	return client.open(
		&File{client: client, pathN: pathN},
		toPflags(os.O_RDONLY))
}

// Open file at pathN for reading, async.
func (client *Client) OpenReadAsync(
	pathN string, req any, onComplete AsyncFunc,
) (f *File, err error) {
	f = &File{client: client, pathN: pathN}
	return f, client.openAsync(f, toPflags(os.O_RDONLY), req, onComplete)
}

// Open file at path using the specified flags
func (client *Client) Open(pathN string, flags int) (*File, error) {
	return client.open(
		&File{client: client, pathN: pathN},
		toPflags(flags))
}

// Open file at path using the specified flags, async
func (client *Client) OpenAsync(
	pathN string, flags int, req any, onComplete AsyncFunc,
) (f *File, err error) {
	f = &File{client: client, pathN: pathN}
	return f, client.openAsync(f, toPflags(flags), req, onComplete)
}

func (client *Client) open(f *File, pflags uint32) (rv *File, err error) {
	err = client.invokeExpect(
		&sshFxpOpenPacket{Path: f.pathN, Pflags: pflags},
		sshFxpHandle,
		func() error {
			f.handle, _ = unmarshalString(client.conn.buff)
			rv = f
			return nil
		})
	if err != nil {
		err = uerr.Chainf(err, "open %s", f.pathN)
	}
	return
}

func (client *Client) openAsync(
	f *File, pflags uint32, req any, onComplete AsyncFunc,
) (
	err error,
) {
	err = client.asyncExpect(
		&sshFxpOpenPacket{Path: f.pathN, Pflags: pflags},
		sshFxpHandle,
		func() error {
			f.handle, _ = unmarshalString(client.conn.buff)
			return nil
		}, nil, req, onComplete)
	if err != nil {
		err = uerr.Chainf(err, "openAsync %s", f.pathN)
	}
	return
}

// Close a handle handle previously returned in the response
// to SSH_FXP_OPEN or SSH_FXP_OPENDIR. The handle becomes invalid
// immediately after this request has been sent.
func (client *Client) closeHandleAsync(
	handle string,
	req any, // may be nil
	onComplete AsyncFunc,
) error {
	return client.asyncExpectStatus(&sshFxpClosePacket{Handle: handle}, nil,
		req, onComplete)
}

// synchronous - waits for any error
func (client *Client) closeHandle(handle string) error {
	return client.invokeExpectStatus(&sshFxpClosePacket{Handle: handle})
}

func (client *Client) stat(path string) (attr *FileStat, err error) {
	err = client.invokeExpect(
		&sshFxpStatPacket{Path: path},
		sshFxpAttrs,
		func() (err error) {
			attr, _, err = unmarshalAttrs(client.conn.buff)
			return
		})
	return
}

func (client *Client) fstat(handle string) (attr *FileStat, err error) {
	err = client.invokeExpect(
		&sshFxpFstatPacket{Handle: handle},
		sshFxpAttrs,
		func() (err error) {
			attr, _, err = unmarshalAttrs(client.conn.buff)
			return
		})
	return
}

// get VFS (file system) statistics from a remote host.
//
// Implement the statvfs@openssh.com SSH_FXP_EXTENDED feature from
// http://www.opensource.apple.com/source/OpenSSH/OpenSSH-175/openssh/PROTOCOL?txt.
func (client *Client) StatVFS(pathN string) (rv *StatVFS, err error) {
	err = client.invokeExpect(
		&sshFxpStatvfsPacket{Path: pathN},
		sshFxpExtendedReply,
		func() (err error) {
			rv = &StatVFS{}
			err = binary.Read(bytes.NewReader(client.conn.buff), binary.BigEndian, rv)
			if err != nil {
				rv = nil
				err = errors.New("can not parse StatVFS reply")
			}
			return
		})
	return
}

// Remove pathN.  Return error if pathN does not exist, or if pathN is a non-empty
// directory.
func (client *Client) Remove(pathN string) error {
	err := client.removeFile(pathN)
	// some servers, *cough* osx *cough*, return EPERM, not ENODIR.
	// serv-u returns ssh_FX_FILE_IS_A_DIRECTORY
	// EPERM is converted to os.ErrPermission so it is not a StatusError
	if err, ok := err.(*StatusError); ok {
		switch err.Code {
		case sshFxFailure, sshFxFileIsADirectory:
			return client.RemoveDirectory(pathN)
		}
	}
	if os.IsPermission(err) {
		return client.RemoveDirectory(pathN)
	}
	return err
}

func (client *Client) removeFile(pathN string) error {
	return client.invokeExpectStatus(&sshFxpRemovePacket{Filename: pathN})
}

func (client *Client) RemoveAsync(
	pathN string,
	req any, // onComplete request
	onComplete AsyncFunc,
) error {
	return client.asyncExpectStatus(
		&sshFxpRemovePacket{Filename: pathN},
		nil, req, onComplete)
}

// RemoveDirectory removes a directory path.
func (client *Client) RemoveDirectory(pathN string) error {
	return client.invokeExpectStatus(&sshFxpRmdirPacket{Path: pathN})
}

// Rename oldN to newN, error if newN exists.
func (client *Client) Rename(oldN, newN string) error {
	return client.invokeExpectStatus(
		&sshFxpRenamePacket{Oldpath: oldN, Newpath: newN})
}

// Rename oldN to newN, async.
func (client *Client) RenameAsync(
	oldN, newN string,
	req any, // onComplete request
	onComplete AsyncFunc,
) (err error) {
	return client.asyncExpectStatus(
		&sshFxpRenamePacket{Oldpath: oldN, Newpath: newN}, nil, req, onComplete)
}

// Rename oldN to newN, replacing newN if it exists.
//
// Uses the posix-rename@openssh.com extension.
func (client *Client) PosixRename(oldN, newN string) error {
	return client.invokeExpectStatus(
		&sshFxpPosixRenamePacket{Oldpath: oldN, Newpath: newN})
}

// Rename oldN to newN, replacing newN if it exists, async.
//
// Uses the posix-rename@openssh.com extension.
func (client *Client) PosixRenameAsync(
	oldN, newN string,
	req any, // onComplete request
	onComplete AsyncFunc,
) (err error) {
	return client.asyncExpectStatus(
		&sshFxpPosixRenamePacket{Oldpath: oldN, Newpath: newN},
		nil, req, onComplete)
}

// Request server to canonicalize pathN to an absolute path.
//
// This is useful for converting path names containing ".." components,
// or relative pathnames without a leading slash into absolute paths.
func (client *Client) RealPath(pathN string) (canonN string, err error) {
	err = client.invokeExpect(
		&sshFxpRealpathPacket{Path: pathN},
		sshFxpName,
		func() (err error) {
			count, buff := unmarshalUint32(client.conn.buff)
			if count != 1 {
				err = unexpectedCount(1, count)
				return
			}
			canonN, _ = unmarshalString(buff) // ignore attributes
			return
		})
	return
}

// Return the current working directory of the server. Operations
// involving relative paths will be based at this location.
func (client *Client) Getwd() (string, error) {
	return client.RealPath(".")
}

// Create the specified directory. An error will be returned if a file or
// directory with the specified path already exists, or if the directory's
// parent folder does not exist (the method cannot create complete paths).
func (client *Client) Mkdir(dirN string) error {
	return client.invokeExpectStatus(&sshFxpMkdirPacket{Path: dirN})
}

// Create the dirN directory, along with any necessary parents.
// If dirN exists and is a directory, do nothing and return nil.
// If dirN exists and is not a directory, return error.
func (client *Client) MkdirAll(dirN string) (err error) {
	if 0 == len(dirN) || "." == dirN || "/" == dirN {
		return // no reason to create root or current dir
	}
	// "" will clean to ".", as will ".", as will "./", as will "foo/.."
	dirN = path.Clean(dirN)
	if "/" == dirN || "." == dirN {
		return // no reason to create root or current dir
	}

	// if already exists, we either have nothing to do, or cannot continue
	stat, err := client.Stat(dirN)
	if nil == err {
		if stat.IsDir() {
			return // no reason to recreate dir
		}
		return &os.PathError{Op: "mkdir", Path: dirN, Err: syscall.ENOTDIR}
	}

	err = client.MkdirAll(path.Dir(dirN)) // ensure existence of parent(s)
	if err != nil {
		return
	}
	return client.Mkdir(dirN)
}

// Delete dirN and all files (if dirN is a dir) of any kind contained in dirN.
//
// Return error if dirN does not exist or is not readable, or if a file is not
// removable.
func (client *Client) RemoveAll(dirN string) (err error) {

	dirN = path.Clean(dirN)

	// Get the file/directory information
	stat, err := client.Stat(dirN)
	if err != nil {
		return
	}

	if stat.IsDir() { // Delete files recursively in the directory
		var files []*File
		files, err = client.ReadDir(dirN)
		if err != nil {
			return
		}

		for _, file := range files {
			if file.IsDir() { // Recursively delete subdirectories
				err = client.RemoveAll(file.Name())
				if err != nil {
					return
				}
			} else { // Delete individual files
				err = client.Remove(file.Name())
				if err != nil {
					return
				}
			}
		}
	}
	return client.Remove(dirN)
}

// Delete all files contained in dirN, but do not delete dirN.
//
// Return error if dirN is not readable or not a dir, or if a file is not
// removable.
func (client *Client) RemoveAllIn(dirN string) (err error) {

	dirN = path.Clean(dirN)

	files, err := client.ReadDir(dirN)
	if err != nil {
		return
	}

	for _, file := range files {
		if file.IsDir() { // Recursively delete subdirectories
			err = client.RemoveAll(file.Name())
			if err != nil {
				return
			}
		} else { // Delete individual files
			err = client.Remove(file.Name())
			if err != nil {
				return
			}
		}
	}
	return
}

// convert ssh/sftp status/errors into stdlib errors, or to nil if not an error
func maybeError(buff []byte) error {
	status := unmarshalStatus(buff).(*StatusError)
	switch status.Code {
	case sshFxEOF:
		return io.EOF
	case sshFxNoSuchFile:
		return os.ErrNotExist
	case sshFxPermissionDenied:
		return os.ErrPermission
	case sshFxOk:
		return nil
	default:
		return status
	}
}

// Convert the flags passed to OpenFile into ssh flags.
// Unsupported flags are ignored.
func toPflags(flags int) (rv uint32) {
	switch flags & (os.O_RDONLY | os.O_WRONLY | os.O_RDWR) {
	case os.O_RDONLY:
		rv |= sshFxfRead
	case os.O_WRONLY:
		rv |= sshFxfWrite
	case os.O_RDWR:
		rv |= sshFxfRead | sshFxfWrite
	}
	if flags&os.O_APPEND == os.O_APPEND {
		rv |= sshFxfAppend
	}
	if flags&os.O_CREATE == os.O_CREATE {
		rv |= sshFxfCreat
	}
	if flags&os.O_TRUNC == os.O_TRUNC {
		rv |= sshFxfTrunc
	}
	if flags&os.O_EXCL == os.O_EXCL {
		rv |= sshFxfExcl
	}
	return
}

// Convert Go permission bits to POSIX permission bits.
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
