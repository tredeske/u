package uboot

import (
	"testing"
)

func TestBoot(t *testing.T) {

	if !Testing {
		t.Fatalf("Testing not set as it should be")
	}

	b := Boot{
		Version: "test version",
		ConfigF: "test.yml",
	}

	err := b.Boot()
	if err != nil {
		t.Fatalf("Unable to run Boot: %s", err)
	}

	err = b.Redirect()
	if err != nil {
		t.Fatalf("Unable to run Boot: %s", err)
	}
}
