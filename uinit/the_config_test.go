package uinit

import (
	"os"
	"path"
	"testing"

	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
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

	if ulog.DebugEnabled {
		t.Fatalf("debug should not be enabled for all")
	} else if !ulog.IsDebugEnabledFor("foo") {
		t.Fatalf("debug *should* be enabled for foo")
	} else if ulog.IsDebugEnabledFor("bar") {
		t.Fatalf("debug should *not* be enabled for bar")
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
