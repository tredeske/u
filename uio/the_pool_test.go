package uio

import "testing"

func TestPool(t *testing.T) {

	p := NewBufferPool(0, 0)
	b := p.Get()
	if nil == b {
		t.Fatalf("Did not get buffer")
	} else if 0 == b.Len() {
		t.Fatalf("Buffer had no length")
	} else if !b.IsValid() {
		t.Fatalf("Buffer invalid")
	}

	b.Return()
	if b.IsValid() {
		t.Fatalf("Buffer should be invalid")
	}

	b2 := p.Get()

	if b != b2 {
		t.Fatalf("Did not get recycled")
	}

}
