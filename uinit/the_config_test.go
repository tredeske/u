package uinit

import (
	"os"
	"path"
	"testing"

	"github.com/tredeske/u/uconfig"
)

func TestInitConfig(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	configF := path.Join(cwd, "the_config_test_config.yml")

	config, err := InitConfig(configF)
	if err != nil {
		t.Fatal(err)
	}

	one := ""
	err = config.GetString("needOne", &one)
	if err != nil {
		t.Fatal(err)
	}
	if "oneV" != one {
		config.Log()
		t.Fatalf("one does not match: 'oneVs' != '%s'", one)
	}

	if 0 == len(uconfig.ThisHost) {
		t.Fatalf("uconfig.ThisHost not set")
	} else if 0 == len(uconfig.ThisIp) {
		t.Fatalf("uconfig.ThisIp not set")
	}
}
