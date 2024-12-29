package usftp

import (
	"fmt"
	"io"
	"slices"
	"sync/atomic"

	"github.com/tredeske/u/uerr"
)

const errReqTrunc_ = uerr.Const("request truncated")

// a bidirectional channel on which client requests are multiplexed
//
// https://filezilla-project.org/specs/draft-ietf-secsh-filexfer-02.txt
type conn_ struct {
	wC chan *clientReq_ // input chan to writer
	w  io.WriteCloser   // for writer

	maxPacket int
	client    *Client
	closed    atomic.Bool // the end

	rC      chan *clientReq_ // chan from writer to reader
	pos     int              // for reader
	r       io.Reader        // for reader
	backing []byte           // for reader
	buff    []byte           // for reader, req.onResp
}

func (conn *conn_) Construct(r io.Reader, w io.WriteCloser, c *Client) {
	conn.client = c
	conn.maxPacket = c.maxPacket
	conn.r = r
	conn.w = w
	conn.rC = make(chan *clientReq_, 2048)
	conn.wC = make(chan *clientReq_, 2048)
	// we add a little extra since data pkts can be 4+1+4 larger
	conn.backing = make([]byte, conn.maxPacket+16)
	conn.buff = conn.backing[:0]
	conn.closed.Store(true)
}

func (conn *conn_) Close() error {
	conn.closeConn()
	return nil
}

func (conn *conn_) closeConn() (wasClosed bool) {
	if conn.closed.CompareAndSwap(false, true) {
		close(conn.wC)
		return false
	}
	return true
}

type autoResp_ bool

const (
	autoRespond_   autoResp_ = true
	manualRespond_ autoResp_ = false
)

// these are created for each request to the server.
//
//	client/file -----> conn.writer ------> conn.reader
//
// and, in some cases
//
//	pumper ------> conn.reader
//
// once passed to the next guy, do not touch any of the fields in the req!
// each worker is free to modify req while they have it.
type clientReq_ struct {
	id         uint32 // request id filled in by writer
	expectPkts uint32 // pkts to send
	expectType uint8  // expected resp type (status always expected)
	cancelled  bool   // reader use

	// automatically call ensure before onResp, and onError after
	// onError must be set
	autoResp autoResp_

	// When pumpPkts is not set (most cases) this is the single packet to send.
	pkt idAwarePkt_

	// When multiple packets may be required for the requests, then this assumes
	// control of sending packets until the request is complete.
	//
	// This runs in the clientConn.writer context
	pumpPkts func(id uint32, conn *conn_, buff []byte) (sent uint32, err error)

	// Called for each received pkt related to the request.
	// This runs in the clientConn.reader context
	onResp func(id, length uint32, typ uint8, conn *conn_) error

	// Notify requestor that a problem occurred.
	//
	// This is only not set for "fire and forget" requests.
	//
	// This may run in either the clientConn writer or reader contexts
	onError func(error)

	client *Client
}

func (conn *conn_) RequestSingle(
	pkt idAwarePkt_,
	expectType uint8,
	autoResp autoResp_,
	onResp func(id, length uint32, typ uint8, conn *conn_) error,
	onError func(error),
) (
	err error,
) {
	req := conn.client.request()
	req.expectType = expectType
	req.autoResp = autoResp
	req.onResp = onResp
	req.onError = onError
	req.pkt = pkt
	req.expectPkts = 1
	return conn.Request(req)
}

func (conn *conn_) Request(req *clientReq_) (err error) {
	const errClosed = uerr.Const("sftp conn closed")

	if req.autoResp && nil == req.onError { // assert
		panic("autoResp set, but no onError set")
	}
	defer uerr.IgnoreClosedChanPanic()
	err = errClosed
	conn.wC <- req
	err = nil
	return
}

func (conn *conn_) writer() {
	const defaultSz = 16 * 1024 * 1024
	var err error
	idGen := uint32(1) // generate req ids
	var buff []byte
	if defaultSz <= conn.maxPacket+256 {
		buff = make([]byte, conn.maxPacket+256) // account for header
	} else {
		buff = make([]byte, defaultSz)
	}

	defer func() { // cleanup
		wasClosed := conn.closeConn()
		close(conn.rC)
		if !wasClosed && err != nil {
			err = uerr.Chainf(err, "SFTP writer")
			conn.client.reportError(err)
		}

		// notify any pending reqs still in writer chan
		err = &StatusError{
			Code: sshFxConnectionLost,
			msg:  "cancelled",
		}
		for req := range conn.wC {
			if nil != req.onError {
				req.onError(err)
			}
		}
	}()

	for req := range conn.wC {
		if 0 != idGen&0xb000_0000 {
			idGen = 1 // make sure pkt ids for this req don't wrap
		}
		req.id = idGen

		if nil == req.pumpPkts {
			conn.rC <- req
			req.pkt.setId(idGen)
			idGen++
			err = sendPacket(conn.w, buff[:], req.pkt)
			if err != nil {
				if nil != req.onError {
					req.onError(err)
				}
				return
			}

		} else { // for File.WriteTo, Write, WriteAt, ReadFrom

			var nsent uint32
			nsent, err = req.pumpPkts(idGen, conn, buff)
			if err != nil {
				return
			}
			idGen += nsent
		}
	}
}

// used by reader to cancel any reqs in flight while closing
func (conn *conn_) cancelReqs(reqs map[uint32]*clientReq_) {

	err := &StatusError{
		Code: sshFxConnectionLost,
		msg:  "cancelled",
	}

	//
	// cancel in order, but only the lowest id pkt for each req
	//
	if 0 != len(reqs) {
		cancelList := make([]uint32, 0, len(reqs))
		for id, _ := range reqs {
			cancelList = append(cancelList, id)
		}
		slices.Sort(cancelList)
		for _, id := range cancelList {
			req, ok := reqs[id]
			if !ok || nil == req.onError {
				continue
			}
			req.onError(err)
			hi := req.id + req.expectPkts - 1
			for rm := id; rm <= hi; rm++ {
				delete(reqs, rm)
			}
		}
	}

	//
	// drain the reader chan until it closes
	//
	for req := range conn.rC {
		if nil != req.onError {
			req.onError(err)
		}
	}
}

func (conn *conn_) reader() {
	const errUnexpected = uerr.Const("Unexpected SFTP req ID in response")
	var err error
	var id, length uint32
	var typ uint8
	var alive bool
	var found bool
	var req *clientReq_

	reqs := make(map[uint32]*clientReq_, 8192)

	defer func() {
		wasClosed := conn.closeConn()
		if !wasClosed && err != nil {
			conn.client.reportError(err)
		}
		conn.cancelReqs(reqs)
	}()

	for {

		//
		// get the next pkt header
		//
		id, length, typ, err = conn.readIdHeader()
		if err != nil {
			err = uerr.Chainf(err, "read SFTP header")
			return
		} else if length < 4 {
			err = uerr.Chainf(errShortPacket, "read SFTP header")
			return
		}

		//
		// check for req updates.
		//
		for {
			select {
			case req, alive = <-conn.rC:
				if !alive { // chan closed
					return
				}
				lo := req.id
				hi := req.id + req.expectPkts - 1
				for ; lo <= hi; lo++ {
					reqs[lo] = req
				}
				continue
			default: // don't wait on conn.rC
			}
			break
		}

		//
		// match to req
		//
		req, found = reqs[id]
		if !found {
			err = errUnexpected
			return
		}
		delete(reqs, id)

		//
		// handle response
		//
		req.expectPkts--
		if !req.cancelled {
			if req.expectType != typ && sshFxpStatus != typ { // server error
				err = fmt.Errorf("Reader expected SFTP packet type %d, but got %d",
					req.expectType, typ)
				return
			}
			if req.autoResp || sshFxpStatus == typ {
				err = conn.ensure(int(length))
				if err != nil {
					err = uerr.Chainf(err, "read SFTP payload")
					return
				}
			}
			reqErr := req.onResp(id, length, typ, conn)
			if req.autoResp && nil != req.onError {
				req.onError(reqErr)  // autoResp - whether err nil or not
				req.cancelled = true // ignore any responses still outstanding
			} else if reqErr != nil {
				req.cancelled = true // ignore any responses still outstanding
			}

		} else { // skip cancelled
			err = conn.ensure(int(length))
			if err != nil {
				err = uerr.Chainf(err, "read (skip) SFTP payload")
				return
			}
		}
		if 0 == req.expectPkts {
			req.recycle()
		}
		req = nil

		if int(length) > len(conn.buff) {
			conn.pos = 0
			conn.buff = conn.backing[:0]
		} else {
			conn.bump(int(length))
		}
	}
}

func (conn *conn_) readIdHeader() (id, length uint32, typ uint8, err error) {
	// id packets always start with 4 byte size, followed by 1 byte type,
	// followed by 4 byte id
	if err = conn.ensure(9); err != nil {
		return
	}
	length, _ = unmarshalUint32(conn.buff)
	if length > uint32(len(conn.backing)) {
		err = fmt.Errorf("recv pkt: %d bytes, but max is %d", length, len(conn.backing))
		return
	} else if length < 5 {
		err = errShortPacket
		return
	}
	length -= 5
	typ = conn.buff[4]
	id, _ = unmarshalUint32(conn.buff[5:])
	conn.bump(9)
	return
}

func (conn *conn_) ensure(amount int) (err error) { // help inline
	if amount <= len(conn.buff) {
		return
	}
	return conn.ensureRead(amount)
}

// only call from ensure()
func (conn *conn_) ensureRead(amount int) (err error) {
	if 0 != len(conn.buff) {
		amount -= len(conn.buff)
		copy(conn.backing, conn.buff)
		conn.buff = conn.backing[:len(conn.buff)]
	}
	if amount > len(conn.backing)-len(conn.buff) {
		return fmt.Errorf("cannot ensure space for %d when remaining backing is %d",
			amount, len(conn.backing)-len(conn.buff))
	}
	nread, err := io.ReadAtLeast(conn.r, conn.backing[len(conn.buff):], amount)
	if err != nil {
		return
	}
	conn.buff = conn.backing[:nread+len(conn.buff)]
	conn.pos = 0
	return
}

func (conn *conn_) bump(amount int) {
	conn.pos += amount
	conn.buff = conn.buff[amount:]
}

func (conn *conn_) Start() (exts map[string]string, err error) {
	// initial exchange has no pkt ids
	// - send init
	// - get back version and extensions
	//
	// https://filezilla-project.org/specs/draft-ietf-secsh-filexfer-02.txt
	initPkt := &sshFxInitPacket{
		Version: sftpProtocolVersion,
	}
	buff := make([]byte, 0, 4096)
	err = sendPacket(conn.w, buff, initPkt)
	if err != nil {
		return
	}

	// happily, we can reuse since version is same size as id
	version, length, typ, err := conn.readIdHeader()
	if err != nil {
		return
	} else if err = conn.ensure(int(length)); err != nil {
		return
	} else if typ != sshFxpVersion {
		err = &unexpectedPacketErr{sshFxpVersion, typ}
		return
	} else if version != sftpProtocolVersion {
		err = &unexpectedVersionErr{sftpProtocolVersion, version}
		return
	}

	if 0 != length {
		exts = make(map[string]string)
		for 0 != length {
			var ext extensionPair
			var data []byte
			ext, data, err = unmarshalExtensionPair(conn.buff)
			if err != nil {
				return
			}
			exts[ext.Name] = ext.Data
			amount := len(conn.buff) - len(data)
			conn.bump(amount)
			length -= uint32(amount)
		}
	}

	//
	// now we can start the reader and writer as all other pkts are idAware
	//
	conn.closed.Store(false)
	go conn.writer()
	go conn.reader()

	return
}
