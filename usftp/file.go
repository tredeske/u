package usftp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"time"

	"github.com/tredeske/u/uerr"
)

const ErrOpenned = uerr.Const("file already openned")

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
	client *Client
	pathN  string
	handle string   // empty if not open
	offset int64    // current offset within remote file
	attrs  FileStat // if Mode bits not set, then not populated
	Stash  any      // stash whatever you want here
}

// normally create with client.Open or client.ReadDir
func NewFile(client *Client, pathN string) *File {
	return &File{
		client: client,
		pathN:  pathN,
	}
}

func (f *File) IsOpen() bool { return 0 != len(f.handle) }

func (f *File) Client() *Client { return f.client }

// if File is not currently open, it is possible to change the Client
func (f *File) SetClient(c *Client) error {
	if 0 != len(f.handle) {
		return ErrOpenned
	}
	f.client = c
	return nil
}

// return cached FileStat, which may not be populated with file attributes.
//
// if Mode bits are zero, then it is not populated.
//
// it will be populated after a ReadDir, or a Stat call
func (f *File) FileStat() FileStat { return f.attrs }

// if attrs are populated, mod time in unix serespConds
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

// change the name
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
	_, err = f.client.open(f, toPflags(os.O_RDONLY))
	return
}

// Open the file for read, async.
//
// async safe
func (f *File) OpenReadAsync(req any, onComplete AsyncFunc) (err error) {
	if 0 != len(f.handle) {
		return ErrOpenned
	}
	err = f.client.openAsync(f, toPflags(os.O_RDONLY), req, onComplete)
	return
}

// Open file using the specified flags
//
// async safe
func (f *File) Open(flags int) (err error) {
	if 0 != len(f.handle) {
		return ErrOpenned
	}
	_, err = f.client.open(f, toPflags(flags))
	return
}

// Open the file, async.
//
// async safe
func (f *File) OpenAsync(flags int, req any, onComplete AsyncFunc) (err error) {
	if 0 != len(f.handle) {
		return ErrOpenned
	}
	err = f.client.openAsync(f, toPflags(flags), req, onComplete)
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
	return f.client.closeHandle(handle)
}

// close the File, async.
//
// Use nil for request and respC to "fire and forget".  This is useful when
// closing after an error encountered or for done reading, but dangerous after
// a successful write, as it is possible the write is not 100% complete and a
// failure is detected during close.
//
// syncronize access
func (f *File) CloseAsync(req any, onComplete AsyncFunc) error {
	if 0 == len(f.handle) {
		return nil
	}
	handle := f.handle
	f.handle = ""
	return f.client.closeHandleAsync(handle, req, onComplete)
}

// remove the file.  it may remain open.
//
// async safe
func (f *File) Remove() (err error) {
	return f.client.Remove(f.pathN)
}

// remove the file, async.  it may remain open.
//
// async safe
func (f *File) RemoveAsync(req any, onComplete AsyncFunc) error {
	return f.client.RemoveAsync(f.pathN, req, onComplete)
}

// rename file.
//
// synchronize access
func (f *File) Rename(newN string) (err error) {
	err = f.client.Rename(f.pathN, newN)
	if err != nil {
		return
	}
	f.pathN = newN
	return
}

// Rename file, but only if it doesn't already exist.
//
// synchronize access
func (f *File) RenameAsync(newN string, req any, onComplete AsyncFunc) error {
	return f.client.asyncExpectStatus(
		&sshFxpRenamePacket{
			Oldpath: f.pathN,
			Newpath: newN,
		},
		func(status error) {
			if nil == status { // success
				f.pathN = newN
			}
		},
		req, onComplete)
}

// rename file, even if newN already exists (replacing it).
//
// uses the posix-rename@openssh.com extension
//
// synchronize access
func (f *File) PosixRename(newN string) (err error) {
	err = f.client.PosixRename(f.pathN, newN)
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
func (f *File) PosixRenameAsync(newN string, req any, onComplete AsyncFunc) error {
	return f.client.asyncExpectStatus(
		&sshFxpPosixRenamePacket{
			Oldpath: f.pathN,
			Newpath: newN,
		},
		func(status error) {
			if nil == status { // success
				f.pathN = newN
			}
		},
		req, onComplete)
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

	chunkSz, lastChunkSz, req := f.buildReadReq(amount, f.offset)
	conn := &f.client.conn
	responder := f.client.responder()
	req.onError = responder.onError
	req.autoResp = manualRespond_
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
) (
	chunkSz, lastChunkSz uint32,
	req *clientReq_,
) {
	maxPkt := int64(f.client.maxPacket)
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

	req = f.client.request()
	req.expectType = sshFxpData
	req.autoResp = manualRespond_
	req.expectPkts = uint32(expectPkts)
	if 1 == expectPkts {
		pkt := &sshFxpReadPacket{
			Handle: f.handle,
			Offset: uint64(offset),
			Len:    chunkSz,
		}
		req.pkt = pkt
		req.expectPkts = 1
		return
	}

	req.pumpPkts =
		func(id uint32, conn *conn_, buff []byte) (nsent uint32, err error) {
			pkt := sshFxpReadPacket{
				Handle: f.handle,
			}
			pos := 0
			for 0 != expectPkts {
				pos += 4
				pkt.ID = id
				id++
				pkt.Offset = uint64(offset)
				offset += int64(chunkSz)
				expectPkts--
				if 0 == expectPkts {
					pkt.Len = lastChunkSz
				} else {
					pkt.Len = chunkSz
				}
				b, _ := pkt.appendTo(buff[pos:pos])
				length := len(b)
				binary.BigEndian.PutUint32(buff[pos-4:], uint32(length))
				pos += length
				nsent++
				if 0 == expectPkts || len(buff) < pos+4+length {
					_, err = conn.w.Write(buff[:pos])
					if err != nil {
						return
					}
					pos = 0
				}
			}
			return
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

	chunkSz, lastChunkSz, req := f.buildReadReq(int64(len(toBuff)), offset)
	conn := &f.client.conn
	responder := f.client.responder()
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
// read and an error, if any. When Read encounters an error or EOF respCondition after
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
		attrs, err = f.client.stat(f.pathN)
	} else {
		attrs, err = f.client.fstat(f.handle)
	}
	if err != nil {
		return
	}
	f.attrs = *attrs
	return
}

// implement io.ReaderFrom
//
// Copy from io.Reader into this file starting at current offset.
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
// WithoutSlowReadFrom ClientOption.
//
// synchronize i/o ops
func (f *File) ReadFrom(r io.Reader) (nread int64, err error) {

	if 0 == len(f.handle) {
		return 0, os.ErrClosed
	}

	//
	// we need to know the amount we'll be reading up front, as we need to be
	// able to tell the conn.writer and conn.reader how many packets to expect,
	// so that we can pump data to the sftp server while getting back acks.
	//
	//var remain int64
	remain, limited := surmiseRemaining(r)
	if !limited {
		return f.readFromSlow(r)
	} else if 0 == remain {
		return 0, nil
	}

	maxPacket := f.client.maxPacket
	expectPkts := remain / int64(maxPacket)
	if remain != expectPkts*int64(maxPacket) {
		expectPkts++
	}

	responder := f.client.responder()
	req := f.client.request()
	req.expectType = sshFxpStatus
	req.autoResp = manualRespond_
	req.onError = responder.onError
	req.expectPkts = uint32(expectPkts)

	// conn still ok after readErr, but we need to report to our caller
	var readErr error
	req.pumpPkts =
		func(id uint32, conn *conn_, buff []byte) (nsent uint32, err error) {
			packetsToSend := expectPkts // need our own counter in closure
			pkt := sshFxpWritePacket{Handle: f.handle}
			length := pkt.sizeBeforeData()
			for 0 != packetsToSend {
				packetsToSend--
				pkt.ID = id
				id++
				amount := maxPacket
				if remain < int64(amount) {
					amount = int(remain)
				}
				pkt.Offset = uint64(f.offset)

				readB := buff[4+length : 4+length+amount]
				var readAmount int
				readAmount, readErr = io.ReadFull(r, readB)
				nread += int64(readAmount)
				if readErr != nil {
					if readErr != io.ErrUnexpectedEOF {
						return
					}
					//
					// we ran out of data before expectation met, so someone
					// likely was fibbing to us about the amount.
					//
					readErr = nil
					if 0 == readAmount {
						return
					}
				}
				pkt.Length = uint32(readAmount)
				bigEnd_.PutUint32(buff, uint32(length+readAmount))
				pkt.appendTo(buff[4:4])

				_, err = conn.w.Write(buff[:4+length+readAmount])
				if err != nil {
					return
				}
				f.offset += int64(readAmount)
				nsent++
				if amount != readAmount { // from io.ErrUnexpectedEOF, above
					return
				}
			}
			return
		}

	conn := &f.client.conn
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

	err = f.client.conn.Request(req)
	if err != nil {
		return
	}
	err = responder.await()
	if err != nil {
		if errReqTrunc_ == err {
			err = nil // we didn't get as much as expected, but that's ok
		}
	}
	if nil == err {
		err = readErr
	}
	return
}

// this may not be slow, but it is more complicated and resource intensive.
// it requires sending multple reqs to the conn.reader.
func (f *File) readFromSlow(r io.Reader) (nread int64, err error) {

	if f.client.withoutSlowReadFrom {
		return 0, errors.New("attempt to use File.ReadFrom with slow Reader")
	}

	var respLock sync.Mutex
	var respAcks, respPkts uint32
	var respSendDone bool
	var respErr error
	respCond := sync.Cond{L: &respLock}

	onError := func(err error) {
		respLock.Lock()
		respErr = err
		respLock.Unlock()
		respCond.Signal()
	}

	conn := &f.client.conn
	onResp := func(id, length uint32, typ uint8) (err error) {
		if typ == sshFxpStatus {
			var signal bool
			err = maybeError(conn.buff) // may be nil
			respLock.Lock()
			if nil == err {
				respAcks++
				if respSendDone && respAcks == respPkts {
					respErr = errReqTrunc_
					signal = true
				}
			} else {
				respErr = err
				signal = true
			}
			respLock.Unlock()
			if signal {
				respCond.Signal()
			}
		}
		return
	}

	//
	// send 1 req at a time, but only the first req goes to the conn.writer
	//
	req := f.client.request()
	req.expectType = sshFxpStatus
	req.autoResp = manualRespond_
	req.onError = onError
	req.expectPkts = 1
	req.onResp = onResp

	// conn still ok after readErr, but we need to report to our caller
	var readErr error

	req.pumpPkts =
		func(id uint32, conn *conn_, buff []byte) (nsent uint32, err error) {
			maxPacket := f.client.maxPacket
			pkt := sshFxpWritePacket{Handle: f.handle}
			length := pkt.sizeBeforeData()

			defer func() {
				if nil == err { // no conn err.  if conn err, conn notifies us.
					var signal bool
					respLock.Lock()
					respSendDone = true
					respPkts = nsent
					signal = (nsent == respAcks)
					respErr = errReqTrunc_
					respLock.Unlock()
					if signal {
						respCond.Signal()
					}
				}
			}()

			for {
				pkt.ID = id
				id++
				pkt.Offset = uint64(f.offset)

				readB := buff[4+length : 4+length+maxPacket]
				var readAmount int
				readAmount, readErr = r.Read(readB)
				nread += int64(readAmount)
				if readErr != nil {
					if readErr != io.EOF {
						return
					} else if 0 == readAmount { // eof, and no data read
						readErr = nil
						return // conn.writer may get a trunc event
					}
				}

				if 0 != nsent {
					req := f.client.request()
					req.expectType = sshFxpStatus
					req.autoResp = manualRespond_
					req.onError = onError
					req.onResp = onResp
					req.expectPkts = 1
					conn.rC <- req
				}

				pkt.Length = uint32(readAmount)
				bigEnd_.PutUint32(buff, uint32(length+readAmount))
				pkt.appendTo(buff[4:4])

				_, err = conn.w.Write(buff[:4+length+readAmount])
				if err != nil {
					return
				}
				f.offset += int64(readAmount)
				nsent++

				if io.EOF == readErr {
					readErr = nil
					return
				}
			}
		}
	err = f.client.conn.Request(req)
	if err != nil {
		return
	}
	respLock.Lock()
	for {
		respCond.Wait()
		if nil != respErr {
			err = respErr
			break
		}
	}
	respLock.Unlock()
	if err != nil {
		if errReqTrunc_ == err {
			err = nil // reached EOF
		}
	}
	err = readErr
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
func (f *File) WriteAt(dataB []byte, offset int64) (written int, err error) {

	if 0 == len(f.handle) {
		return 0, os.ErrClosed
	} else if 0 == len(dataB) {
		return
	}

	responder := f.client.responder()

	maxPacket := f.client.maxPacket
	expectPkts := len(dataB) / maxPacket
	if len(dataB) != expectPkts*maxPacket {
		expectPkts++
	}

	req := f.client.request()
	req.expectType = sshFxpStatus
	req.autoResp = manualRespond_
	req.onError = responder.onError
	req.expectPkts = uint32(expectPkts)

	req.pumpPkts =
		func(id uint32, conn *conn_, buff []byte) (nsent uint32, err error) {
			packetsToSend := expectPkts // need our own counter in closure
			pkt := sshFxpWritePacket{Handle: f.handle}
			length := pkt.sizeBeforeData()
			for 0 != packetsToSend {
				packetsToSend--
				pkt.ID = id
				id++
				amount := len(dataB)
				if amount > maxPacket {
					amount = maxPacket
				}
				pkt.Offset = uint64(offset)
				offset += int64(amount)
				pkt.Length = uint32(amount)
				pkt.Data = dataB[:amount]
				dataB = dataB[amount:]

				bigEnd_.PutUint32(buff, uint32(length+amount))
				pkt.appendTo(buff[4:4])

				_, err = conn.w.Write(buff[:4+length])
				if err != nil {
					return
				}
				_, err = conn.w.Write(pkt.Data)
				if err != nil {
					return
				}

				nsent++
				written += amount
			}
			return
		}

	conn := &f.client.conn
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
		return f.client.setstat(f.pathN, sshFileXferAttrUIDGID, fs)
	} else {
		return f.client.fsetstat(f.handle, sshFileXferAttrUIDGID, fs)
	}
}

// Change the permissions of the current file.
//
// See Client.Chmod for details.
func (f *File) Chmod(mode os.FileMode) error {
	if 0 == len(f.handle) {
		return f.client.setstat(f.pathN, sshFileXferAttrPermissions, toChmodPerm(mode))
	} else {
		return f.client.fsetstat(f.handle, sshFileXferAttrPermissions, toChmodPerm(mode))
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
		return f.client.setstat(f.pathN, sshFileXferAttrExtended, attrs)
	} else {
		return f.client.fsetstat(f.handle, sshFileXferAttrExtended, attrs)
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
		return f.client.setstat(f.pathN, sshFileXferAttrSize, uint64(size))
	} else {
		return f.client.fsetstat(f.handle, sshFileXferAttrSize, uint64(size))
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
	return f.client.invokeExpectStatus(&sshFxpFsyncPacket{Handle: f.handle})
}

// Asynchronously request a flush of the contents of a File to stable storage.
//
// Requires the server to support the fsync@openssh.com extension.
//
// async safe
func (f *File) SyncAsync(req any, onComplete AsyncFunc) error {
	if 0 == len(f.handle) {
		return os.ErrClosed
	}
	return f.client.asyncExpectStatus(
		&sshFxpFsyncPacket{Handle: f.handle}, nil, req, onComplete)
}
