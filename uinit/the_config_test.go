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
}
