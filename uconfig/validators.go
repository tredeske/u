package uconfig

import (
	"errors"
	"fmt"
)

// use with Section.GetInt to validate signed int
type IntValidator func(int64) error

// use with Section.GetUInt to validate unsigned int
type UIntValidator func(uint64) error

// use with Section.GetString to validate string
type StringValidator func(string) error

var (
	ErrStringBlank    = errors.New("String value empty")
	ErrStringNotBlank = errors.New("String value not empty")
)

// return a range validator for GetInt
func IntRange(min, max int64) IntValidator {
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
			err = fmt.Errorf("int is not positive (is %d)", v)
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
