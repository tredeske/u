package usftp

import (
	"fmt"
	"io"
	"sync/atomic"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"
	"github.com/tredeske/u/usync"
)

// a bidirectional channel on which client requests are multiplexed
//
// https://filezilla-project.org/specs/draft-ietf-secsh-filexfer-02.txt
type clientConn_ struct {
	r io.Reader
	w io.WriteCloser

	wC chan *clientReq_
	rC chan *clientReq_

	backing []byte
	buff    []byte
	pos     int
	closed  atomic.Bool
}

type clientReq_ struct {
	id         uint32 // request id filled in by writer
	expectPkts uint32 // pkts to send
	expectType uint8  // expected resp type (status always expected)
	noAutoResp bool   // disable invoke of onError after onResp
	//pkts       []idAwarePkt_  // request packets to send
	//single     [1]idAwarePkt_ // we mostly only ever send one per

	pkt     idAwarePkt_                 // we mostly ever send one per
	nextPkt func(id uint32) idAwarePkt_ // when more than 1 pkt

	onResp  func(id, length uint32, typ uint8) error
	onError func(error)
}

func newClientReq(
	pkt idAwarePkt_,
	expectType uint8,
	noAutoResp bool,
	onResp func(id, length uint32, typ uint8) error,
	onError func(error),
) (
	req *clientReq_,
) {
	req = &clientReq_{
		expectType: expectType,
		noAutoResp: noAutoResp,
		onResp:     onResp,
		onError:    onError,
	}
	req.pkt = pkt
	req.expectPkts = 1
	//req.single[0] = pkt
	//req.pkts = req.single[:]
	return
}

func (conn *clientConn_) Close() error {
	conn.closeConn()
	return nil
}

func (conn *clientConn_) closeConn() (wasClosed bool) {
	if conn.closed.CompareAndSwap(false, true) {
		close(conn.wC)
		return false
	}
	return true
}

func (conn *clientConn_) Construct(r io.Reader, w io.WriteCloser, maxPkt int) {
	conn.r = r
	conn.w = w
	conn.rC = make(chan *clientReq_, 2048)
	conn.wC = make(chan *clientReq_, 2048)
	// we add a little extra since data pkts can be 4+1+4 larger
	conn.backing = make([]byte, maxPkt+16)
	conn.buff = conn.backing[:0]
	conn.closed.Store(true)
}

func (conn *clientConn_) Start() (exts map[string]string, err error) {
	// initial exchange has no pkt ids
	// - send init
	// - get back version and extensions
	//
	// https://filezilla-project.org/specs/draft-ietf-secsh-filexfer-02.txt
	initPkt := &sshFxInitPacket{
		Version: sftpProtocolVersion,
	}
	buff := make([]byte, 0, 8192)
	err = sendPacket(conn.w, buff, initPkt)
	if err != nil {
		return
	}

	length, typ, err := conn.readHeader()
	if err != nil {
		return
	}
	if err = conn.ensure(int(length)); err != nil {
		return
	}
	if typ != sshFxpVersion {
		err = &unexpectedPacketErr{sshFxpVersion, typ}
		return
	}

	version, _, err := unmarshalUint32Safe(conn.buff)
	if err != nil {
		return
	}
	conn.bump(4)
	length -= 4

	if version != sftpProtocolVersion {
		err = &unexpectedVersionErr{sftpProtocolVersion, version}
		return
	}

	if 0 != length {
		exts = make(map[string]string)
	}
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

	//
	// now we can start the reader and writer as all other pkts are idAware
	//
	conn.closed.Store(false)
	go conn.writer()
	go conn.reader()

	return
}

func (conn *clientConn_) RequestSingle(
	pkt idAwarePkt_,
	expectType uint8,
	noAutoResp bool,
	onResp func(id, length uint32, typ uint8) error,
	onError func(error),
) (
	err error,
) {
	return conn.Request(newClientReq(pkt, expectType, noAutoResp, onResp, onError))
}

func (conn *clientConn_) Request(req *clientReq_) (err error) {
	const errClosed = uerr.Const("sftp conn closed")
	defer usync.BareIgnoreClosedChanPanic()
	err = errClosed
	conn.wC <- req
	err = nil
	return
}

func (conn *clientConn_) writer() {
	var err error
	idGen := uint32(1) // generate req ids
	buff := make([]byte, 8192)

	defer func() {
		wasClosed := conn.closeConn()
		close(conn.rC)
		if !wasClosed && err != nil {
			// TODO

			ulog.Errorf("SFTP write failed: %s", err)
		}
	}()

	for req := range conn.wC {
		req.id = idGen

		//ulog.Printf("XXX: send idGen %d, expect=%d", req.id, req.expectPkts)

		conn.rC <- req

		if 1 == req.expectPkts {
			req.pkt.setId(idGen)
			idGen++
			//ulog.Printf("XXX: send single: %#v", req.pkt)
			err = sendPacket(conn.w, buff[:], req.pkt)
			if err != nil {
				if nil != req.onError {
					req.onError(err)
				}
				return
			}
			continue
		}

		for i := uint32(0); i < req.expectPkts; i++ {
			pkt := req.nextPkt(idGen)
			idGen++
			err = sendPacket(conn.w, buff[:], pkt)
			if err != nil {
				if nil != req.onError {
					req.onError(err)
				}
				return
			}
			//ulog.Printf("XXX: sent %d", pkt.id())
		}
	}
}

func (conn *clientConn_) reader() {
	const errUnexpected = uerr.Const("Unexpected SFTP req ID in response")
	var err error
	var length uint32
	var typ uint8
	var alive bool
	var found bool
	var req *clientReq_

	reqs := make(map[uint32]*clientReq_, 1024)

	defer func() {
		wasClosed := conn.closeConn()

		if !wasClosed && err != nil {
			if nil != req && nil != req.onError {
				req.onError(err)
			}
			ulog.Warnf("SFTP reader failed: %s", err)
		}
	}()

	for {
		//
		// wait until we get a response pkt
		//
		// id packets always start with 4 byte size,
		// followed by 1 byte type
		// followed by 4 byte req id
		//
		if err = conn.ensure(9); err != nil {
			return
		}

		//dbgLen := len(conn.buff)
		//if dbgLen >= 48 {
		//dbgLen = 48
		//}
		//ulog.Printf("XXX: read\n%s", hex.Dump(conn.buff[:dbgLen]))

		length, typ, err = conn.readHeader()
		if err != nil {
			return
		} else if length < 4 {
			err = errShortPacket
			return
		}
		id, _ := unmarshalUint32(conn.buff)
		conn.bump(4)
		length -= 4

		//ulog.Printf("XXX: recv %d, len=%d", id, length)

		//
		// match to req
		//
		req, found = reqs[id]
		for !found {
			select {
			case req, alive = <-conn.rC:
				if !alive {
					return
				}
				lo := req.id
				hi := req.id + req.expectPkts - 1
				//hi := req.id + uint32(len(req.pkts)) - 1
				found = (id >= lo && id <= hi)
				for ; lo <= hi; lo++ {
					reqs[lo] = req
				}
			default:
				err = errUnexpected
				return
			}
		}
		delete(reqs, id)

		//
		// handle response
		//
		if req.expectType != typ && sshFxpStatus != typ {
			err = fmt.Errorf("Expected packet type %d, but got %d",
				req.expectType, typ)
			return
		}
		reqErr := req.onResp(id, length, typ)
		if !req.noAutoResp && nil != req.onError {
			req.onError(reqErr) // autoResp - whether err nil or not
		}
		req = nil // disable onError after here
		//if err != nil {
		//	ulog.Printf("XXX: onResp %s", err)
		//	return
		//}
		if int(length) > len(conn.buff) {
			conn.pos = 0
			conn.buff = conn.backing[:0]
		} else {
			conn.bump(int(length))
		}
	}
}

func (conn *clientConn_) readHeader() (length uint32, typ uint8, err error) {
	// packets always start with 4 byte size, followed by 1 byte type
	if err = conn.ensure(5); err != nil {
		return
	}
	length, _ = unmarshalUint32(conn.buff)
	if length > uint32(len(conn.backing)) {
		err = fmt.Errorf("recv pkt: %d bytes, but max is %d", length, len(conn.backing))
		//err = errLongPacket
		return
	} else if length == 0 {
		err = errShortPacket
		return
	}
	length--
	typ = conn.buff[4]
	conn.bump(5)
	return
}

func (conn *clientConn_) ensure(amount int) (err error) { // help inline
	if amount <= len(conn.buff) {
		return
	}
	return conn.ensureRead(amount)
}

// only call from ensure()
func (conn *clientConn_) ensureRead(amount int) (err error) {
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

func (conn *clientConn_) bump(amount int) {
	conn.pos += amount
	conn.buff = conn.buff[amount:]
}
