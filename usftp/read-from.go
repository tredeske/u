package usftp

import (
	"io"
	"sync"
	"sync/atomic"

	"github.com/tredeske/u/uerr"
)

// a special response handler for this special operation
// - conn.writer (pumpPkts) could finish first
// - or, conn.reader (onResp) could finish first
// - or, could be a rando onError issue
type readFrom_ struct {
	lock     sync.Mutex
	cond     sync.Cond
	acks     int64
	pkts     int64
	pumpDone atomic.Bool
	err      error
	pumpErr  error // conn still ok, but we need to report to our caller
	f        *File
	fromR    io.Reader
	nread    int64
	req      *clientReq_
}

func (resp *readFrom_) readFrom(f *File, fromR io.Reader) (nread int64, err error) {
	resp.cond = sync.Cond{L: &resp.lock}
	resp.f = f
	resp.fromR = fromR
	//
	// send 1 req at a time, but only the first req goes to the conn.writer
	//
	resp.req = f.client.request()
	resp.req.expectType = sshFxpStatus
	resp.req.autoResp = manualRespond_
	resp.req.onError = resp.onError
	resp.req.expectPkts = 1
	resp.req.onResp = resp.onResp
	resp.req.pumpPkts = resp.pumpPkts

	err = f.client.conn.Request(resp.req)
	if err != nil {
		return
	}
	return resp.wait()
}

func (resp *readFrom_) onError(err error) {
	resp.pumpDone.Store(true)
	resp.lock.Lock()
	resp.err = err
	resp.lock.Unlock()
	resp.cond.Signal()
}

func (resp *readFrom_) onResp(
	id, length uint32,
	typ uint8,
	conn *conn_,
) (err error) {
	if typ != sshFxpStatus {
		panic("impossible!")
	}
	err = maybeError(conn.buff) // may be nil
	//
	// if there was an error, or if pumpPkts is done and we've got all
	// of the responses, then signal the response waiter
	//
	var signal bool
	resp.lock.Lock()
	if nil == err {
		resp.acks++
		if resp.pumpDone.Load() && resp.acks == resp.pkts {
			resp.err = errReqTrunc_
			signal = true
		}
	} else {
		resp.pumpDone.Store(true)
		resp.err = err
		signal = true
	}
	resp.lock.Unlock()
	if signal {
		resp.cond.Signal()
	}
	return
}

// if not a conn err.  if it is a conn err, then conn will do the notification.
func (resp *readFrom_) done(totalPkts int64, err, readErr error) {
	resp.pumpErr = readErr
	if nil == err && resp.pumpDone.CompareAndSwap(false, true) {
		//
		// mark that we're done and what we did.
		// if we completed first, then signal resp waiter
		//
		var signal bool
		resp.lock.Lock()
		resp.pkts = totalPkts
		signal = (totalPkts == resp.acks)
		resp.err = errReqTrunc_
		resp.lock.Unlock()
		if signal {
			resp.cond.Signal()
		}
	}
}

func (resp *readFrom_) wait() (nread int64, err error) {
	resp.lock.Lock()
	for {
		resp.cond.Wait()
		if nil != resp.err {
			err = resp.err
			break
		}
	}
	resp.lock.Unlock()
	nread = resp.nread
	return
}

func (resp *readFrom_) pumpPkts(
	id uint32, conn *conn_, buff []byte,
) (
	nsent uint32, err error,
) {
	const errShortWrite = uerr.Const("SSH did not accept full write")
	var readErr error
	pumpReq := resp.req
	pumpReq.id = id
	maxPacket := resp.f.client.maxPacket // max data payload
	pkt := sshFxpWritePacket{
		Handle: resp.f.handle,
		Offset: uint64(resp.f.offset),
	}
	length := pkt.sizeBeforeData()  // of sshFxpWritePacket w/o payload
	maxSz := 4 + length + maxPacket // + sftp packet size header
	buffPos := 0
	expectPkts := uint32(0)
	totalPkts := int64(nsent)

	defer func() {
		resp.done(totalPkts, err, readErr)
	}()

	for {
		pkt.ID = id
		id++

		pktB := buff[buffPos:]
		readB := pktB[4+length : maxSz]
		var readAmount int
		readAmount, readErr = resp.fromR.Read(readB)
		if 0 != readAmount {
			resp.nread += int64(readAmount)
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
			newReq := resp.f.client.request()
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
		totalPkts += int64(expectPkts)
		expectPkts = 0
		buffPos = 0

		if nil != readErr || resp.pumpDone.Load() {
			if io.EOF == readErr {
				readErr = nil
			}
			return
		}
	}
}
