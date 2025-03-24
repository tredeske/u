package golum

import (
	"fmt"
	"log"

	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/uinit"
	"github.com/tredeske/u/ulog"
	"github.com/tredeske/u/uregistry"
)

// for test - load specified components
func TestLoadAndStart(config any) (err error) {

	log.Println("G: test load and start")

	s, err := uconfig.NewSection(config)
	if err != nil {
		return
	}

	var dbg *uconfig.Section
	err = s.GetSectionIf("debug", &dbg)
	if err != nil {
		return
	} else if nil != dbg {
		err = uinit.InitDebug(dbg)
		if err != nil {
			return
		}
	}

	var comps *uconfig.Array
	err = s.GetArray("components", &comps)
	if err != nil {
		return
	}
	err = LoadAndStart(comps)
	return
}

// for test - reload components based on new config
func TestReload(config any) (err error) {

	log.Println("G: test reload")

	s, err := uconfig.NewSection(config)
	if err != nil {
		return
	}
	var comps *uconfig.Array
	err = s.GetArray("components", &comps)
	if err != nil {
		return
	}
	err = Reload(comps)
	return
}

// for test - stop named component but keep the golum around so it can be restarted
func TestStopComponent(name string) (err error) {

	log.Println("G: test stop ", name)

	g, found := getGolum(name)
	if !found {
		return fmt.Errorf("No such component: %s", name)
	}
	g.Stop()
	return
}

// for test - reload named component
func TestReloadComponent(name string) (err error) {

	log.Println("G: test reload ", name)

	g, found := getGolum(name)
	if !found {
		return fmt.Errorf("No such component: %s", name)
	}

	g.Stop()

	err = g.Build()
	if err != nil {
		return
	}
	g.AfterBuild()
	g.Start()
	return
}

// for test - put this in a defer() to unload all components at end of test
func TestStop() {
	ulog.Debugf("G: TestStop")
	prototypes_.Clear()
	golums_.Range(
		func(k, v any) (cont bool) {
			ulog.Debugf("G: stopping %s", k)
			g := v.(*golum_)
			g.Stop()
			delGolum(g)
			return true
		})
	golums_.Clear()
	uregistry.TestClearAll()
}
