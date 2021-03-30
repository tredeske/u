package uerr

import (
	"errors"
	"strings"
	"testing"
)

func TestError(t *testing.T) {
	errType := errors.New("Test error")
	errType2 := errors.New("Test error 2")
	var err error

	if !CausedBy(err, err) {
		t.Fatal("nil error should relate to nil")
	} else if CausedBy(err, errType) {
		t.Fatal("nil error should not relate to errType")
	}

	err = errType
	if !CausedBy(err, err) {
		t.Fatal("error should relate to errType")
	} else if !errors.Is(err, errType) {
		t.Fatal("error should relate (errors.Is) to errType")
	}

	err = Chainf(errType, "Chain error to base error")
	if !CausedBy(err, errType) {
		t.Fatal("extended error should relate to errType")
	} else if !errors.Is(err, errType) {
		t.Fatal("error should relate (errors.Is) to errType")
	} else if CausedBy(err, errType2) {
		t.Fatal("extended error should NOT relate to errType2")
	}

	err = Chainf(err, "Chain again")
	if !CausedBy(err, errType) {
		t.Fatal("extended (2) error should relate to errType")
	} else if CausedBy(err, errType2) {
		t.Fatal("extended (2) error should NOT relate to errType2")
	}

	matches := CauseMatches(err,
		func(err error) (matches bool) {
			return strings.Contains(err.Error(), "to base error")
		})
	if !matches {
		t.Fatal("no match!")
	}
}

func TestCode(t *testing.T) {
	const CODE = 2
	cause := errors.New("cause")
	chained := ChainfCode(cause, CODE, "chain")
	again := Chainf(chained, "chain again")

	code, ok := GetCode(again)
	if !ok {
		t.Fatalf("Could not get code")
	} else if CODE != code {
		t.Fatalf("Code not correct")
	}

	code, ok = GetCode(cause)
	if ok {
		t.Fatalf("should be no code")
	} else if 0 != code {
		t.Fatalf("code should be zero")
	}
}

func TestMixin(t *testing.T) {
	type MyError struct {
		UError
	}
	cause := errors.New("a cause")

	var err error
	err = RecastCode(&MyError{}, cause, 5, "errors are fun")

	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is does not match!")
	}

	_ = error(err)

	_, isError := err.(error)
	if !isError {
		t.Fatalf("Not an error!")
	}

	_, isUError := err.(chainable_)
	if !isUError {
		t.Fatalf("Not an UError!")
	}

	_, isCorrectError := err.(*MyError)
	if !isCorrectError {
		t.Fatalf("Not correct kind of error")
	}

	var myError *MyError
	if !errors.As(err, &myError) {
		t.Fatalf("errors.As does not match!")
	}
}

func TestNil(t *testing.T) {
        if !IsNil(nil) {
                t.Fatalf("IsNil fails on naked nil")
        }
        var err error
        if !IsNil(err) {
                t.Fatalf("IsNil fails on var nil")
        }

        f := func(err error) bool {
                return IsNil(err)
        }
        if !f(nil) {
                t.Fatalf("IsNil fails on func nil")
        }

        rf := func(err error) error {
                return err
        }
        if !IsNil(rf(err)) {
                t.Fatalf("IsNil fails on returned nil")
        }
        if !IsNil(rf(nil)) {
                t.Fatalf("IsNil fails on returned nil")
        }

}
