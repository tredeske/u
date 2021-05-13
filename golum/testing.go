package golum

import (
	"fmt"

	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
	"github.com/tredeske/u/uregistry"
)

//
// for test - load specified components
//
func TestLoadAndStart(config interface{}) (err error) {

	s, err := uconfig.NewSection(config)
	if err != nil {
		return
	}
	var comps *uconfig.Array
	err = s.GetArray("components", &comps)
	if err != nil {
		return
	}
	err = LoadAndStart(comps)
	return
}

//
// for test - reload components based on new config
//
func TestReload(config interface{}) (err error) {

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

//
// for test - stop named component
//
func TestStopComponent(name string) (err error) {
	g, found := getGolum(name)
	if !found {
		return fmt.Errorf("No such component: %s", name)
	}
	g.manager.StopGolum(name)
	return
}

//
// for test - reload named component
//
func TestReloadComponent(name string) (err error) {
	g, found := getGolum(name)
	if !found {
		return fmt.Errorf("No such component: %s", name)
	}

	err = g.manager.ReloadGolum(name, g.config)
	return
}

//
// for test - put this in a defer() to unload all components at end of test
//
func TestStop() {
	ulog.Debugf("G: TestStop")
	golums_.Range(
		func(itK, itV interface{}) (cont bool) {
			ulog.Debugf("G: stopping %s", itK)
			stopGolum(itV.(*golum_))
			return true
		})
	uregistry.TestClearAll()
	//managers_ = make(map[string]Manager)
}
