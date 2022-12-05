package golum

import (
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/uregistry"
)

type (

	// for use with AutoStartable/AutoReloadable - managed must implement
	Startable interface {
		Start() (err error)
	}

	// for use with AutoStoppable/AutoReloadable - managed must implement
	Stoppable interface {
		Stop()
	}

	// for use with AutoReloadable - managed must implement
	Reloadable interface {
		Startable
		Stoppable

		//
		// The reload sequence is:
		// * golum call Reload on all changed components and new components
		// * golum calls Start on all of the above
		//
		// It is strongly recommended that implementers load a config struct
		// with the new values, and then apply that when Start is invoked.  This
		// will prevent problems when a config change is made, and some other
		// component has a bad config.
		//
		Reload(name string, config *uconfig.Section) (rv Reloadable, err error)

		//
		// provide command line help (via -show)
		//
		Help(name string, help *uconfig.Help)
	}
)

// mixin to disable help
type UnhelpfulReloadable struct{}

func (this *UnhelpfulReloadable) Help(name string, help *uconfig.Help) {
	help.Init(name, "This component is antisocial and has no help")
}

// mixin for Managers to automatically start a Startable
// managed must implement Startable
type AutoStartable struct{}

// implement Manager (partial)
func (this *AutoStartable) StartGolum(name string) (err error) {
	var g Startable
	err = uregistry.GetValid(name, &g)
	if nil == err {
		err = g.Start()
	}
	return
}

// mixin for Managers to automatically stop a Stoppable
// managed must implement Stoppable
type AutoStoppable struct{}

// implement Manager (partial)
func (this *AutoStoppable) StopGolum(name string) {
	var g Stoppable
	err := uregistry.Remove(name, &g)
	if nil == err && nil != g {
		g.Stop()
	}
}

// generic manager for Reloadable components
//
// see AddReloadable
type reloadableMgr_ struct {
	AutoStartable
	AutoStoppable
	Prototype Reloadable
}

// implement Manager
func (this *reloadableMgr_) HelpGolum(name string, help *uconfig.Help) {
	this.Prototype.Help(name, help)
}

// implement Manager
func (this *reloadableMgr_) NewGolum(name string, c *uconfig.Section) (err error) {
	g, err := this.Prototype.Reload(name, c)
	if err != nil {
		return
	}
	creatingPut(name, g)
	return
}

// implement Manager
func (this *reloadableMgr_) ReloadGolum(
	name string,
	c *uconfig.Section,
) (err error) {

	var g Reloadable
	err = uregistry.GetValid(name, &g)
	if err != nil {
		return
	}
	newG, err := g.Reload(name, c)
	if err != nil { //|| newG == g {
		return
	}

	uregistry.Put(name, newG)
	//g.Stop()
	err = newG.Start()
	return
}

/* does not appear possible

//
// default impl for managers that store golums in uregistry.
// must also be AutoStoppable
//
type AutoReloadable struct{}

func (this *AutoReloadable) ReloadGolum(name string, c *uconfig.Section) (err error) {
	var existing Stoppable
	err = uregistry.GetValid(name, &existing)
	if err != nil {
		return
	}
	rVal := reflect.ValueOf(this)
	newF := rVal.MethodByName("NewGolum")
	if newF.IsValid() {
		err = errors.New("Does not implement NewGolum")
		return
	}
	startF := rVal.MethodByName("StartGolum")
	if startF.IsValid() {
		err = errors.New("Does not implement StartGolum")
		return
	}
	nameV := reflect.ValueOf(name)
	cV := reflect.ValueOf(c)
	errV := reflect.ValueOf(err)

	retVals := newF.Call([]reflect.Value{nameV, cV})
	errV.Set(retVals[0])
	if err != nil {
		return
	}
	existing.Stop()

	retVals = startF.Call([]reflect.Value{nameV})
	errV.Set(retVals[0])
	return
}
*/
