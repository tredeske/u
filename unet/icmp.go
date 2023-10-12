package unet

// see /usr/include/linux/icmp.h
const (
	// ICMP v4 types
	//
	// seen, for example, in unix.SockExtendedErr.Type
	ICMP_ECHOREPLY      = 0  // Echo Reply
	ICMP_DEST_UNREACH   = 3  // Destination Unreachable
	ICMP_SOURCE_QUENCH  = 4  // Source Quench
	ICMP_REDIRECT       = 5  // Redirect (change route)
	ICMP_ECHO           = 8  // Echo Request
	ICMP_TIME_EXCEEDED  = 11 // Time Exceeded
	ICMP_PARAMETERPROB  = 12 // Parameter Problem
	ICMP_TIMESTAMP      = 13 // Timestamp Request
	ICMP_TIMESTAMPREPLY = 14 // Timestamp Reply
	ICMP_INFO_REQUEST   = 15 // Information Request
	ICMP_INFO_REPLY     = 16 // Information Reply
	ICMP_ADDRESS        = 17 // Address Mask Request
	ICMP_ADDRESSREPLY   = 18 // Address Mask Reply
	NR_ICMP_TYPES       = 18

	// Codes for ICMP_DEST_UNREACH
	// seen, for example, in unix.SockExtendedErr.Code
	ICMP_NET_UNREACH    = 0 // Network Unreachable
	ICMP_HOST_UNREACH   = 1 // Host Unreachable
	ICMP_PROT_UNREACH   = 2 // Protocol Unreachable
	ICMP_PORT_UNREACH   = 3 // Port Unreachable
	ICMP_FRAG_NEEDED    = 4 // Fragmentation Needed/DF set
	ICMP_SR_FAILED      = 5 // Source Route failed
	ICMP_NET_UNKNOWN    = 6
	ICMP_HOST_UNKNOWN   = 7
	ICMP_HOST_ISOLATED  = 8
	ICMP_NET_ANO        = 9
	ICMP_HOST_ANO       = 10
	ICMP_NET_UNR_TOS    = 11
	ICMP_HOST_UNR_TOS   = 12
	ICMP_PKT_FILTERED   = 13 // Packet filtered
	ICMP_PREC_VIOLATION = 14 // Precedence violation
	ICMP_PREC_CUTOFF    = 15 // Precedence cut off
	NR_ICMP_UNREACH     = 15 // instead of hardcoding immediate value

	// Codes for ICMP_REDIRECT
	ICMP_REDIR_NET     = 0 // Redirect Net
	ICMP_REDIR_HOST    = 1 // Redirect Host
	ICMP_REDIR_NETTOS  = 2 // Redirect Net for TOS
	ICMP_REDIR_HOSTTOS = 3 // Redirect Host for TOS

	// Codes for ICMP_TIME_EXCEEDED
	ICMP_EXC_TTL      = 0 // TTL count exceeded
	ICMP_EXC_FRAGTIME = 1 // Fragment Reass time exceeded
)

// see /usr/include/linux/icmpv6.h
const (
	// ICMP v6 Types
	ICMPV6_DEST_UNREACH = 1
	ICMPV6_PKT_TOOBIG   = 2
	ICMPV6_TIME_EXCEED  = 3
	ICMPV6_PARAMPROB    = 4

	ICMPV6_ECHO_REQUEST = 128
	ICMPV6_ECHO_REPLY   = 129

	// codes for ICMPV6_DEST_UNREACH
	ICMPV6_NOROUTE        = 0
	ICMPV6_ADM_PROHIBITED = 1
	ICMPV6_NOT_NEIGHBOUR  = 2
	ICMPV6_ADDR_UNREACH   = 3
	ICMPV6_PORT_UNREACH   = 4
	ICMPV6_POLICY_FAIL    = 5
	ICMPV6_REJECT_ROUTE   = 6

	// codes for ICMPV6_TIME_EXCEED
	ICMPV6_EXC_HOPLIMIT = 0
	ICMPV6_EXC_FRAGTIME = 1

	// codes for ICMPV6_PARAMPROB
	ICMPV6_HDR_FIELD   = 0
	ICMPV6_UNK_NEXTHDR = 1
	ICMPV6_UNK_OPTION  = 2
	ICMPV6_HDR_INCOMP  = 3
)

// ICMP v4
type IcmpHdr struct {
	Type     uint8
	Code     uint8
	Checksum uint16
	Payload  uint32
}

func (icmp *IcmpHdr) EchoId() uint16 { // convert from BE
	return uint16(((icmp.Payload >> 8) & 0xff) | (icmp.Payload << 8))
}
func (icmp *IcmpHdr) EchoSeq() uint16 { // convert from BE
	return uint16((icmp.Payload >> 24) | ((icmp.Payload >> 8) & 0xff00))
}
func (icmp *IcmpHdr) Mtu() uint16 { // convert from BE
	return uint16((icmp.Payload >> 24) | ((icmp.Payload >> 8) & 0xff00))
}
func (icmp *IcmpHdr) Gateway() uint32 { // convert from BE
	return Htonl(icmp.Payload)
}

//func (icmp *IcmpHdr) EchoId() uint16  { return uint16(icmp.Payload) }
//func (icmp *IcmpHdr) EchoSeq() uint16 { return uint16(icmp.Payload >> 16) }
//func (icmp *IcmpHdr) Mtu() uint16     { return uint16(icmp.Payload >> 16) }
//func (icmp *IcmpHdr) Gateway() uint32 { return icmp.Payload }

// ICMP v6
type IcmpV6Hdr struct {
	Type     uint8
	Code     uint8
	Checksum uint16
	Payload  uint32
}

func (icmp *IcmpV6Hdr) EchoId() uint16 { // convert from BE
	return uint16(((icmp.Payload >> 8) & 0xff) | (icmp.Payload << 8))
}
func (icmp *IcmpV6Hdr) EchoSeq() uint16 { // convert from BE
	return uint16((icmp.Payload >> 24) | ((icmp.Payload >> 8) & 0xff00))
}
func (icmp *IcmpV6Hdr) Mtu() uint32 { // convert from BE
	return Htonl(icmp.Payload)
}

//func (icmp *IcmpV6Hdr) EchoId() uint16  { return uint16(icmp.Payload) }
//func (icmp *IcmpV6Hdr) EchoSeq() uint16 { return uint16(icmp.Payload >> 16) }
//func (icmp *IcmpV6Hdr) Mtu() uint32     { return icmp.Payload }
