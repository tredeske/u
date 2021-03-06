package uio

import "testing"

/*
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
*/

func TestBytesPool(t *testing.T) {
	p := (&BytesPool{}).Construct(0)

	bb := p.Get()
	if 0 == len(bb) {
		t.Fatalf("Did not get byte slice")
	}

	p.Put(bb)

	bb = p.Get()
	if 0 == len(bb) {
		t.Fatalf("Did not get byte slice again")
	}
}
