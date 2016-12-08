package uexec

import (
	"strings"
	"testing"
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
