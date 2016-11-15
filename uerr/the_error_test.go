package uerr

import (
	"errors"
	"testing"
)

func TestError(t *testing.T) {
	errType := errors.New("Test error")
	errType2 := errors.New("Test error 2")
	var err error

	if !ErrorRelatedTo(err, err) {
		t.Fatal("nil error should relate to nil")
	} else if ErrorRelatedTo(err, errType) {
		t.Fatal("nil error should not relate to errType")
	}

	err = errType
	if !ErrorRelatedTo(err, err) {
		t.Fatal("error should relate to errType")
	}

	err = Chainf(errType, "Chain error to base error")
	if !ErrorRelatedTo(err, errType) {
		t.Fatal("extended error should relate to errType")
	} else if ErrorRelatedTo(err, errType2) {
		t.Fatal("extended error should NOT relate to errType2")
	}

	err = Chainf(err, "Chain again")
	if !ErrorRelatedTo(err, errType) {
		t.Fatal("extended (2) error should relate to errType")
	} else if ErrorRelatedTo(err, errType2) {
		t.Fatal("extended (2) error should NOT relate to errType2")
	}
}
