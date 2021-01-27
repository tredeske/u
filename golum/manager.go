package golum

import (
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
)

//
// implement to manage component lifecycle for your components
//
// See also AutoStartable, AutoStoppable, AutoReloadable
//
type Manager interface {
	//
	// command line help (via -show)
	//
	HelpGolum(name string, help *uconfig.Help)

	// create a new component with the specifed name and config
	//
	NewGolum(name string, config *uconfig.Section) (err error)

	// start the named component
	//
	StartGolum(name string) (err error)

	// stop the named component
	//
	StopGolum(name string)

	// reload the named component with the new config
	//
	ReloadGolum(name string, config *uconfig.Section) (err error)
}

//
// placeholder for disabled components
//
type Disabled struct{}

//
// mixin for Managers that do not support start
//
type Unstartable struct{}

func (this *Unstartable) StartGolum(name string) (err error) {
	return nil
}

//
// mixin for Managers that do not support stop
//
type IgnoreStop struct{}

func (this *IgnoreStop) StopGolum(name string) {}

//
// mixin for Managers that do not support stop
//
type Unstoppable struct{}

func (this *Unstoppable) StopGolum(name string) {
	ulog.Warnf("Cannot stop %s", name)
}

//
// mixin for Managers that do not support reload
//
type IgnoreReload struct{}

func (this *IgnoreReload) ReloadGolum(name string, c *uconfig.Section) (err error) {
	return
}

//
// mixin for Managers that do not support reload
//
type Unreloadable struct{}

func (this *Unreloadable) ReloadGolum(name string, c *uconfig.Section) (err error) {
	ulog.Warnf("Cannot reload %s", name)
	return
}

//
// mixin to disable help
//
type Unhelpful struct{}

func (this *Unhelpful) HelpGolum(name string, help *uconfig.Help) {
	help.Init(name, "This component is antisocial and has no help")
}

//
// add a component lifecycle manager for the named type
//
// the name corresponds to the 'type' in the YAML
//
func AddManager(name string, manager Manager) {
	//log.Printf("Adding manager %s", name)
	if _, found := managers_[name]; found {
		panic("Duplicate golum manager installed: " + name)
	}
	managers_[name] = manager
}

//
// add a component lifecycle manager for the named realoadable type
//
// the name corresponds to the 'type' in the YAML
//
// the prototype does not need to be initialized - it just needs to be in
// a state where the Reload func is usable.
//
// example:
//     init() {
//         golum.AddReloadable("name", &ReloadableThing{})
//     }
//
func AddReloadable(name string, prototype Reloadable) {
	AddManager(name,
		&reloadableMgr_{
			Prototype: prototype,
		})
}
