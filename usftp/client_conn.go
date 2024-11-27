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
	id         uint32         // request id filled in by writer
	expectType uint8          // expected resp type (status always expected)
	noAutoResp bool           // disable invoke of onError after onResp
	pkts       []idAwarePkt_  // request packets to send
	single     [1]idAwarePkt_ // we mostly only ever send one per
	onResp     func(id, length uint32, typ uint8) error
	onError    func(error)
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
	req.single[0] = pkt
	req.pkts = req.single[:]
	return
}

func (c *clientConn_) Close() error {
	c.closeConn()
	return nil
}

func (c *clientConn_) closeConn() (wasClosed bool) {
	if c.closed.CompareAndSwap(false, true) {
		close(c.wC)
		return false
	}
	return true
}

func (c *clientConn_) Construct(r io.Reader, w io.WriteCloser, maxPkt int) {
	c.r = r
	c.w = w
	c.rC = make(chan *clientReq_, 512)
	c.wC = make(chan *clientReq_, 512)
	c.backing = make([]byte, maxPkt)
	c.buff = c.backing[:0]
	c.closed.Store(true)
}

func (c *clientConn_) Start() (exts map[string]string, err error) {
	// initial exchange has no pkt ids
	// - send init
	// - get back version and extensions
	//
	// https://filezilla-project.org/specs/draft-ietf-secsh-filexfer-02.txt
	initPkt := &sshFxInitPacket{
		Version: sftpProtocolVersion,
	}
	err = sendPacket(c.w, initPkt)
	if err != nil {
		return
	}

	length, typ, err := c.readHeader()
	if err != nil {
		return
	}
	if err = c.ensure(int(length)); err != nil {
		return
	}
	if typ != sshFxpVersion {
		err = &unexpectedPacketErr{sshFxpVersion, typ}
		return
	}

	version, _, err := unmarshalUint32Safe(c.buff)
	if err != nil {
		return
	}
	c.bump(4)
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
		ext, data, err = unmarshalExtensionPair(c.buff)
		if err != nil {
			return
		}
		exts[ext.Name] = ext.Data
		amount := len(c.buff) - len(data)
		c.bump(amount)
		length -= uint32(amount)
	}

	//
	// now we can start the reader and writer as all other pkts are idAware
	//
	c.closed.Store(false)
	go c.writer()
	go c.reader()

	return
}

func (c *clientConn_) RequestSingle(
	pkt idAwarePkt_,
	expectType uint8,
	noAutoResp bool,
	onResp func(id, length uint32, typ uint8) error,
	onError func(error),
) (
	err error,
) {
	return c.Request(newClientReq(pkt, expectType, noAutoResp, onResp, onError))
}

func (c *clientConn_) Request(req *clientReq_) (err error) {
	defer usync.BareIgnoreClosedChanPanic()
	err = errClosed
	c.wC <- req
	err = nil
	return
}

func (c *clientConn_) writer() {
	var err error
	idGen := uint32(1) // generate req ids

	defer func() {
		wasClosed := c.closeConn()
		close(c.rC)
		if !wasClosed && err != nil {
			// TODO

			ulog.Errorf("SFTP write failed: %s", err)
		}
	}()

	for req := range c.wC {
		req.id = idGen

		for _, pkt := range req.pkts {
			pkt.setId(idGen)
			idGen++
		}
		c.rC <- req
		for _, pkt := range req.pkts {
			err = sendPacket(c.w, pkt)
			if err != nil {
				if nil != req.onError {
					req.onError(err)
				}
				return
			}
		}
	}
}

func (c *clientConn_) reader() {
	const errUnexpected = uerr.Const("Unexpected SFTP req ID in response")
	var err error
	var length uint32
	var typ uint8
	var alive bool
	var found bool
	var req *clientReq_

	reqs := make(map[uint32]*clientReq_, 1024)

	defer func() {
		wasClosed := c.closeConn()

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
		if err = c.ensure(9); err != nil {
			return
		}
		length, typ, err = c.readHeader()
		if err != nil {
			return
		} else if length < 4 {
			err = errShortPacket
			return
		}
		id, _ := unmarshalUint32(c.buff)
		c.bump(4)
		length -= 4

		//
		// match to req
		//
		req, found = reqs[id]
		for !found {
			select {
			case req, alive = <-c.rC:
				if !alive {
					return
				}
				lo := req.id
				hi := req.id + uint32(len(req.pkts)) - 1
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
		if int(length) > len(c.buff) {
			c.pos = 0
			c.buff = c.backing[:0]
		} else {
			c.bump(int(length))
		}
	}
}

func (c *clientConn_) readHeader() (length uint32, typ uint8, err error) {
	// packets always start with 4 byte size, followed by 1 byte type
	if err = c.ensure(5); err != nil {
		return
	}
	length, _ = unmarshalUint32(c.buff)
	if length > maxMsgLength {
		err = errLongPacket
		return
	} else if length == 0 {
		err = errShortPacket
		return
	}
	length--
	typ = c.buff[4]
	c.bump(5)
	return
}

func (c *clientConn_) ensure(amount int) (err error) {
	if amount <= len(c.buff) {
		return
	}
	return c.ensureRead(amount)
}

// only call from ensure()
func (c *clientConn_) ensureRead(amount int) (err error) {
	if amount <= len(c.buff) { // repeat test
		return
	}
	if 0 != len(c.buff) {
		amount -= len(c.buff)
		copy(c.backing, c.buff)
		c.buff = c.backing[:len(c.buff)]
	}
	nread, err := io.ReadAtLeast(c.r, c.backing[len(c.buff):], amount)
	if err != nil {
		return
	}
	c.buff = c.backing[:nread+len(c.buff)]
	c.pos = 0
	return
}

func (c *clientConn_) bump(amount int) {
	c.pos += amount
	c.buff = c.buff[amount:]
}
