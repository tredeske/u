package uconfig

import (
	"errors"
	"fmt"
	"math/bits"
	"net"
	nurl "net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/tredeske/u/uerr"
)

/*

Not sure how useful this would be since Go does not allow parameterized methods
so we cannot have a Section.GetNumber[N Number](...) method.  Yuck.

type Number interface {
	constraints.Integer | constraints.Float
}
type NumberValidator[N Number] func(N) error

// return a range validator for numbers
func IsIn[N Number](min, max N) NumberValidator[N] {
	if max < min {
		panic("max cannot be less than min")
	}
	return func(v N) (err error) {
		if v < min {
			err = fmt.Errorf("value (%v) less than min (%v)", v, min)
		} else if v > max {
			err = fmt.Errorf("value (%v) greater than max (%v)", v, max)
		}
		return
	}
}

// return a validator to error if v is not positive
func IsPos[N Number]() NumberValidator[N] {
	return func(v N) (err error) {
		if v <= 0 {
			err = fmt.Errorf("number is not positive (is %v)", v)
		}
		return
	}
}

// return a validator to error if v is negative
func IsNonNeg[N Number]() NumberValidator[N] {
	return func(v N) (err error) {
		if v < 0 {
			err = fmt.Errorf("Number is negative (is %v)", v)
		}
		return
	}
}

// return a validator to error if v not at least min
func IsAtLeast[N Number](min N) NumberValidator[N] {
	return func(v N) (err error) {
		if v < min {
			err = fmt.Errorf("Number is not at least %v (is %v)", min, v)
		}
		return
	}
}
*/

// use with Section.GetFloat64, Chain.GetFloat64 to validate float
type FloatValidator func(float64) error

// use with Section.GetInt, Chain.GetInt to validate signed int
type IntValidator func(int64) error

// use with Section.GetUInt, Chain.GetUInt to validate unsigned int
type UIntValidator func(uint64) error

// use with Section.GetString, Chain.GetString to validate string
type StringValidator func(string) error

const (
	ErrStringBlank    = uerr.Const("String value empty")
	ErrStringNotBlank = uerr.Const("String value not empty")
)

// return a range validator for GetInt
func FloatRange(min, max float64) FloatValidator {
	if max < min {
		panic(fmt.Sprintf("max (%f) cannot be less than min (%f)", max, min))
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
		panic(fmt.Sprintf("max (%d) cannot be less than min (%d)", max, min))
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
		panic(fmt.Sprintf("max (%d) cannot be less than min (%d)", max, min))
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

// create a UintValidator to verify value is 0 or valid
func UIntZeroOr(validator UIntValidator) UIntValidator {
	return func(v uint64) (err error) {
		if 0 != v {
			err = validator(v)
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

// create a IntValidator to verify value is 0 or valid
func IntZeroOr(validator IntValidator) IntValidator {
	return func(v int64) (err error) {
		if 0 != v {
			err = validator(v)
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

// a StringValidator to verify string length
func StringLen(min, max int) StringValidator {
	if min > max {
		panic("max must be greater than or equalt to min")
	}
	return func(v string) (err error) {
		if min > len(v) {
			err = fmt.Errorf("'%s' is too short.  Must be >= %d", v, min)
		} else if max < len(v) {
			err = fmt.Errorf("'%s' is too long.  Must be <= %d", v, max)
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
	`^(?:[a-z0-9]|[a-z0-9][a-z0-9\-]{0,61}[a-z0-9])(?:\.(?:[a-z0-9]|[a-z0-9][a-z0-9\-]{0,61}[a-z0-9]))*$`)

// check with net.ParseIP first to rule out if it is an IP addr
func ValidHostname(s string) bool {
	return 256 > len(s) && 0 < len(s) && validHostname_.MatchString(s)
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

// return a StringValidator to ensure valid host:port
func ValidHostPort() StringValidator {
	return func(v string) (err error) {
		host, portStr, err := net.SplitHostPort(v)
		if err != nil {
			return
		} else if 0 == len(host) {
			return errors.New("no host specified as part of host:port")
		} else if 0 == len(portStr) {
			return errors.New("no port specified as part of host:port")
		} else if nil == net.ParseIP(host) && !ValidHostname(host) {
			return fmt.Errorf("String (%s) not a valid IP or hostname", host)
		}
		port, err := strconv.Atoi(portStr)
		if nil == err && (0 >= port || 65535 < port) {
			err = errors.New("port " + portStr + " out of range")
		}
		return
	}
}

// return a StringValidator to ensure valid URL with a valid scheme
func ValidUrl(scheme ...string) StringValidator {
	return func(v string) (err error) {
		url, err := nurl.Parse(v)
		if err != nil {
			return
		} else if 0 == len(scheme) {
			return // good
		}
		for _, s := range scheme {
			if s == url.Scheme {
				return // good
			}
		}
		return fmt.Errorf("URL (%s) is not one of %s", v, strings.Join(scheme, ", "))
	}
}

// return a StringValidator to ensure valid http or https URL
func ValidHttpUrl() StringValidator {
	return ValidUrl("http", "https")
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
