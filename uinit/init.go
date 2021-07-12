package uinit

import (
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
)

//
// Used upon process initialization to load initial config.
//
func InitConfig(configF string) (config *uconfig.Section, err error) {

	err = uconfig.InitEnv()
	if err != nil {
		return
	}

	ulog.Debugf("Loading config config: '%s'", configF)
	config, err = uconfig.NewSection(configF)
	if nil != err {
		return
	}
	ulog.Printf("Loaded config: '%s'", configF)

	var dbg *uconfig.Section
	err = config.GetSectionIf("debug", &dbg)
	if err != nil {
		return
	} else if nil != dbg {
		err = InitDebug(dbg)
	}
	return
}
