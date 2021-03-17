package uinit

import (
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
	"github.com/tredeske/u/ustrings"
)

func InitDebug(config *uconfig.Section) (err error) {
	var enable, disable []string
	err = config.Chain().
		GetStrings("enable", &enable).
		GetStrings("disable", &disable).
		Error
	if err != nil {
		return
	}

	if ustrings.Contains(enable, "all") {
		ulog.DebugEnabled = true
	} else {
		for _, item := range enable {
			ulog.SetDebugEnabledFor(item)
		}
	}
	if ustrings.Contains(disable, "all") {
		ulog.DebugEnabled = false
	} else {
		for _, item := range disable {
			ulog.SetDebugDisabledFor(item)
		}
	}
	return
}
