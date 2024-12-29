package unet

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"runtime"
	"slices"
	"syscall"
	"time"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"
)

const (
	MtuMinIpv4  = 576  // RFC 791, but RFC 4821 says 1024 is smallest practical
	MtuMinIpv6  = 1280 // RFC 2460
	MtuMaxJumbo = 9216 // supported by most (all?) equipment, but 9000 common

	//
	// This one needs explanation.
	//
	// IPv4 has a 16 bit length field, but that includes the header and options.
	// The header is 20 bytes, so the payload is a max of 65515 bytes.
	//
	// UDP has a 16 bit length field, so it's max is 65535, but then the IPv4 header
	// added to that would be 65555!  You probably only see something like this
	// for loopback, where likely there is no 'real' ipv4 header.
	//
	// Indeed, loopback often has mtu set at 65536.
	//
	// See also RFC 2675 (jumbograms), which talks about mtu of 65575 for
	// non-jumbograms!  We don't support the notion of jumbograms here.
	//
	// However, with all that in mind, even with loopback mtu of 65536, the highest
	// probed value will be 65535!  This makes sense since largest UDP datagram is
	// 65535.
	//
	MtuMax = 65535
)

// client side of MTU discovery - sends UDP probes and sees what happens
type MtuProber struct {

	//
	// if set, log output
	//
	Name string

	//
	// if set, interval between sending probes.  Default 120ms
	//
	ProbeInterval time.Duration

	//
	// set this to create initial buffer, or one will be created w/o your help
	//
	OnStart func(size uint16) (space []byte)

	//
	// if set, is called to create pkt from space prior to each send.
	//
	// size is how many bytes pkt should be in length
	//
	BeforeSend func(size uint16, space []byte) (pkt []byte, err error)

	//
	// if set, is called after receiving each pkt
	//
	AfterRecv func(pkt []byte) (err error)

	farAddr    Address        // where we're probing to
	sizes      []uint32       // mtu sizes we're trying (sorted)
	hits       map[uint32]int // number of successful probes at each size
	sock       *Socket        // the socket used for sending probes
	checkSock  *Socket        // the socket used for checking kernel pmtu cache
	closeSock  bool           // if we create the socket, we close it
	closeCheck bool           // if we create the socket, we close it

	//
	// if set, set smallest probe size.  otherwise, smallest compliant will be used
	//
	MtuMin uint16

	//
	// if set, set largest probe size.  otherwise, reasonable max will be used
	//
	MtuMax uint16

	//
	// Result: if non-zero, kernel cached PMTU value
	//
	CachedPmtu uint16

	//
	// Result: detected/probed PMTU value
	//
	Pmtu uint16

	//
	// Result: overhead of IP and UDP
	//
	Overhead uint16

	//
	// Result: if BeforeSend and AfterRecv not set, this is computed
	//
	LatencyAvg time.Duration
	LatencyMin time.Duration
	LatencyMax time.Duration
}

// set external socket to be used to send/recv probes
// sock should be connected, constructed similarly as per NewSock
func (this *MtuProber) SetProbeSock(sock *Socket) {
	if nil != this.sock {
		panic("sock already set!")
	} else if IsSockaddrPortOrIpZero(sock.FarAddr) {
		panic(fmt.Sprintf("probeSock.FarAddr not set! (%#v)", sock.FarAddr))
	}
	this.sock = sock
}

// set external socket to be used to query kernel pmtu
// must be constructed similarly to NewProbeSock
// sock.FarAddr must be set
func (this *MtuProber) SetCheckSock(sock *Socket) {
	if nil != this.checkSock {
		panic("checkSock already set!")
	} else if IsSockaddrPortOrIpZero(sock.FarAddr) {
		panic(fmt.Sprintf("checkSock.FarAddr not set! (%#v)", sock.FarAddr))
	}
	this.checkSock = sock
}

// create a new socket to be the probe socket, used to send/recv probes.
func (this *MtuProber) NewProbeSock(src, dst Address) (err error) {
	if nil != this.sock {
		panic("probe sock already set!")
	}
	this.sock, err = this.newSock(src, dst)
	if err != nil {
		return
	}
	this.closeSock = true
	return
}

// Create a new socket to be the check socket, used to query kernel pmtu table.
// If not set, then the probe socket will be used.
func (this *MtuProber) NewCheckSock(src, dst Address) (err error) {
	if nil != this.checkSock {
		panic("check sock already set!")
	}
	this.checkSock, err = this.newSock(src, dst)
	if err != nil {
		return
	}
	this.closeCheck = true
	return
}

func (this *MtuProber) newSock(src, dst Address) (sock *Socket, err error) {
	const errUnset = uerr.Const("dst address (ip and port) not set")
	if dst.IsEitherZero() {
		return nil, errUnset
	}
	return NewSocket().
		SetNearAddress(src).
		SetFarAddress(dst).
		ConstructUdp().
		SetOptReuseAddr().
		//
		// if not large enough, pkts will be dropped
		//
		SetOptRcvBuf(4 * 1024 * 1024).
		SetOptMtuDiscover(MtuDiscoProbe).
		Bind().
		Connect().
		Done()
}

func (this *MtuProber) Close() {
	if this.closeSock && nil != this.sock {
		this.sock.Close()
	}
	this.sock = nil
	if this.closeCheck && nil != this.checkSock {
		this.checkSock.Close()
	}
	this.checkSock = nil
}

// the kernel caches pmtu on a per destination basis as it gets icmp pkts back
// saying dest unreach due to pkt too large
//
// we can get this value from the kernel with a socket that is connected to the
// destination.
//
// the value returned may be:
// - the mtu of the interface
// - the cached mtu from mtu discovery
// - a lie (if some device sent back an untruthful ICMP)
func (this *MtuProber) getCachedPmtu() (pmtu uint16, err error) {
	var cached int
	sock := this.checkSock
	if nil == sock {
		sock = this.sock
	}
	err = sock.GetOptMtu(&cached).Error
	if err != nil {
		//ulog.Printf("get opt mtu: %s %#v", err, err)
		if errors.Is(err, syscall.ENOTCONN) {
			err = nil
		}
		return
	}
	if 0 < cached && 65535 >= cached {
		this.addSize(uint32(cached))
		pmtu = uint16(cached)
		if 0 != len(this.Name) && this.CachedPmtu != pmtu {
			ulog.Printf("%s: MTU Probe: detected cached PMTU %d", this.Name, pmtu)
		}
		this.CachedPmtu = pmtu
	}
	return
}

// perform probing until PMTU is known
func (this *MtuProber) Probe() (pmtu uint16, err error) {

	verbose := 0 != len(this.Name)
	this.CachedPmtu = 0
	this.Pmtu = 0
	this.Overhead = 0
	this.LatencyAvg = 0
	this.LatencyMin = 0
	this.LatencyMax = 0
	this.sizes = make([]uint32, 0, 128)
	this.hits = make(map[uint32]int, 128)
	if 20*time.Millisecond >= this.ProbeInterval {
		this.ProbeInterval = 120 * time.Millisecond
	}

	err = this.ensureDefaults()
	if err != nil {
		return
	}
	this.Overhead = uint16(this.sock.IpOverhead() + UDP_OVERHEAD)
	lowest := uint32(this.MtuMin)
	highest := uint32(this.MtuMax)
	var lastLow, lastHi, responses, hiRx uint32
	var probeBackoff time.Duration

	defer func() {
		this.sizes = nil
		this.hits = nil
		if nil == err {
			if nil != this.checkSock {
				cached, errCheck := this.getCachedPmtu()
				if nil == errCheck {
					pmtu = cached
				}
			}
			this.Pmtu = pmtu
			ulog.Printf("%s: MTU Probe: done.  pmtu=%d, highest=%d, lowest=%d, hiRx=%d", this.Name, pmtu, highest, lowest, hiRx)
		}
		this.Close()
	}()

	//
	// pre check kernel PMTU cache
	//
	this.getCachedPmtu()

	//
	// start disco
	//
	var space []byte
	if nil != this.OnStart {
		space = this.OnStart(this.MtuMax)
		if 0 == len(space) {
			err = errors.New("OnStart must allocate some space")
			return
		}
	} else {
		space = make([]byte, this.MtuMax)
	}

	var totalMics, samples int64
	if nil == this.BeforeSend {
		this.BeforeSend = func(size uint16, space []byte) (pkt []byte, err error) {
			binary.LittleEndian.PutUint64(space, uint64(time.Now().UnixMicro()))
			return space[:size], nil
		}
		if nil == this.AfterRecv {
			this.AfterRecv = func(pkt []byte) (err error) {
				samples++
				mics := time.Now().UnixMicro() -
					int64(binary.LittleEndian.Uint64(pkt))
				totalMics += mics
				latency := time.Duration(1000 * mics)
				if this.LatencyMax < latency {
					this.LatencyMax = latency
				}
				if this.LatencyMin == 0 || this.LatencyMin > latency {
					this.LatencyMin = latency
				}
				return
			}
		}
	}

	recvBuff := make([]byte, len(space))
	gotHiRx := false
	addSizes := true

	poller := Poller{}
	err = poller.Open()
	if err != nil {
		return
	}
	defer poller.Close()
	poller.Add(&Polled{

		Sock: this.sock,

		OnHup: func(polled *Polled) (ok bool, err error) {
			return false, io.EOF
		},

		//
		// what to do when we get response datagram during Poll() (PollFor, ...)
		//
		// !ok means to break out of Poll()
		//
		OnInput: func(polled *Polled) (ok bool, err error) {

			nread, from, err := this.sock.RecvFrom(recvBuff, syscall.MSG_DONTWAIT)
			if verbose && 0 < nread {
				ulog.Printf("%s: MTU Probe: recv %d (mtu %d)", this.Name, nread,
					nread+int(this.Overhead))
			}
			if err != nil {
				if !errors.Is(err, syscall.EMSGSIZE) {
					return false, err
				}
				//
				// some device sent us back ICMP with MTU we should use.
				//
				// we can read this from the kernel with a connected socket,
				// but we've learned from hard experience that the device
				// sending the ICMP may be lying to us, so we ought to press
				// on.
				//
				var cached uint16
				cached, err = this.getCachedPmtu()
				if err != nil {
					return false, uerr.Chainf(err, "getting EMSGSIZE mtu")
				}
				if verbose {
					ulog.Printf("%s: MTU Probe: recv got EMSGSIZE %d",
						this.Name, cached)
				}
				responses++
				addSizes = true
				return true, nil

			} else {
				var fromAddr Address
				fromAddr.FromSockaddr(from)
				if this.farAddr != fromAddr {
					if verbose {
						ulog.Printf("%s: MTU Probe: got stray pkt from %s",
							this.Name, fromAddr.String())
					}
					return true, nil
				}
			}
			responses++
			addSizes = true
			sz := uint32(nread) + uint32(this.Overhead)
			if sz > highest {
				highest = sz
			}
			this.hits[sz] = this.hits[sz] + 1
			if sz >= hiRx {
				hiRx = sz
				gotHiRx = true
			}
			if sz > lowest {
				lowest = sz
				if lowest == highest { // converged
					return false, nil // stop polling this round
				}
			}
			if nil != this.AfterRecv {
				err = this.AfterRecv(recvBuff[:nread])
				if err != nil {
					return
				}
			}
			return true, nil
		},
	})

	//
	// send pkts and collect responses until we have pmtu
	//
	var times int
	var pkt []byte
	for lowest != highest {
		if verbose {
			ulog.Printf("%s: MTU Probe: lowest = %d, highest = %d",
				this.Name, lowest, highest)
		}
		logSend := lowest != lastLow || highest != lastHi
		lastLow = lowest
		lastHi = highest
		if addSizes {
			this.addSizes(lowest, highest)
			addSizes = false
		}
		for _, sz := range this.sizes {
			if sz < lowest || sz > highest {
				continue
			}
			pktSz := uint16(sz) - this.Overhead
			pkt, err = this.BeforeSend(pktSz, space)
			if err != nil {
				return
			} else if int(pktSz) != len(pkt) {
				err = errors.New("BeforeSend must create correct sized packet")
				return
			}
			if verbose && logSend {
				ulog.Printf("%s: MTU Probe: sending %d (mtu %d)",
					this.Name, pktSz, sz)
			}
			err = this.sock.SendTo(pkt, 0, nil)
			if err != nil {
				if errors.Is(err, syscall.ECONNREFUSED) {
					//
					// ignore temporary condition where peer not up yet
					//
					err = nil
					runtime.Gosched()
					break
				} else if !errors.Is(err, syscall.EMSGSIZE) {
					err = uerr.Chainf(err, "sending %d pkt", pktSz)
					return
				}
				//
				// EMSGSIZE
				//
				// before we sent, an ICMP came back from a device saying a
				// previous pkt was too big, and the kernel cached the
				// value provided in the ICMP as the PMTU.
				//
				// we are sending with MtuDiscoProbe, so kernel should ignore
				// any cached PMTU, but we may be hitting the interface MTU.
				//
				// we can read the cached PMTU from the kernel with a connected
				// socket, but the device may have lied to us, so we should
				// positively validate this with actual pkts.
				//
				// in any case, the pkt we just sent is too big, so take it off
				// the list.
				//
				var cachedPmtu uint16
				cachedPmtu, err = this.getCachedPmtu()
				if err != nil {
					return
				}
				if verbose {
					ulog.Printf(
						"%s: MTU Probe: send %d (mtu %d) got EMSGSIZE %d",
						this.Name, pktSz, sz, cachedPmtu)
				}
				highest = sz - 1
				if lowest >= highest {
					pmtu = cachedPmtu
					return //////////////////////// done
				}
				addSizes = true
				break
			}
		}

		//
		// check for responses
		//
		gotHiRx = false
		preHiRx := hiRx
		if 0 == responses {
			if 60*time.Second > probeBackoff {
				probeBackoff += this.ProbeInterval
			}
		} else {
			probeBackoff = 0
			responses = 0
		}
		_, err = poller.PollFor(this.ProbeInterval + probeBackoff)
		if err != nil {
			err = uerr.Chainf(err, "by receiver")
			return
		}
		if gotHiRx {
			if preHiRx == hiRx && 0 != hiRx && this.hasNextSize(hiRx) {

				times++
				if times >= 3 {
					break // all done
				}
			} else {
				times = 0
			}
		}
	}
	if hiRx > 65535 {
		panic("highest somehow larger than 65535!")
	}
	if 0 != samples {
		this.LatencyAvg = time.Duration((totalMics / samples) * 1000)
	}
	pmtu = uint16(hiRx)
	return
}

func (this *MtuProber) ensureDefaults() (err error) {
	if nil == this.sock {
		return errors.New("Cannot discover MTU w/o a socket!")
	}
	var nearAddr Address
	err = this.sock.
		GetNearAddress(&nearAddr).
		GetFarAddress(&this.farAddr).
		Error
	if err != nil {
		return
	} else if !nearAddr.IsPortSet() || !nearAddr.IsIpSet() {
		return errors.New("Near socket address missing port or IP")
	} else if !this.farAddr.IsPortSet() || !this.farAddr.IsIpSet() {
		return errors.New("Far socket address missing port or IP")
	}

	minMtu := uint16(MtuMinIpv4)
	if this.sock.IsIpv6() {
		minMtu = MtuMinIpv6
	}
	if 0 == this.MtuMax {
		this.MtuMax = MtuMaxJumbo
	} else if this.MtuMax < minMtu {
		return fmt.Errorf("MtuMax must be at least %d", minMtu)
	}
	if this.MtuMax > MtuMax {
		return errors.New("MtuMax cannot be greater than 65536")
	}

	//
	// allow setting below standard, but not ridiculously low
	//
	if 256 > this.MtuMin {
		this.MtuMin = minMtu
	} else if this.MtuMin < minMtu {
		return fmt.Errorf("MtuMin must be at least %d", minMtu)
	}
	if this.MtuMin > this.MtuMax {
		return fmt.Errorf("MtuMin (%d) must be less than MtuMax (%d)",
			this.MtuMin, this.MtuMax)
	}
	return
}

func (this *MtuProber) addSizes(lowest, highest uint32) {
	const PKTS = 8 // pow of 2

	if lowest > highest {
		panic("highest must be > lowest")
	} else if 64 > lowest {
		panic("lowest out of range")
	}

	this.addSize(highest)
	if 0 != this.hits[lowest] {
		//
		// we already successfully got a response with lowest, so we should try
		// one higher - this really cuts down on probing since lowest of last
		// round is likely to be the highest possible
		//
		this.addSize(lowest + 1)
	}

	idx := slices.Index(this.sizes, lowest)
	if -1 != idx && len(this.sizes) > idx+1 {
		highest = this.sizes[idx+1]
	}
	if 1 >= (highest - lowest) {
		return
	}
	part := (highest - lowest) / PKTS
	if 0 == part {
		part = 1
	}
	for i := 0; i < PKTS+1; i++ {
		sz := uint32(lowest) + uint32(i)*part
		if PKTS == i { // in case of round off
			sz = highest
		}
		this.addSize(sz)
	}
}

func (this *MtuProber) addSize(sz uint32) {
	if sz > 65535 {
		sz = 65535
	}
	if !slices.Contains(this.sizes, sz) {
		this.sizes = append(this.sizes, sz)
		slices.Sort(this.sizes)
	}
}

func (this *MtuProber) hasNextSize(sz uint32) bool {
	idx := slices.Index(this.sizes, sz)
	return -1 != idx && len(this.sizes) > idx+1 && sz+1 == this.sizes[idx+1]
}

//
//
//

// MTU discovery server - echos back pkts sent by MtuProber client
type MtuEchoer struct {

	//
	// if set, log output
	//
	Name string

	//
	// if set, called on receipt of each pkt
	//
	OnPacket func(pkt []byte, from syscall.Sockaddr) (err error)

	sock      *Socket
	poller    Poller
	closeSock bool
}

// sock should be connected, constructed similarly as per NewSock
func (this *MtuEchoer) SetSock(sock *Socket) {
	if nil != this.sock {
		panic("sock already set!")
	}
	this.sock = sock
}

// add a UDP socket, suitable for use to discover MTU
func (this *MtuEchoer) NewSock(near Address) (err error) {
	if nil != this.sock {
		panic("sock already set!")
	}
	sock := &Socket{}
	err = sock.
		SetNearAddress(near).
		ConstructUdp().
		SetOptReuseAddr().
		//
		// if not large enough, pkts will be dropped
		//
		SetOptRcvBuf(4 * 1024 * 1024).
		SetOptMtuDiscover(MtuDiscoProbe).
		Bind().
		Error
	if err != nil {
		return
	}
	this.sock = sock
	this.closeSock = true
	return
}

func (this *MtuEchoer) Close() {
	this.poller.Close()
	if this.closeSock {
		this.sock.Close()
	}
}

// echo datagrams for the duration, or forever if timeout not positive
func (this *MtuEchoer) Echo(timeout time.Duration) (err error) {
	if nil == this.sock {
		panic("No sock set!")
	}
	if !this.poller.IsStarted() {
		err = this.setupPoller()
		if err != nil {
			return
		}
	}

	if 0 <= timeout {
		_, err = this.poller.PollFor(timeout)
	} else {
		_, err = this.poller.PollForever()
	}
	return
}

func (this *MtuEchoer) setupPoller() (err error) {

	var nearAddr Address
	err = this.sock.GetNearAddress(&nearAddr).Error
	if err != nil {
		return
	} else if nearAddr.AsIp().IsUnspecified() {
		return errors.New("Near socket address cannot be a wildcard addr")
	}

	recvBuff := make([]byte, 65536)

	defer func() {
		if err != nil {
			this.poller.Close()
		}
	}()

	err = this.poller.Open()
	if err != nil {
		return
	}

	err = this.poller.Add(&Polled{
		Sock: this.sock,

		OnInput: func(p *Polled) (ok bool, err error) {
			nread, from, err := this.sock.RecvFrom(recvBuff, syscall.MSG_DONTWAIT)
			if err != nil {
				return false, err
			} else if 0 > nread {
				return false, fmt.Errorf("got negative nread, but no error!")
			}
			if nil != this.OnPacket {
				err = this.OnPacket(recvBuff[:nread], from)
				if err != nil {
					return
				}
			} else if 0 != len(this.Name) {
				ulog.Printf("%s: MTU echoing %d", this.Name, nread)
			}

			//
			// note: we ensure that the nearAddr is not a wildcard addr (0.0.0.0)
			// so that responses from this sock will have a correct IP on them
			//
			err = this.sock.SendTo(recvBuff[:nread], 0, from)
			if err != nil {
				return
			}
			return true, nil
		},
	})
	return
}
