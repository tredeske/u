package uinit

import (
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
)

// Used upon process initialization to load a global config, then a service
// config that will resolve against the global config.
//
func InitConfig(globalF, configF string) (
	global, config *uconfig.Section, err error) {

	err = uconfig.InitEnv()
	if err != nil {
		return
	}

	if 0 != len(globalF) {
		ulog.Debugf("Loading global config: %s", globalF)
		global, err = uconfig.EmptySection().LoadYaml(globalF)
	} else {
		global, err = uconfig.NewSection(nil)
	}
	if nil != err {
		return
	}

	if 0 != len(configF) {
		ulog.Debugf("Loading service config: %s", configF)
		if config, err = global.LoadYaml(configF); nil != err {
			return
		}
	}

	c := global
	if nil != config {
		c = config
	}
	var dbg *uconfig.Section
	err = c.GetSection("debug", &dbg)
	if nil == err && nil != dbg {
		err = InitDebug(dbg)
	}
	return
}
