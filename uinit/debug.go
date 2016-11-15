package uinit

import (
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
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
	for _, item := range enable {
		if "all" == item {
			ulog.DebugEnabled = true
		} else {
			ulog.DebugEnabledFor[item] = true
		}
	}
	for _, item := range disable {
		if "all" == item {
			ulog.DebugEnabled = false
		} else {
			ulog.DebugDisabledFor[item] = true
		}
	}
	return
}
