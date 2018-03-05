package golum

import (
	"fmt"
	"log"

	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"
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

//
// implement to manage component lifecycle for your components
//
// See also Helper
//
type Manager interface {
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
// default impl for Managers that do not support start
//
type Unstartable struct{}

func (this *Unstartable) StartGolum(name string) (err error) {
	return nil
}

//
// default impl for managers that store golums in uregistry and which
// implement Startable
//
type AutoStartable struct{}

func (this *AutoStartable) StartGolum(name string) (err error) {
	var g Startable
	err = uregistry.GetValid(name, &g)
	if nil == err {
		err = g.Start()
	}
	return
}

//
// default impl for Managers that do not support stop
//
type IgnoreStop struct{}

func (this *IgnoreStop) StopGolum(name string) {}

//
// default impl for Managers that do not support stop
//
type Unstoppable struct{}

func (this *Unstoppable) StopGolum(name string) {
	ulog.Warnf("Cannot stop %s", name)
}

//
// default impl for managers that store golums in uregistry and which
// implement Stoppable
//
type AutoStoppable struct{}

func (this *AutoStoppable) StopGolum(name string) {
	var g Stoppable
	err := uregistry.Remove(name, &g)
	if nil == err && nil != g {
		g.Stop()
	}
	return
}

//
// default impl for Managers that do not support reload
//
type IgnoreReload struct{}

func (this *IgnoreReload) ReloadGolum(name string, c *uconfig.Section) (err error) {
	return
}

//
// default impl for Managers that do not support reload
//
type Unreloadable struct{}

func (this *Unreloadable) ReloadGolum(name string, c *uconfig.Section) (err error) {
	ulog.Warnf("Cannot reload %s", name)
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

//
// handle to a loaded service
//
type Loaded struct {
	ready []*golum_
}

//
// add a component lifecycle manager for the named type
//
func AddManager(name string, manager Manager) {
	//log.Printf("Adding manager %s", name)
	if _, found := managers_[name]; found {
		panic("Duplicate golum manager installed: " + name)
	}
	managers_[name] = manager
}

// track a component
//
type golum_ struct {
	name     string
	manager  Manager
	disabled bool
	hosts    []string
	config   *uconfig.Section
}

var (
	managers_ map[string]Manager = make(map[string]Manager) // by type
	golums_   map[string]*golum_ = make(map[string]*golum_) // by comp name
)

//
// Load components using the available lifecycle managers
//
// The components are loaded from a config array, where each element must
// have a 'name', 'type', and 'config' value.
//
// The 'type' corresponds to the name of the registered Manager.
//
// The 'config' is provided to the manager to create the component.
//
// The 'name' is the unique namem of the component.
//
func Load(configs *uconfig.Array) (rv *Loaded, err error) {
	if nil == configs || 0 == configs.Len() {
		return
	}
	rv = &Loaded{
		ready: make([]*golum_, 0, configs.Len()),
	}
	comp := 0
	err = configs.Each(func(i int, config *uconfig.Section) (err error) {
		comp = i

		// load component
		//
		g, err := loadGolum(config)
		if err != nil {
			return
		}
		if g.disabled {
			log.Printf("G: Disabled %s", g.name)
			uregistry.Put(g.name, Disabled{})
			return
		}
		if _, exists := golums_[g.name]; exists {
			err = fmt.Errorf("Duplicate instance '%s' not allowed", g.name)
			return
		}
		err = addGolum(g)
		if err != nil {
			return
		}
		rv.ready = append(rv.ready, g)
		return
	})
	if err != nil {
		err = uerr.Chainf(err, "Loading component %d", comp)
	}
	return
}

//
// load and start the components
//
func LoadAndStart(configs *uconfig.Array) (err error) {
	loaded, err := Load(configs)
	if err != nil {
		return
	}
	return loaded.Start()
}

//
// Unload and stop the specified component
//
func Unload(name string) (unloaded bool) {
	g, ok := golums_[name]
	if !ok {
		return false
	}
	stopGolum(g)
	return true
}

//
// start the loaded components
//
func (this *Loaded) Start() (err error) {
	log.Printf("G: Start begin")
	for i, g := range this.ready {
		log.Printf("G: Starting %s", g.name)
		err = g.manager.StartGolum(g.name)
		if err != nil {
			err = uerr.Chainf(err, "Starting %s", g.name)
			return
		}
		this.ready[i] = nil
	}
	log.Printf("G: Start complete")
	return
}

//
// reload components, starting any new ones, stopping any deleted ones
//
func Reload(configs *uconfig.Array) (err error) {
	log.Printf("G: Reload begin")
	start := make([]*golum_, 0, configs.Len())
	present := make(map[string]bool)
	err = configs.Each(func(i int, config *uconfig.Section) (err error) {
		g, err := loadGolum(config)
		if err != nil {
			return
		}
		if g.disabled {
			log.Printf("G: Disabled %s", g.name)
			return
		}
		present[g.name] = true
		if existing, exists := golums_[g.name]; exists {
			if existing.config.DiffersFrom(g.config) {
				log.Printf("G: Reloading %s", existing.name)
				err = existing.manager.ReloadGolum(existing.name, g.config)
				if nil == err {
					existing.config = g.config
				}
			}
		} else {
			err = addGolum(g)
			if err != nil {
				return
			}
			start = append(start, g)
		}
		return
	})
	if err != nil {
		return
	}

	// stop and remove any that are not part of new config
	//
	for _, g := range golums_ {
		if _, exists := present[g.name]; !exists {
			stopGolum(g)
		}
	}

	// start any new
	//
	for _, g := range start {
		log.Printf("G: Starting %s", g.name)
		err = g.manager.StartGolum(g.name)
		if err != nil {
			err = uerr.Chainf(err, "Starting %s", g.name)
			return
		}
	}
	log.Printf("G: Reload complete")
	return
}

func stopGolum(g *golum_) {
	log.Printf("G: Stopping %s", g.name)
	g.manager.StopGolum(g.name)
	delete(golums_, g.name)
}

func loadGolum(config *uconfig.Section) (g *golum_, err error) {
	manager := ""
	g = &golum_{
		config: &uconfig.Section{},
	}
	err = config.Chain().
		WarnExtraKeys("name", "type", "disabled", "config", "hosts", "note").
		GetValidString("name", "", &g.name).
		Then(func() { config.NameContext(g.name) }).
		GetValidString("type", "", &manager).
		GetBool("disabled", &g.disabled).
		GetStrings("hosts", &g.hosts).
		GetSection("config", &g.config).
		Error
	if err != nil {
		return
	}
	if !g.disabled {
		if 0 != len(g.hosts) {
			g.disabled = true
			for _, h := range g.hosts {
				if uconfig.IsThisHost(h) {
					g.disabled = false
					break
				}
			}
			if g.disabled {
				return
			}
		}
		g.manager = managers_[manager]
		if nil == g.manager {
			err = fmt.Errorf("No such manager (%s) for %s", manager, g.name)
			return
		}

		//
		// if it is a helper, then check the config
		//
		h, isHelper := g.manager.(Helper)
		if isHelper {
			help := &uconfig.Help{}
			h.HelpGolum(g.name, help)
			g.config.WarnUnknown(help)
		}
	}
	return
}

func addGolum(g *golum_) (err error) {
	log.Printf("G: New %s", g.name)
	err = g.manager.NewGolum(g.name, g.config)
	if err != nil {
		err = uerr.Chainf(err, "Creating '%s'", g.name)
	} else {
		golums_[g.name] = g
	}
	return
}

//
// -----------------------------------------------------------
//

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
	g := golums_[name]
	if nil == g {
		return fmt.Errorf("No such component: %s", name)
	}
	g.manager.StopGolum(name)
	return
}

//
// for test - reload named component
//
func TestReloadComponent(name string) (err error) {
	g := golums_[name]
	if nil == g {
		return fmt.Errorf("No such component: %s", name)
	}

	err = g.manager.ReloadGolum(name, g.config)
	return
}

//
// for test - put this in a defer() to unload all components at end of test
//
func TestStop() {
	for _, g := range golums_ {
		stopGolum(g)
	}
}
