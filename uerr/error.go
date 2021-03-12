//
// Package uerr enables chaining errors and has some error related utilities.
//
// To chain an error:
//
//     var cause error
//     err := uerr.Chainf(cause, "Nasty problem %d", 5)
//
//     -or-
//
//     err := uerr.Chainf(cause, "Nasty problem %d", 5).SetCode(5)
//
// Or, you can make your own types based on it:
//
//     type MyError struct {
//         uerr.UError
//     }
//     var cause error
//
//     err := uerr.Novel(&MyError{}, cause, "my message about %s", "this")
//
//     -or-
//
//     var code int
//     err := uerr.NovelCode(&MyError{}, cause, code, "my message about %s", "this")
//
// You can also use directly:
//
//     var err error
//     err = &uerr.UError{Message: fmt.Sprintf(format, ...), Code: code}
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

type chainable interface {
	Chainf(cause error, format string, args ...interface{}) *UError
	SetCode(code int) *UError
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
// create a specific type of error from an existing cause
//
//     type MyError struct {
//         uerr.UError
//     }
//     err := uerr.Novel(&MyError{}, cause, "my message about %s", "this")
//
func Novel(novel, cause error, format string, args ...interface{}) error {
	return NovelCode(novel, cause, 0, format, args...)
}

//
// create a specific type of error from an existing cause, with error code
//
func NovelCode(novel, cause error, code int, format string, args ...interface{}) error {
	chainable, ok := novel.(chainable)
	if !ok {
		return Chainf(cause, "UNCHAINABLE ERROR: "+format, args...)
	}
	chainable.Chainf(cause, format, args...).SetCode(code)
	return novel
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

func (this *UError) Chainf(cause error, format string, args ...interface{}) *UError {
	if IsNil(cause) {
		cause = nil
	}
	this.Cause = cause
	if 0 != len(format) {
		msg := fmt.Sprintf(format, args...)
		if nil == cause {
			this.Message = msg
		} else {
			this.Message = msg + ", caused by: " + cause.Error()
		}
	} else if nil != cause {
		this.Message = cause.Error()
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
// walk the error cause chain and run matchF until it returns true or we
// get to the root cause
//
func CauseMatches(err error, matchF func(err error) bool) bool {
	for {
		if matchF(err) {
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
func CauseMatchesString(err error, match string) (rv bool) {
	return CauseMatches(err,
		func(err error) bool {
			return strings.Contains(err.Error(), match)
		})
}
