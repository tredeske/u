package uboot

import (
	"log"
	"os"
	"time"

	"github.com/tredeske/u/golum"
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/uexec"
	"github.com/tredeske/u/uinit"
	"github.com/tredeske/u/uio"
	"github.com/tredeske/u/ulog"
)

// manage auto reloading of components
//
type reloader_ struct {
	components string        //
	interval   time.Duration //
}

func (this *reloader_) Start() {
	uexec.MakeGo("config reloader", func() (err error) {
		log.Printf("Watching %s for changes", ConfigF)
		uio.FileWatch(ConfigF, this.interval, this.reload)
		return
	})
}

func (this *reloader_) reload(f string, fi os.FileInfo, err error) {
	log.Printf("G: Investigating change in config file '%s'", f)
	if err != nil {
		ulog.Errorf("G: Problem checking config file '%s': %s", f, err)
		return
	}

	_, config, err := uinit.InitConfig(GlobalF, ConfigF)
	if err != nil {
		ulog.Errorf("Unable to parse %s: %s", ConfigF, err)
		return
	}

	config.AddSub("logDir", LogD)
	config.AddSub("name", Name)
	var gconfig *uconfig.Array
	err = config.GetValidArray(this.components, &gconfig)
	if err != nil {
		ulog.Errorf("Getting '%s' from %s: %s", this.components, ConfigF, err)
		return
	}
	err = golum.Reload(gconfig)
	if err != nil {
		ulog.Errorf("Unable to load components: %s", err)
	}
	return
}
