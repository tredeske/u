package uconfig

import (
	"errors"
	"fmt"
	"math/bits"
	"net"
	"regexp"
)

// use with Section.GetFloat64, Chain.GetFloat64 to validate float
type FloatValidator func(float64) error

// use with Section.GetInt, Chain.GetInt to validate signed int
type IntValidator func(int64) error

// use with Section.GetUInt, Chain.GetUInt to validate unsigned int
type UIntValidator func(uint64) error

// use with Section.GetString, Chain.GetString to validate string
type StringValidator func(string) error

var (
	ErrStringBlank    = errors.New("String value empty")
	ErrStringNotBlank = errors.New("String value not empty")
)

// return a range validator for GetInt
func FloatRange(min, max float64) FloatValidator {
	if max < min {
		panic("max cannot be less than min")
	}
	return func(v float64) (err error) {
		if v < min {
			err = fmt.Errorf("value (%f) less than min (%f)", v, min)
		} else if v > max {
			err = fmt.Errorf("value (%f) greater than max (%f)", v, max)
		}
		return
	}
}

// return a range validator for GetInt
func IntRange(min, max int64) IntValidator {
	if max < min {
		panic("max cannot be less than min")
	}
	return func(v int64) (err error) {
		if v < min {
			err = fmt.Errorf("value (%d) less than min (%d)", v, min)
		} else if v > max {
			err = fmt.Errorf("value (%d) greater than max (%d)", v, max)
		}
		return
	}
}

// return a range validator for GetUInt
func UIntRange(min, max uint64) UIntValidator {
	if max < min {
		panic("max cannot be less than min")
	}
	return func(v uint64) (err error) {
		if v < min {
			err = fmt.Errorf("value (%d) less than min (%d)", v, min)
		} else if v > max {
			err = fmt.Errorf("value (%d) greater than max (%d)", v, max)
		}
		return
	}
}

// return a validator to error if v is not positive
func IntPos() IntValidator {
	return func(v int64) (err error) {
		if v <= 0 {
			err = fmt.Errorf("int is not positive (is %d)", v)
		}
		return
	}
}

// return a validator to error if v is negative
func IntNonNeg() IntValidator {
	return func(v int64) (err error) {
		if v < 0 {
			err = fmt.Errorf("int is negative (is %d)", v)
		}
		return
	}
}

// return a validator to error if v not at least min
func IntAtLeast(min int64) IntValidator {
	return func(v int64) (err error) {
		if v < min {
			err = fmt.Errorf("int is not at least %d (is %d)", min, v)
		}
		return
	}
}

// return a validator to error if v not a power of 2 > 0
func IntPow2() IntValidator {
	return func(v int64) (err error) {
		if 1 > v || 1 != bits.OnesCount64(uint64(v)) {
			err = fmt.Errorf("int (%d) is not a power of 2 > 0", v)
		}
		return
	}
}

// a StringValidator to verify string blank
func StringBlank() StringValidator {
	return func(v string) (err error) {
		if 0 != len(v) {
			err = ErrStringNotBlank
		}
		return
	}
}

// a StringValidator to verify string not blank
func StringNotBlank() StringValidator {
	return func(v string) (err error) {
		if 0 == len(v) {
			err = ErrStringBlank
		}
		return
	}
}

// a StringValidator to verify string matches regular expression
func StringMatch(re *regexp.Regexp) StringValidator {
	return func(v string) (err error) {
		if 0 == len(v) {
			err = errors.New("value is blank")
		} else if !re.MatchString(v) {
			err = fmt.Errorf("'%s' does not match %s", v, re.String())
		}
		return
	}
}

// either one or other must be true
func StringOr(one, other StringValidator) StringValidator {
	return func(v string) (err error) {
		err = one(v)
		if nil == err {
			return
		}
		err = other(v)
		return
	}
}

// create a StringValidator to verify value is blank or valid
func StringBlankOr(validator StringValidator) StringValidator {
	return func(v string) (err error) {
		if 0 != len(v) {
			err = validator(v)
		}
		return
	}
}

// create a StringValidator to verify value is one of listed
func StringOneOf(choices ...string) StringValidator {
	return func(v string) (err error) {
		for _, choice := range choices {
			if choice == v {
				return
			}
		}
		return fmt.Errorf("String (%s) not in %#v", v, choices)
	}
}

// see RFC 952 and 1123 (section 2.1)
// since caps are folded to lower case, we insist all lower case to avoid
// confusion.
var validHostname_ = regexp.MustCompile(
	`^(?:[a-z0-9][a-z0-9\-]{0,61}[a-z0-9])(?:\.(?:[a-z0-9]|[a-z0-9][a-z0-9\-]{0,61}[a-z0-9]))*$`)

// check with net.ParseIP first to rule out if it is an IP addr
func ValidHostname(s string) bool {
	return 256 > len(s) && 1 < len(s) && validHostname_.MatchString(s)
}

// create a StringValidator to verify value is an IP or hostname
func StringHostOrIp() StringValidator {
	return func(v string) (err error) {
		if nil == net.ParseIP(v) && !ValidHostname(v) {
			err = fmt.Errorf("String (%s) not a valid IP or hostname", v)
		}
		return
	}
}

// create a StringValidator to verify value is an IP
func StringIp() StringValidator {
	return func(v string) (err error) {
		if nil == net.ParseIP(v) {
			err = fmt.Errorf("String (%s) not a valid IP", v)
		}
		return
	}
}

/*
// create a StringValidator to verify value not one of listed
func StringNot(invalid ...string) StringValidator {
	return func(v string) (err error) {
		for _, iv := range invalid {
			if iv == v {
				return fmt.Errorf("String cannot be (%s)", v)
			}
		}
		return
	}
}
*/
