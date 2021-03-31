package u

import (
	"errors"
	"testing"

	"github.com/tredeske/u/uerr"
)

func TestUerr(t *testing.T) {

	var err error

	type MyError struct {
		uerr.UError
	}

	err = uerr.Cast(&MyError{}, "my error")
	if nil == err {
		t.Fatalf("should have an error here")
	}

	var myError *MyError
	if !errors.As(err, &myError) {
		t.Fatalf("errors.As does not match!")
	}
}
