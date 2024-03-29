package uexec

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestShTo(t *testing.T) {

	//
	// ToString
	//
	c := NewChild("/bin/ls", "-la")
	output, err := c.ShToString()
	if err != nil {
		t.Fatalf("Problem running cmd: %s, stderr: %s", err, c.GetStderr())
	} else if 0 == len(output) {
		t.Fatalf("ShToString: Did not get any output!")
	}

	//
	// ToArray
	//
	c.Reset()
	lines, err := c.ShToArray(nil, nil, nil)
	if err != nil {
		t.Fatalf("Problem running cmd: %s, stderr: %s", err, c.GetStderr())
	} else if 0 == len(lines) {
		t.Fatalf("No output!")
	}

	//
	// compare ToString to ToArray
	//
	joined := strings.Join(lines, "\n") + "\n"
	if joined != output {
		t.Fatalf("Output does not match ('%s' != '%s')", output, joined)
	}
}

func TestShToBuff(t *testing.T) {

	var bb bytes.Buffer
	c := NewChild("/bin/ls", "-la")
	err := c.ShToBuff(&bb)
	if err != nil {
		t.Fatalf("Problem running cmd: %s, stderr: %s", err, c.GetStderr())
	} else if 0 == bb.Len() {
		t.Fatalf("Did not get any output!")
	}
	stderr := c.GetStderr()
	if 0 != len(stderr) {
		t.Fatalf("Got some stderr! : '%s'", stderr)
	}

	//
	// combine output
	//
	bb.Reset()
	c.Reset()
	c.CombineOutput()
	err = c.ShToBuff(&bb)
	if err != nil {
		t.Fatalf("Problem running cmd: %s, stderr: %s", err, c.GetStderr())
	} else if 0 == bb.Len() {
		t.Fatalf("Did not get any output!")
	}

	//
	// operate on stderr output instead
	//
	bb.Reset()
	c.Reset()
	c.StderrOnly()
	err = c.ShToBuff(&bb)
	if err != nil {
		t.Fatalf("Problem running cmd: %s, stderr: %s", err, bb.String())
	} else if 0 != bb.Len() {
		t.Fatalf("Should not have gotten any stderr output")
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
	// test cancelling it explicitly
	//
	c = NewChild("/bin/sleep", "2")
	c.SetTimeout(4 * time.Second)

	go func() {
		time.Sleep(50 * time.Millisecond)
		c.Cancel()
	}()

	err = c.ShToNull()
	if nil == err {
		t.Fatalf("Should be an error at this point, since job should be cancelled")
	}
}
