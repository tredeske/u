package unet

import (
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/tredeske/u/ulog"
)

// These are the known IP DSCP / TOS names and codes.
func HelpDscp() string {
	b := strings.Builder{}
	b.Grow(80 * (len(dscpLookup_) + 12))
	b.WriteString(
		`IP DSCP/TOS (differentiated services / type of service) bits for
prioritizing IP packets.

DSCP: differentiated services code point (replaces TOS)
TOS:  type of service (semi-deprecated)

Can check these against /usr/include/netinet/ip.h

DSCP defines several traffic classes. The primary DSCP classes are,
per RFC 4594, and RFC 8622:

- Lower-Effort (LE)
- Default Forwarding (DF)
- Assured Forwarding (AF)
- Expedited Forwarding (EF)
- Class Selector (CS)

In the AF (assured forwarding) classes, drop priority is susceptibility
to dropping, so AF11 is less prone to being dropped than AF13.

You can set the value by Name or Code

Low 2 bits are reserved for ECN - do not set them!

        Name          Code                Note
--------------------  ----  -------------------------------------
`)
	for _, v := range dscpLookup_ {
		b.WriteString(fmt.Sprintf("%20s  0x%02x  %s\n", v.Name, v.Code, v.Note))
	}
	return b.String()
}

type dscp_ struct {
	Name string
	Code byte
	Note string
}

var dscpLookup_ = []dscp_{
	{"CS0", 0x00, "normal (class 0)"},
	{"DF", 0x00, "default forwarding (old TOS 'Normal-Service')"},
	{"LE", 0x04, "low effort bulk data"},
	{"Maximize-Throughput", 0x08, "maximize throughput (old TOS naming)"},
	{"Minimize-Delay", 0x10, "minimize delay (old TOS naming)"},
	{"CS1", 0x20, "low priority data"},
	{"AF11", 0x28, "high throughput (low drop priority)"},
	{"AF12", 0x30, "high throughput (med drop priority)"},
	{"AF13", 0x38, "high throughput (high drop priority)"},
	{"CS2", 0x40, "low latency"},
	{"AF21", 0x48, "low latency (low drop priority)"},
	{"AF22", 0x50, "low latency (med drop priority)"},
	{"AF23", 0x58, "low latency (high drop priority)"},
	{"CS3", 0x60, "video"},
	{"AF31", 0x68, "multimedia streaming (low drop priority)"},
	{"AF32", 0x70, "multimedia streaming (med drop priority)"},
	{"AF33", 0x78, "multimedia streaming (high drop priority)"},
	{"CS4", 0x80, "real time interactive"},
	{"AF41", 0x88, "multimedia conferencing (low drop priority)"},
	{"AF42", 0x90, "multimedia conferencing (med drop priority)"},
	{"AF43", 0x98, "multimedia conferencing (high drop priority)"},
	{"CS5", 0xa0, "signalling (ip telephony)"},
	{"voice-admit", 0xb0, "voice"},
	{"EF", 0xb8, "telephony, expedited forwarding"},
	{"CS6", 0xc0, "network routing / control"},
	{"CS7", 0xe0, "reserved"},
}

// uconfig validator
func ValidDscpTos(value string) (err error) {
	_, err = LookupDscpTos(value)
	return
}

// DSCP / TOS value: can be one of the known DSCP types or just a numeric.
//
// Valid values that all mean the same thing:
// - AF42       - by name
// - 144        - decimal
// - 0x90       - hex
// - 0o220      - octal
// - 0b10010000 - binary
func LookupDscpTos(value string) (code byte, err error) {
	if 0 == len(value) {
		code = 0
	} else if '0' > value[0] || '9' < value[0] { // not a number
		dscp, found := findDscpByName(value)
		code = dscp.Code
		if !found {
			err = fmt.Errorf("DSCP value (%s) not found", value)
		}
	} else { // some sort of number
		var u uint64
		u, err = strconv.ParseUint(value, 0, 8)
		if nil == err {
			code = byte(u)
			err = ValidDscpTosCode(code)
		}
	}
	return
}

// CS7 (0xe0) is the highest known dscp code, but since we operate on some
// strange networks, we allow values above that
func ValidDscpTosCode(code byte) (err error) {
	if code > 0xfc || 0 != (code&0x03) {
		err = errors.New("DSCP must be a 1 byte value with low order 2 bits unset")
	} else if !KnownDscpTosCode(code) {
		ulog.Warnf("IP DSCP/TOS code 0x%x does not match a known value", code)
	}
	return
}

func findDscpByName(name string) (dscp dscp_, ok bool) {
	for i, N := 0, len(dscpLookup_); i < N; i++ {
		if dscpLookup_[i].Name == name {
			return dscpLookup_[i], true
		}
	}
	return
}

func KnownDscpTosCode(code byte) bool {
	for i, N := 0, len(dscpLookup_); i < N; i++ {
		if dscpLookup_[i].Code == code {
			return true
		}
	}
	return false
}
