package uio

import (
	"os"
	"testing"
)

func TestFileCreateExistsRemove(t *testing.T) {

	file := "./FileCreateTest.txt"
	err := FileCreate(file,

		func(f *os.File) (err error) {
			_, err = f.Write([]byte("just a test"))
			return
		})

	if err != nil {
		t.Fatalf("Unable to create %s: %s", file, err)
	}

	if !FileExists(file) {
		t.Fatalf("File %s does not exist!")
	}

	err = FileRemoveAll(file)
	if err != nil {
		t.Fatalf("Unable to rm %s: %s", file, err)
	}

	if FileExists(file) {
		t.Fatalf("File %s still exists!")
	}
}
