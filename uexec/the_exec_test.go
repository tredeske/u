package uexec

import (
	"strings"
	"testing"
	"time"
)

func TestShToArray(t *testing.T) {
	c := NewChild("/bin/ls", "-la")
	output, err := c.ShToString()
	if err != nil {
		t.Fatalf("Problem running cmd: %s", err)
	}

	c.Reset()
	lines, err := c.ShToArray(nil, nil, nil)
	if err != nil {
		t.Fatalf("Problem running cmd: %s", err)
	} else if 0 == len(lines) {
		t.Fatalf("No output!")
	}

	joined := strings.Join(lines, "\n") + "\n"
	if joined != output {
		t.Fatalf("Output does not match ('%s' != '%s')", output, joined)
	}
}

func TestContext(t *testing.T) {

	//
	// test deadline exceeeded
	//
	c := NewChild("/bin/sleep", "2")
	c.SetTimeout(200 * time.Millisecond)
	err := c.ShToNull()
	if nil == err {
		t.Fatalf("Should be an error at this point, since job should be cancelled")
	}

	//
	// test not cancelling it
	//
	c = NewChild("/bin/true")
	c.SetTimeout(200 * time.Millisecond)
	err = c.ShToNull()
	if err != nil {
		t.Fatalf("Should not be an error here: %s", err)
	}

	//
	// test cancelling it
	//
	c = NewChild("/bin/sleep", "2")
	c.SetTimeout(4 * time.Second)

	go func() {
		err := c.ShToNull()
		if nil == err {
			t.Fatalf("Should be an error at this point, since job should be cancelled")
		}
	}()

	time.Sleep(50 * time.Millisecond)
	c.Cancel()

}
