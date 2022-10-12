package ulog

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/tredeske/u/uio"
)

func TestLog(t *testing.T) {
	const (
		MAX   = 1000
		FILES = 2
	)
	tmpD := t.TempDir()
	theFile := filepath.Join(tmpD, "test.log")
	err := Init(theFile, MAX, FILES)
	if err != nil {
		t.Fatalf("Init failed: %s", err)
	} else if !uio.FileExists(theFile) {
		t.Fatalf("Does not exist: %s", theFile)
	}

	fmt.Printf(`
GIVEN max log filesize and max log keep set
 WHEN write enough to cause rotation
 THEN there should only be keep files
`)
	s := "The quick brown fox jumped over the gate and ate the rabbit"
	times := (1 + FILES) * MAX / len(s)
	for i := 0; i < times; i++ {
		Printf(s)
	}
	dirF, err := os.Open(tmpD)
	if err != nil {
		t.Fatalf("Unable to open dir %s: %s", tmpD, err)
	}
	defer dirF.Close()

	files, err := dirF.Readdir(0)
	if err != nil {
		t.Fatalf("Unable to list dir %s: %s", tmpD, err)
	}

	if FILES != len(files) {
		for _, fi := range files {
			fmt.Printf(fi.Name() + "\n")
		}
		t.Fatalf("too many files")
	}
}
