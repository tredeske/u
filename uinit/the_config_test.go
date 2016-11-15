package uinit

import (
	"os"
	"path"
	"testing"
)

func TestInitConfig(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	globalF := path.Join(cwd, "the_config_test_global.yml")
	configF := path.Join(cwd, "the_config_test_config.yml")

	global, config, err := InitConfig(globalF, configF)
	if err != nil {
		t.Fatal(err)
	}

	oneG := ""
	err = global.GetString("needOne", &oneG)
	if err != nil {
		t.Fatal(err)
	}
	one := ""
	err = config.GetString("needOne", &one)
	if err != nil {
		t.Fatal(err)
	}
	if oneG != one {
		global.Log()
		config.Log()
		t.Fatalf("one does not match: '%s' != '%s'", one, oneG)
	}
}
