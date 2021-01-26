package usync

import (
	"testing"
)

func TestAtomicInt(t *testing.T) {
	var i AtomicInt

	i.Set(5)
	if i.Get() != 5 {
		t.Fatalf("Value not set to 5.")
	}
	i.Add(5)
	if i.Get() != 10 {
		t.Fatalf("Value did not add to 10.")
	}

	s := i.String()
	if "10" != s {
		t.Fatalf("value string converted to '%s' instead of '10'", s)
	}
}
