package uerr

import (
	"fmt"
	"strings"
)

//
// Enables chaining errors
//
// You can use directly:
//
//     return &u.Error{Message: fmt.Sprintf(format, ...), Code: code}
// -or-
//     err error
//     return u.Errorcf{code, err, format, ...)}
// -or-
//     *Error err
//     err.Chainf( "some additional %s", "information" )
//     return err
//
// Or, you can make your own types based on it:
//
//     type struct MyError { u.Error }
//
// then:
//
//     (&MyError{}).Fillcf( code, err, format, ... )
//   ...
//
type Error struct {
	Message string
	Code    int
	Cause   error
	Also    error
}

//
// implement error interface
//
func (this Error) Error() string {
	return this.Message
}

/*
func Errorcf(code int, err error, format string, args ...interface{}) *Error {
	this := &Error{Message: err.Error(), Code: code, Cause: err}
	return this.Chainf(format, args...)
}
*/

//
// create a new error based on marker and cause, adding additional info
//
// this is useful if an error occurs (cause) and you want to mark the error
// as being of a certain type (marker)
//
func ChainMarkf(cause, marker error, format string, args ...interface{}) *Error {
	if nil == cause {
		return Chainf(marker, format, args...)
	}
	this := &Error{
		Cause:   cause,
		Also:    marker,
		Message: cause.Error(),
	}
	this.Chainf(marker.Error())
	return this.Chainf(format, args...)
}

//
// create a new error based on cause, adding additional info
//
func Chainf(cause error, format string, args ...interface{}) *Error {
	this := &Error{Cause: cause}
	if nil != cause {
		this.Message = cause.Error()
	}
	return this.Chainf(format, args...)
}

/*
func (this *Error) Fillcf(code int, err error, format string, args ...interface{}) *Error {
	this.Message = err.Error()
	return this.Chainf(format, args...)
}
*/

func (this *Error) SetCode(code int) *Error {
	this.Code = code
	return this
}

func (this *Error) Chainf(format string, args ...interface{}) *Error {
	if 0 != len(format) {
		msg := fmt.Sprintf(format, args...)
		if 0 == len(this.Message) {
			this.Message = msg
		} else {
			this.Message = msg + ", caused by: " + this.Message
		}
	}
	return this
}

//
// is the specified error in the the causal chain of errors?
//
func (this *Error) CausedBy(err error) (rv bool) {
	if nil == err {
		rv = false
	} else if err == this || err == this.Cause || err == this.Also {
		rv = true
	} else if nil != this.Cause {
		if eerror, ok := this.Cause.(*Error); ok {
			rv = eerror.CausedBy(err)
		}
	}
	return
}

//
// is the one error caused by the other?
//
func CausedBy(err, causedBy error) (rv bool) {
	if err == causedBy {
		rv = true
	} else if nil != err && nil != causedBy {
		eerror, isUerrError := err.(*Error)
		if isUerrError {
			rv = eerror.CausedBy(causedBy)
		}
	}
	return
}

//
// walk the error cause chain and run matchF until it returns true or we
// get to the root cause
//
func CauseMatches(err error, matchF func(err error) bool) (rv bool) {
	for {
		rv = matchF(err)
		if rv || nil == err {
			break
		}
		eerror, isUerrError := err.(*Error)
		if !isUerrError {
			break
		}
		if nil != eerror.Also {
			rv = matchF(eerror.Also)
			if rv {
				break
			}
		}
		err = eerror.Cause
		if nil == err {
			break
		}
	}
	return
}

//
//
//
func CauseMatchesString(err error, match string) (rv bool) {
	return CauseMatches(err,
		func(err error) bool {
			return strings.Contains(err.Error(), match)
		})
}
