package ulog

import (
	"path/filepath"
	"testing"

	"github.com/tredeske/u/uio"
)

func TestLog(t *testing.T) {
	theFile := filepath.Join(t.TempDir(), "test.log")
	err := Init(theFile, 0)
	if err != nil {
		t.Fatalf("Init failed: %s", err)
	} else if !uio.FileExists(theFile) {
		t.Fatalf("Does not exist: %s", theFile)
	}
}
