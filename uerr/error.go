//
// Package uerr enables chaining errors and has some error related utilities.
//
// To chain an error:
//
//     var cause error
//     err := uerr.Chainf(cause, "Nasty problem %d", 5)
//
//     - or, to add an error code -
//
//     err := uerr.Chainf(cause, "Nasty problem %d", 5).SetCode(5)
//
// Or, you can make your own types based on it
// (to enable distinguishment with errors.Is / errors.As):
//
//     type MyError struct {
//         uerr.UError
//     }
//     var cause error
//
//     err := uerr.Recast(&MyError{}, cause, "my message about %s", "this")
//
//     - or, to add an error code -
//
//     var code int
//     err := uerr.RecastCode(&MyError{}, cause, code, "my message")
//
//     - or, if not chaining from a cause -
//
//     err := uerr.Cast(&MyError{}, "my message")
//
// You can also use directly:
//
//     var err error
//     err = &uerr.UError{
//         Message: fmt.Sprintf(format, ...),
//         Code: code,
//     }
//     err = &uerr.UError{}.
//         Chainf(cause, format, ...).
//         SetCode(code)
//
package uerr

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

type UError struct {
	Message string
	Code    int
	Cause   error
}

type chainable_ interface {
	Chainf(cause error, format string, args ...interface{}) *UError
	SetCode(code int) *UError
}

type codeAware_ interface {
	GetCode() int
}

//
// get string error message.
//
// implement error interface
//
func (this *UError) Error() string {
	return this.Message
}

//
// get cause of this error.
//
// support errors.Is() and errors.As().
//
// implement errors.Unwrap interface
//
func (this *UError) Unwrap() error {
	return this.Cause
}

//
// create a new error based on cause, adding additional info
//
func Chainf(cause error, format string, args ...interface{}) *UError {
	return (&UError{}).Chainf(cause, format, args...)
}

//
// create a specific type of error
//
// 'as' must be typed as follows:
//
//     type MyError struct {
//         uerr.UError
//     }
//     err := uerr.Novel(&MyError{}, "my message about %s", "this")
//
func Cast(as error, format string, args ...interface{}) error {
	return RecastCode(as, nil, 0, format, args...)
}

//
//
// create a specific type of error from an existing cause
//
// 'as' must be typed as follows:
//
//     type MyError struct {
//         uerr.UError
//     }
//     err := uerr.Novel(&MyError{}, cause, "my message about %s", "this")
//
func Recast(as, cause error, format string, args ...interface{}) error {
	return RecastCode(as, cause, 0, format, args...)
}

//
// create a specific type of error from an existing cause, with error code
//
// 'as' must be typed as follows:
//
//     type MyError struct {
//         uerr.UError
//     }
//     err := uerr.NovelCode(&MyError{}, cause, 5, "my message about %s", "this")
//
func RecastCode(
	as, cause error,
	code int,
	format string, args ...interface{},
) error {
	chainable, ok := as.(chainable_)
	if !ok {
		return Chainf(cause, "UNCHAINABLE ERROR: "+format, args...)
	}
	chainable.Chainf(cause, format, args...).SetCode(code)
	return as
}

//
// is the error really nil?
//
func IsNil(err error) bool {
	return err == nil || reflect.ValueOf(err).IsNil()
}

func (this *UError) SetCode(code int) *UError {
	this.Code = code
	return this
}

// impl chainAware_
func (this *UError) GetCode() (code int) {
	return this.Code
}

//
// If a non-zero code is set in the error chain, return it
//
func GetCode(err error) (code int, found bool) {
	for {
		codeAware, ok := err.(codeAware_)
		if ok {
			code = codeAware.GetCode()
			if 0 != code {
				found = true
				return
			}
		}
		err = errors.Unwrap(err)
		if nil == err {
			return
		}
	}
}

//
// set the cause (if non-nil) and message of this error
//
func (this *UError) Chainf(
	cause error,
	format string, args ...interface{},
) *UError {

	if IsNil(cause) {
		cause = nil
	}
	this.Cause = cause

	var causeMsg string
	if nil != cause {
		causeMsg = cause.Error()
		if 0 == len(causeMsg) {
			causeMsg = fmt.Sprintf("%T", cause)
		}
	}

	if 0 != len(format) {
		msg := fmt.Sprintf(format, args...)
		if nil == cause {
			this.Message = msg
		} else {
			this.Message = msg + ", caused by: " + causeMsg
		}
	} else if nil != cause {
		this.Message = causeMsg
	}
	return this
}

//
// is causedBy in the the causal chain of errors?
//
// this method predates errors.Is - prefer errors.Is
//
func (this *UError) CausedBy(causedBy error) (rv bool) {
	if IsNil(causedBy) {
		rv = false
	} else if causedBy == this || causedBy == this.Cause {
		rv = true
	} else if nil != this.Cause {
		rv = errors.Is(this.Cause, causedBy)
	}
	return
}

//
// deprecated - use errors.Is or errors.As
//
// is the one error caused by the other?
//
func CausedBy(err, causedBy error) bool {
	return errors.Is(err, causedBy)
}

//
// Does any error in the chain match criteria?
//
func CauseMatches(err error, criteria func(err error) bool) bool {
	for {
		if criteria(err) {
			return true
		}
		err = errors.Unwrap(err)
		if nil == err {
			return false
		}
	}
}

//
// Does any error in the chain have an Error string containing match?
//
func CauseMatchesString(err error, match string) bool {
	return CauseMatches(err,
		func(err error) bool {
			return strings.Contains(err.Error(), match)
		})
}
