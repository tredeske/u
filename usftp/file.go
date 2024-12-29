package usftp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tredeske/u/uerr"
)

const ErrOpenned = uerr.Const("file already openned")

const errStat_ = uerr.Const("file has no attrs, size needed, but NeverStat set")

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

// if attrs are populated, mode bits of file.  otherwise, bits are zero.
func (f *File) OsFileMode() os.FileMode { return f.attrs.OsFileMode() }

// return the internal FileStat to a go os.FileInfo
func (f *File) OsFileInfo() os.FileInfo { return f.attrs.AsFileInfo(f.pathN) }

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
// If File size is not known (File was not built from ReadDir, or no Stat call was
// placed prior to this), then the StatStrategy set on Client will be followed.
//
// synchronize i/o ops
func (f *File) WriteTo(w io.Writer) (written int64, err error) {

	if 0 == len(f.handle) {
		return 0, os.ErrClosed
	}

	if 0 == f.attrs.Mode && NeverStat == f.client.statStrategy {
		return 0, errStat_
	} else if 0 == f.attrs.Mode || AlwaysStat == f.client.statStrategy {
		_, err := f.Stat()
		if err != nil {
			return 0, err
		}
	}

	amount := int64(f.attrs.Size) - f.offset
	if amount <= 0 {
		return
	}

	chunkSz, lastChunkSz, req := f.buildReadReq(amount, f.offset)
	responder := f.client.responder()
	req.onError = responder.onError
	req.autoResp = manualRespond_
	expectPkts := req.expectPkts

	first := true
	var expectId uint32
	var wError error // problem writing to w

	req.onResp = func(id, length uint32, typ uint8, conn *conn_) (err error) {
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
				if err != nil {
					wError = err
					err = nil
					//
					// patch in io.Discard so that we can finish getting any data
					// from the sftp server so that we don't have to reset the
					// connection.  io.Discard cannot fail, so wError will only be
					// set once.
					//
					w = io.Discard
					length -= uint32(nwrote)
					if 0 != length {
						nwrote, _ = w.Write(buff)
						written += int64(nwrote)
					}
				}
				if int(length) == len(buff) {
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
					wError = err
					err = nil
					w = io.Discard // see above note about patching w
					length -= uint32(nwrote)
					if 0 != length {
						nwrote, _ = w.Write(buff)
						written += int64(nwrote)
					}
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

	err = f.client.conn.Request(req)
	if err != nil {
		return
	}
	err = responder.await()
	if err != nil {
		return
	}
	err = wError
	f.offset += amount
	return
}

// when reading from sftp server, we have to obey the maxPacket limit.
//
// if we request more bytes that that limit, then it will just return a
// truncated amount.
//
// therefore, we split up requests larger than that into chunks using the
// pumpPkts closure to manufacture reqs as needed by the conn writer.
func (f *File) buildReadReq(
	amount, offset int64,
) (
	chunkSz, lastChunkSz uint32,
	req *clientReq_,
) {
	maxPkt := int64(f.client.maxPacket)
	if amount > int64(f.attrs.Size)-offset {
		amount = int64(f.attrs.Size) - offset
	}
	chunkSz = uint32(maxPkt)
	if maxPkt > amount {
		chunkSz = uint32(amount)
	}
	expectPkts := amount / maxPkt
	if amount != expectPkts*maxPkt {
		lastChunkSz = uint32(amount - expectPkts*maxPkt)
		expectPkts++
	} else {
		lastChunkSz = chunkSz
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

			conn.rC <- req

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

// implement io.ReaderAt.  Read up to len toBuff bytes from file at current offset,
// leaving offset unchanged.
//
// synchronize i/o ops
func (f *File) ReadAt(toBuff []byte, offset int64) (nread int, err error) {
	const errMissing = uerr.Const(
		"previous read was short, but this was not - missing data")

	if 0 == len(f.handle) {
		return 0, os.ErrClosed
	}
	if 0 == len(toBuff) {
		return
	}

	if 0 == f.attrs.Mode && NeverStat == f.client.statStrategy {
		return 0, errStat_
	} else if 0 == f.attrs.Mode || AlwaysStat == f.client.statStrategy {
		_, err := f.Stat()
		if err != nil {
			return 0, err
		}
	}
	if offset >= int64(f.attrs.Size) {
		return 0, nil
	}

	chunkSz, lastChunkSz, req := f.buildReadReq(int64(len(toBuff)), offset)
	responder := f.client.responder()
	req.onError = responder.onError
	expectPkts := req.expectPkts //len(req.pkts)

	first := true
	var expectId uint32
	lastShort := false
	req.onResp = func(id, length uint32, typ uint8, conn *conn_) (err error) {
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
			err = fmt.Errorf("ReadAt expecting pkt %d, got %d", expectId, id)
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

	err = f.client.conn.Request(req)
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
	if nil == err && f.offset == int64(f.attrs.Size) {
		err = io.EOF
	}
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
// synchronize i/o ops
func (f *File) ReadFrom(r io.Reader) (nread int64, err error) {
	if 0 == len(f.handle) {
		return 0, os.ErrClosed
	}
	//
	// a special response handler for this special operation
	// - conn.writer (pumpPkts) could finish first
	// - or, conn.reader (onResp) could finish first
	// - or, could be a rando onError issue
	//
	var respLock sync.Mutex
	var respAcks, respPkts uint32
	var respPumpDone atomic.Bool
	var respErr error
	respCond := sync.Cond{L: &respLock}

	onError := func(err error) {
		respPumpDone.Store(true)
		respLock.Lock()
		respErr = err
		respLock.Unlock()
		respCond.Signal()
	}

	onResp := func(id, length uint32, typ uint8, conn *conn_) (err error) {
		if typ != sshFxpStatus {
			panic("impossible!")
		}
		err = maybeError(conn.buff) // may be nil
		//
		// if there was an error, or if pumpPkts is done and we've got all
		// of the responses, then signal the response waiter
		//
		var signal bool
		respLock.Lock()
		if nil == err {
			respAcks++
			if respPumpDone.Load() && respAcks == respPkts {
				respErr = errReqTrunc_
				signal = true
			}
		} else {
			respPumpDone.Store(true)
			respErr = err
			signal = true
		}
		respLock.Unlock()
		if signal {
			respCond.Signal()
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

	// conn still ok after pumpErr, but we need to report to our caller
	var pumpErr error

	req.pumpPkts =
		func(id uint32, conn *conn_, buff []byte) (nsent uint32, err error) {
			const errShortWrite = uerr.Const("SSH did not accept full write")
			var readErr error
			pumpReq := req
			pumpReq.id = id
			maxPacket := f.client.maxPacket // max data payload
			pkt := sshFxpWritePacket{
				Handle: f.handle,
				Offset: uint64(f.offset),
			}
			length := pkt.sizeBeforeData()  // of sshFxpWritePacket w/o payload
			maxSz := 4 + length + maxPacket // + sftp packet size header
			buffPos := 0
			expectPkts := uint32(0)

			defer func() {
				pumpErr = readErr
				// if not a conn err.  if it is a conn err, then conn will
				// do the notification.
				if nil == err && respPumpDone.CompareAndSwap(false, true) {
					//
					// mark that we're done and what we did.
					// if we completed first, then signal resp waiter
					//
					var signal bool
					respLock.Lock()
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

				pktB := buff[buffPos:]
				readB := pktB[4+length : maxSz]
				var readAmount int
				readAmount, readErr = r.Read(readB)
				if 0 != readAmount {
					nread += int64(readAmount)
					pkt.Length = uint32(readAmount)
					bigEnd_.PutUint32(pktB, uint32(length+readAmount))
					pkt.appendTo(pktB[4:4])

					pkt.Offset += uint64(readAmount)
					expectPkts++
					buffPos += 4 + length + readAmount
				} else if nil == readErr { // impossible?
					id--
					continue
				}

				if nil == readErr {
					//
					// if we can do another read, do it
					//
					if len(buff) >= buffPos+maxSz {
						continue
					}

					//
					// we've filled up the buffer as much as we can, so tell the
					// reader about what to expect, and write out the buffer
					//
					newReq := f.client.request()
					*newReq = *pumpReq
					pumpReq.expectPkts = expectPkts
					conn.rC <- pumpReq
					pumpReq = newReq
					pumpReq.id = id
					pumpReq.expectPkts = 0

				} else {
					//
					// we've reached EOF or there was a read problem.
					// flush out any data we have on hand
					//
					if 0 == expectPkts {
						if readErr == io.EOF {
							readErr = nil
						}
						return
					}
					pumpReq.expectPkts = expectPkts
					conn.rC <- pumpReq
					pumpReq = nil
				}

				var nwrote int
				nwrote, err = conn.w.Write(buff[:buffPos])
				if err != nil {
					return
				} else if nwrote != buffPos {
					err = errShortWrite
					return
				}
				nsent += expectPkts
				expectPkts = 0
				buffPos = 0

				if nil != readErr || respPumpDone.Load() {
					if io.EOF == readErr {
						readErr = nil
					}
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
	f.attrs.Size += uint64(nread)
	f.offset += int64(nread)
	if err != nil {
		if errReqTrunc_ == err {
			err = nil // reached EOF
		}
	}
	err = pumpErr
	return
}

// implement io.Writer.  Write bytes to file, appending at current offset.
//
// synchronize i/o ops
func (f *File) Write(b []byte) (nwrote int, err error) {

	if 0 == len(f.handle) {
		return 0, os.ErrClosed
	}
	nwrote, err = f.WriteAt(b, f.offset)
	f.offset += int64(nwrote)
	f.attrs.Size += uint64(nwrote)
	return
}

// implement io.WriterAt. Write bytes to file at current offset, leaving offset
// unchanged.
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

			conn.rC <- req

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

	req.onResp = func(id, length uint32, typ uint8, conn *conn_) (err error) {
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

	err = f.client.conn.Request(req)
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
// Seeking relative to the end of the file will follow the Client StatStrategy.
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
		if 0 == f.attrs.Mode && NeverStat == f.client.statStrategy {
			return f.offset, errStat_
		} else if 0 == f.attrs.Mode || AlwaysStat == f.client.statStrategy {
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
