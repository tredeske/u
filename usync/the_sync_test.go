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

	result, added := i.AddIfLessThan(5, 16)
	if !added {
		t.Fatalf("add did not occur!")
	} else if result != 15 {
		t.Fatalf("Result did not equal 15")
	}
	if i.Get() != 15 {
		t.Fatalf("Value did not add to 15")
	}

	result, added = i.AddIfLessThan(5, 16)
	if added {
		t.Fatalf("add should not occur!")
	} else if result != 0 {
		t.Fatalf("leaked result")
	}
	if i.Get() != 15 {
		t.Fatalf("Value did not remain at 15")
	}
}
