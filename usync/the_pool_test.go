package usync

import (
	"bytes"
	"testing"
)

func TestPool(t *testing.T) {

	p := Pool{
		New: func() (rv interface{}) {
			return bytes.NewBuffer([]byte{})
		},
	}

	bb, ok := p.Get().(*bytes.Buffer)
	if !ok {
		t.Fatalf("Unable to get a buffer!")
	}

	bb.WriteString("foo")

	p.Put(bb)

	bb, ok = p.Get().(*bytes.Buffer)
	if !ok {
		t.Fatalf("Unable to get a buffer!")
	}
	if 0 == bb.Len() {
		t.Fatalf("Did not get back recycled!")
	}

	p.Stop()
	bb, ok = p.Get().(*bytes.Buffer)
	if ok {
		t.Fatalf("Should have gotten back nothing!")
	} else if nil != bb {
		t.Fatalf("Should have gotten back nil!")
	}

}
