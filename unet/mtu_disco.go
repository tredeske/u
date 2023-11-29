package unet

import (
	"errors"
	"fmt"
	"slices"
	"syscall"
	"time"

	"github.com/tredeske/u/ulog"
)

// client side of MTU discovery - sends UDP probes and sees what happens
type MtuProber struct {

	//
	// if set, log output
	//
	Name string

	//
	// set this to create initial buffer, or one will be created w/o your help
	//
	OnStart func(size uint16) (space []byte)

	//
	// if set, is called to create pkt from space prior to each send
	//
	BeforeSend func(size uint16, space []byte) (pkt []byte, err error)

	//
	// if set, is called after receving each pkt
	//
	AfterRecv func(pkt []byte) (err error)
	sock      *Socket
	farAddr   Address

	//
	// if set, set smallest probe size.  otherwise, smallest compliant will be used
	//
	MtuMin uint16

	//
	// if set, set largest probe size.  otherwise, reasonable max will be used
	//
	MtuMax    uint16
	closeSock bool
}

// sock should be connected, constructed similarly as per NewSock
func (this *MtuProber) SetSock(sock *Socket) {
	if nil != this.sock {
		panic("sock already set!")
	}
	this.sock = sock
}

// add a UDP socket, suitable for use to discover MTU
func (this *MtuProber) NewSock(src, dst Address) (err error) {
	if nil != this.sock {
		panic("sock already set!")
	}
	sock := &Socket{}
	err = sock.
		SetNearAddress(&src).
		SetFarAddress(&dst).
		ConstructUdp().
		SetOptReuseAddr().
		SetOptMtuDiscover(MtuDiscoProbe).
		Bind().
		Connect().
		Error
	if err != nil {
		return
	}
	this.sock = sock
	this.closeSock = true
	return
}

func (this *MtuProber) Close() {
	if this.closeSock && nil != this.sock {
		this.sock.Close()
	}
	this.sock = nil
}

// perform probing until PMTU is known
func (this *MtuProber) Probe() (pmtu int, err error) {

	defer this.Close()

	err = this.ensureDefaults()
	if err != nil {
		return
	}

	//
	// pre check kernel PMTU cache
	//
	/*
		err = this.sock.GetOptMtu(&pmtu).Error
		if err != nil {
			return
		} else if 0 < pmtu {
			return
		}
		pmtu = -1
	*/

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

	if nil == this.BeforeSend {
		this.BeforeSend = func(size uint16, space []byte) (pkt []byte, err error) {
			return space[:size], nil
		}
	}

	recvBuff := make([]byte, len(space))
	overhead := uint16(this.sock.IpOverhead() + UDP_OVERHEAD)
	lowest := uint32(this.MtuMin - overhead)
	highest := uint32(this.MtuMax - overhead)
	addSizes := true

	poller := &SinglePoller{

		OnErrorQ: func(fd int) (ok bool, err error) {
			if 0 != len(this.Name) {
				ulog.Printf("%s: MTU Probe got error input!", this.Name)
			}
			return true, nil
		},

		OnInput: func(fd int) (ok bool, err error) {
			nread, from, err := this.sock.RecvFrom(recvBuff, syscall.MSG_DONTWAIT)
			if 0 != len(this.Name) {
				ulog.Printf("%s: MTU Probe recv %d", this.Name, nread)
			}
			if err != nil {
				return false, err
			}
			var fromAddr Address
			fromAddr.FromSockaddr(from)
			if this.farAddr != fromAddr {
				if 0 != len(this.Name) {
					ulog.Printf("%s: MTU Probe got stray pkt from %s",
						this.Name, fromAddr.String())
				}
				return true, nil
			}
			sz := uint32(nread)
			if sz > highest {
				highest = sz
				addSizes = true
			}
			if sz > lowest {
				lowest = sz
				if lowest == highest { // converged
					return false, nil // stop search
				}
				addSizes = true
			}
			if nil != this.AfterRecv {
				err = this.AfterRecv(recvBuff[:nread])
				if err != nil {
					return
				}
			}
			return true, nil
		},
	}

	err = poller.Open(this.sock)
	if err != nil {
		return
	}
	defer poller.Close()

	//
	// send pkts and collect responses until we have pmtu
	//
	sizes := make([]uint32, 0, 128)
	var pkt []byte
	for lowest != highest {
		if 0 != len(this.Name) {
			ulog.Printf("%s: MTU Probe: lowest = %d, highest = %d",
				this.Name, lowest, highest)
		}
		if addSizes {
			sizes = this.addSizes(lowest, highest, sizes)
			addSizes = false
		}
		for _, sz := range sizes {
			pkt, err = this.BeforeSend(uint16(sz), space)
			if err != nil {
				return
			} else if sz != uint32(len(pkt)) {
				err = errors.New("BeforeSend must create correct sized packet")
				return
			}
			if 0 != len(this.Name) {
				ulog.Printf("%s: MTU Probe: sending %d", this.Name, sz)
			}
			err = this.sock.Send(pkt, 0)
			if err != nil {
				if errors.Is(err, syscall.EMSGSIZE) { // got ICMP back with MTU
					err = this.sock.GetOptMtu(&pmtu).Error
					if err != nil {
						return
					} else if 0 < pmtu {
						highest = uint32(pmtu)
						if lowest == highest {
							return
						}
					}
					break

					//
					// ignore temporary condition where peer not up yet
					//
				} else if errors.Is(err, syscall.ECONNREFUSED) {
					continue
				}
				return
			}
		}

		_, err = poller.PollFor(120 * time.Millisecond)
		if err != nil {
			return
		}
	}
	pmtu = int(highest)
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

	minMtu := uint16(576) // ipv4 (RFC 791)
	if this.sock.IsIpv6() {
		minMtu = 1280 // RFC 2460
	}
	if 0 == this.MtuMax {
		this.MtuMax = 9000 // convention. most (all?) equipment supports 9216
	} else if this.MtuMax < minMtu {
		return fmt.Errorf("MtuMax must be at least %d", minMtu)
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

func (this *MtuProber) addSizes(lowest, highest uint32, sizes []uint32) []uint32 {
	const PKTS = 8 // pow of 2

	if 65535 < highest {
		panic("highest out of range")
	} else if 64 > lowest {
		panic("lowest out of range")
	}

	if idx := slices.Index(sizes, lowest); -1 != idx && len(sizes) > idx+1 {
		highest = sizes[idx+1]
	}
	if 1 >= (highest - lowest) {
		return sizes
	}
	part := (highest - lowest) / PKTS
	if 0 == part {
		part = 1
	}
	for i := 0; i < PKTS+1; i++ {
		sz := lowest + uint32(i)*part
		if PKTS == i { // in case of round off
			sz = highest
		}
		if !slices.Contains(sizes, sz) {
			sizes = append(sizes, sz)
			slices.Sort(sizes)
		}
	}
	return sizes
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
	poller    *SinglePoller
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
		SetNearAddress(&near).
		ConstructUdp().
		SetOptReuseAddr().
		SetOptMtuDiscover(MtuDiscoDo).
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
	if nil != this.poller {
		this.poller.Close()
	}
	if this.closeSock {
		this.sock.Close()
	}
}

// echo datagrams for the duration, or forever if timeout not positive
func (this *MtuEchoer) Echo(timeout time.Duration) (err error) {
	if nil == this.sock {
		panic("No sock set!")
	}
	if nil == this.poller {
		err = this.setupPoller()
		if err != nil {
			return
		}
	}

	if 0 < timeout {
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
	poller := &SinglePoller{

		OnErrorQ: func(fd int) (ok bool, err error) {
			if 0 != len(this.Name) {
				ulog.Printf("%s: MTU echo got error input!", this.Name)
			}
			return true, nil
		},

		OnInput: func(fd int) (ok bool, err error) {
			nread, from, err := this.sock.RecvFrom(recvBuff, syscall.MSG_DONTWAIT)
			if 0 != len(this.Name) {
				ulog.Printf("%s: MTU echo got %d", this.Name, nread)
			}
			if err != nil {
				return false, err
			}
			if nil != this.OnPacket {
				err = this.OnPacket(recvBuff[:nread], from)
				if err != nil {
					return
				}
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
	}

	err = poller.Open(this.sock)
	if err != nil {
		return
	}
	this.poller = poller

	return
}
