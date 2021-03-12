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

func TestMixin(t *testing.T) {
	type MyError struct {
		UError
	}
	cause := errors.New("a cause")

	var err error
	err = NovelCode(&MyError{}, cause, 5, "errors are fun")

	if !errors.Is(err, cause) {
		t.Fatalf("errors.Is does not match!")
	}

	_ = error(err)

	_, isError := err.(error)
	if !isError {
		t.Fatalf("Not an error!")
	}

	_, isUError := err.(chainable)
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
