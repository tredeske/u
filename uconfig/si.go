package uconfig

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unsafe"
)

type (
	Int interface {
		int | int64 | int32 | int16 | int8
	}

	UInt interface { // and also byte, but we can't list that
		uint | uint64 | uint32 | uint16 | uint8 | uintptr
	}
)

// convert a string to an int64, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
//
// also handled:
// - if string begins with "0x", then it is hex
// - if string begins with "0", then it is octal
func IntFromSiString[I Int](s string, rv *I) (err error) {
	return intFromSiString(s, rv, mkAny_)
}

// convert a string to an int64 byte size, taking into account SI prefixes:
// K, M, G, T, P: treat as base 1024 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
//
// also handled:
// - if string begins with "0x", then it is hex
// - if string begins with "0", then it is octal
func IntFromByteSizeString[I Int](s string, rv *I) (err error) {
	return intFromSiString(s, rv, mkSize_)
}

// convert a string to an int64 bit rate, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
//
// also handled:
// - if string begins with "0x", then it is hex
// - if string begins with "0", then it is octal
func IntFromBitRateString[I Int](s string, rv *I) (err error) {
	return intFromSiString(s, rv, mkRate_)
}

// convert a string to an int64, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
//
// also handled:
// - if string begins with "0x", then it is hex
// - if string begins with "0", then it is octal
func intFromSiString[I Int](s string, rv *I, kind multKind_) (err error) {
	var mult int64
	bits := int(unsafe.Sizeof(rv) * 8)
	hi := int64(1<<(bits-1) - 1)  // from math.MaxInt
	lo := int64(-1 << (bits - 1)) // from math.MinInt
	mult, s, err = multiplier(s, kind)
	if err != nil {
		return
	}
	if strings.Contains(s, ".") {
		var f float64
		f, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return
		}
		f *= float64(mult)
		if f > float64(hi) || f < float64(lo) {
			err = fmt.Errorf("%T cannot hold %s", *rv, s)
			return
		}
		*rv = I(f)

	} else {
		var i int64
		i, err = strconv.ParseInt(s, 0, bits)
		if err != nil {
			return
		} else if (i > 0 && hi/mult < i) || i < 0 && lo/mult > i {
			err = fmt.Errorf("%T cannot hold %s", *rv, s)
			return
		}
		*rv = I(i * mult)
	}
	return
}

// convert a string to an uint64 bit rate, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
//
// also handled:
// - if string begins with "0x", then it is hex
// - if string begins with "0", then it is octal
func UIntFromBitRateString[U UInt](s string, rv *U) (err error) {
	return uintFromSiString(s, rv, mkRate_)
}

// convert a string to an uint64 byte size, taking into account SI prefixes:
// K, M, G, T, P: treat as base 1024 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
//
// also handled:
// - if string begins with "0x", then it is hex
// - if string begins with "0", then it is octal
func UIntFromByteSizeString[U UInt](s string, rv *U) (err error) {
	return uintFromSiString(s, rv, mkSize_)
}

// convert a string to an uint64, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
//
// also handled:
// - if string begins with "0x", then it is hex
// - if string begins with "0", then it is octal
func UIntFromSiString[U UInt](s string, rv *U) (err error) {
	return uintFromSiString(s, rv, mkAny_)
}

// convert a string to an uint64, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
//
// also handled:
// - if string begins with "0x", then it is hex
// - if string begins with "0", then it is octal
func uintFromSiString[U UInt](s string, rv *U, kind multKind_) (err error) {
	var mult int64
	bits := int(unsafe.Sizeof(rv) * 8)
	hi := uint64(1<<bits - 1)
	mult, s, err = multiplier(s, kind)
	if err != nil {
		return
	} else if s[0] == '-' {
		err = fmt.Errorf("Cannot parse negative number to %T", *rv)
		return
	}
	if strings.Contains(s, ".") {
		var f float64
		f, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return
		} else if 0. > f {
			err = fmt.Errorf("Cannot parse negative number to %T", *rv)
			return
		}
		f *= float64(mult)
		if f > float64(hi) {
			err = fmt.Errorf("%T cannot contain %s", *rv, s)
			return
		}
		*rv = U(f)

	} else {
		var u uint64
		u, err = strconv.ParseUint(s, 0, bits)
		if err != nil {
			return
		} else if hi/uint64(mult) < u {
			err = fmt.Errorf("%T cannot contain %s", *rv, s)
			return
		}
		*rv = U(u * uint64(mult))
	}
	return
}

// convert a string to an float64 bit rate, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
func Float64FromBitRateString(s string) (rv float64, err error) {
	return float64FromSiString(s, mkRate_)
}

// convert a string to an float64 byte size, taking into account SI prefixes:
// K, M, G, T, P: treat as base 1024 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
func Float64FromByteSizeString(s string) (rv float64, err error) {
	return float64FromSiString(s, mkSize_)
}

// convert a string to an float64, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
func Float64FromSiString(s string) (rv float64, err error) {
	return float64FromSiString(s, mkAny_)
}

// convert a string to an float64, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
func float64FromSiString(s string, kind multKind_) (rv float64, err error) {
	var mult int64
	mult, s, err = multiplier(s, kind)
	if err != nil {
		return
	}
	rv, err = strconv.ParseFloat(s, 64)
	if err != nil {
		return
	}
	rv *= float64(mult)
	return
}

type multKind_ int

const (
	mkAny_  multKind_ = 0 // just use the SI suffix as-is
	mkRate_ multKind_ = 1 // interpret/enforce for bit rate
	mkSize_ multKind_ = 2 // interpret/enforce for byte amount
)

// K, M, G, T, P: base 1000 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
// if kind is mkRate, then Ki, Mi, etc are not allowed
// if kind is mkSize, then K, M, etc are interpretted as Ki, Mi, etc
func multiplier(s string, kind multKind_) (mult int64, rv string, err error) {
	n := len(s)
	if 0 == n {
		err = errors.New("Cannot convert empty string to number")
		return
	}
	rv = s
	mult = int64(1)
	last := s[n-1]

	//
	// hex numbers do not have si
	//
	if 2 < n && '0' == s[0] && ('x' == s[1] || 'X' == s[1]) {
		return
	} else if '0' <= last && '9' >= last { // a number - no suffix
		return
	}

	//
	// detect SI units
	// k = 1000
	// ki = 1024
	// m = 1000000
	// mi = 1024*1024
	// ...
	//
	if 'i' == last {
		if 2 >= n {
			err = fmt.Errorf("Cannot convert '%s' to a number", s)
			return
		} else if mkRate_ == kind {
			err = fmt.Errorf("Cannot use '%s' as a bit rate", s)
			return
		}
		kind = mkSize_
		last = s[n-2]
		rv = s[:n-2]

	} else {
		rv = s[:n-1]
	}

	if mkSize_ == kind {
		switch last {
		case 'p', 'P':
			mult = 1 << 50
		case 't', 'T':
			mult = 1 << 40
		case 'g', 'G':
			mult = 1 << 30
		case 'm', 'M':
			mult = 1 << 20
		case 'k', 'K':
			mult = 1 << 10
		default:
			err = fmt.Errorf("Unknown SI unit in string: %s", s)
			return
		}

	} else {
		switch last {
		case 'p', 'P':
			mult = 1_000_000_000_000_000
		case 't', 'T':
			mult = 1_000_000_000_000
		case 'g', 'G':
			mult = 1_000_000_000
		case 'm', 'M':
			mult = 1_000_000
		case 'k', 'K':
			mult = 1_000
		default:
			err = fmt.Errorf("Unknown SI unit in string: %s", s)
			return
		}
	}
	return
}
