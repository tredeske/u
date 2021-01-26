package golum

import (
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/uregistry"
)

// for use with AutoStartable
type Startable interface {
	Start() (err error)
}

// for use with AutoStoppable
type Stoppable interface {
	Stop()
}

// for use with AutoReloadable
type Reloadable interface {
	Startable
	Stoppable

	//
	// if rv is the same as current managed golum then the registry
	// entry will stay the same and the stop/start will not occur.
	//
	// if rv is different, then the registry will be updated, and
	// Stop/Start will occur.
	//
	Reload(name string, config *uconfig.Section) (rv Reloadable, err error)
}

//
// mixin for Managers to automatically start a Startable
// managed must implement Startable
//
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

//
// mixin for Managers to automatically stop a Stoppable
// managed must implement Stoppable
//
type AutoStoppable struct{}

// implement Manager (partial)
func (this *AutoStoppable) StopGolum(name string) {
	var g Stoppable
	err := uregistry.Remove(name, &g)
	if nil == err && nil != g {
		g.Stop()
	}
}

//
// mixin for Managers to automatically perform standard reload
// managed golum must implement Reloadable
//
type AutoReloadable struct {
	AutoStartable
	AutoStoppable
}

//
// implement Manager (partial)
//
func (this *AutoReloadable) ReloadGolum(
	name string,
	c *uconfig.Section,
) (err error) {

	var g Reloadable
	err = uregistry.GetValid(name, &g)
	if err != nil {
		return
	}
	newG, err := g.Reload(name, c)
	if err != nil || newG == g {
		return
	}

	uregistry.Put(name, newG)
	g.Stop()
	err = newG.Start()
	return
}

//
// helper to implement NewGolum for AutoReloadable
//
// example:
//     func (this *mgr_) NewGolum(name string, c *uconfig.Section) (err error) {
//         return golum.NewReloadable(name, c, &ThingThatIsReloadable{})
//     }
//
func NewReloadable(name string, c *uconfig.Section, g Reloadable) (err error) {

	newG, err := g.Reload(name, c)
	if err != nil {
		return
	}
	uregistry.Put(name, newG)
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
