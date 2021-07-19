package uconfig

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

//
// convert a string to an int64, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
//
// also handled:
// - if string begins with "0x", then it is hex
// - if string begins with "0", then it is octal
//
func Int64FromSiString(s string) (rv int64, err error) {
	var mult int64
	mult, s, err = multiplier(s)
	if err != nil {
		return
	}
	if strings.Contains(s, ".") {
		var f float64
		f, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return
		}
		rv = int64(f * float64(mult))

	} else {
		rv, err = strconv.ParseInt(s, 0, 64)
		if err != nil {
			return
		}
		rv *= mult
	}
	return
}

//
// convert a string to an uint64, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
//
// also handled:
// - if string begins with "0x", then it is hex
// - if string begins with "0", then it is octal
//
func UInt64FromSiString(s string) (rv uint64, err error) {
	var mult int64
	mult, s, err = multiplier(s)
	if err != nil {
		return
	}
	if strings.Contains(s, ".") {
		var f float64
		f, err = strconv.ParseFloat(s, 64)
		if err != nil {
			return
		}
		rv = uint64(f * float64(mult))

	} else {
		rv, err = strconv.ParseUint(s, 0, 64)
		if err != nil {
			return
		}
		rv *= uint64(mult)
	}
	return
}

//
// convert a string to an float64, taking into account SI prefixes:
// K, M, G, T, P: base 1000 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
//
func Float64FromSiString(s string) (rv float64, err error) {
	var mult int64
	mult, s, err = multiplier(s)
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

//
// K, M, G, T, P: base 1000 multipliers
// Ki, Mi, Gi, Ti, Pi: base 1024 multipliers
//
func multiplier(s string) (mult int64, rv string, err error) {
	n := len(s)
	if 0 == n {
		err = errors.New("Cannot convert empty string to number")
		return
	}
	rv = s

	//
	// hex numbers do not have si
	//
	if 2 < n && '0' == s[0] && ('x' == s[1] || 'X' == s[1]) {
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
	mult = int64(1)
	last := s[n-1]
	if 'i' == last {
		if 1 == n {
			err = errors.New("Cannot convert 'i' to number")
			return
		}
		last = s[n-2]
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
		rv = s[:n-2]

	} else if '0' > last || '9' < last { // not a number
		switch last {
		case 'p', 'P':
			mult = 1000000000000000
		case 't', 'T':
			mult = 1000000000000
		case 'g', 'G':
			mult = 1000000000
		case 'm', 'M':
			mult = 1000000
		case 'k', 'K':
			mult = 1000
		default:
			err = fmt.Errorf("Unknown SI unit in string: %s", s)
			return
		}
		rv = s[:n-1]
	}
	return
}
