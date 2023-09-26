// Package golum is used to load components into the process
// based on YAML configuration from uconfig.
//
// A component registers a service prototype with golum:
//
//	golum.AddReloadable("serviceType", prototype)
//
// Later, golum is able to provide the YAML config to create the component
//
//	components:
//	  - name:     instanceName
//	    type:     serviceType
//	    disabled: false
//	    timeout:  2s
//	    hosts:    []
//	    note:     a few words about this
//	    config:
//	      foo:    bar
//	      ...
//
// * disabled: optional flag to disable the component
// * hosts:    optional array to indicate which hosts component is enabled on
// * note:     optional field to describe component
// * timeout:  optional how much time to wait for component to start, or fail
//
// Other components can lookup and rendezvous with it using uregistry:
//
//	var instance *Service
//	err = uregistry.Get("instanceName", &instance)
//
// Test methods are also provided.  Take a look at some of the test cases in
// this package for how they can be used.
//
// In an actual process, this is all managed from main with uboot.
//
// You can see the available components and their settings from the command line.
//
//	program -show all
//	program -show [component]
package golum

import (
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/uregistry"
)

// gets called with err non-nil when problem occurs during reload, and gets
// called again with err nil when problem is resolved
type FailFunc func(name string, err error)

const (
	MIN_TIMEOUT time.Duration = 2 * time.Second
)

type (

	// track a component
	golum_ struct {
		name      string     // name of component
		prototype Reloadable // build based on this
		curr      Reloadable
		old       Reloadable
		hosts     []string
		timeout   time.Duration
		config    *uconfig.Section
		disabled  bool
		needStart bool
		failed    bool
	}

	nr_ struct {
		name string
		r    Reloadable
	}
)

var (
	prototypes_ sync.Map    // by type
	golums_     sync.Map    // *golum_ by comp name
	DryRun      atomic.Bool //
	lock_       sync.Mutex
	ready_      []*golum_
	onFail_     FailFunc = func(name string, err error) {
		if nil != err {
			log.Printf("WARN: G: component %s failed: %s", name, err)
		} else {
			log.Printf("G: component %s ok", name)
		}
	}
)

// register a fail handler in case of fail during a reload
func OnFail(onFail FailFunc) {
	lock_.Lock()
	onFail_ = onFail
	lock_.Unlock()
}

func getProto(typ string) Reloadable {
	it, ok := prototypes_.Load(typ)
	if ok {
		return it.(Reloadable)
	}
	return nil
}

// Load components using the available prototypes
//
// The components are loaded from a config array, where each element must
// have a 'name', 'type', and 'config' value.
//
// The 'type' corresponds to the name of the registered Manager.
//
// The 'config' is provided to the manager to create the component.
//
// The 'name' is the unique name of the component.
func Load(configs *uconfig.Array) (err error) {
	if nil == configs || 0 == configs.Len() {
		return
	}

	//
	// create the golums to manage the reloadables
	//
	present := make(map[string]struct{})
	ready := make([]*golum_, 0, configs.Len())

	err = configs.Each(func(config *uconfig.Section) (err error) {

		g, err := newGolum(config)
		if err != nil {
			return
		}

		if _, exists := present[g.name]; exists {
			err = fmt.Errorf("Duplicate component '%s' not allowed", g.name)
			return
		}
		present[g.name] = struct{}{}

		if _, exists := golums_.Load(g.name); exists {
			err = fmt.Errorf("Duplicate golum instance '%s' not allowed", g.name)
			return
		}

		ready = append(ready, g)
		return
	})
	if err != nil {
		return
	}

	//
	// build the reloadables
	//
	for _, g := range ready {
		err = g.Build()
		if err != nil {
			return
		}
	}

	//
	// only commit changes when all is ok
	//
	if !DryRun.Load() {
		for _, g := range ready {
			g.AfterBuild()
			putGolum(g)
		}
		lock_.Lock()
		ready_ = ready
		lock_.Unlock()
	}
	return
}

// load and start the components
func LoadAndStart(configs *uconfig.Array) (err error) {
	err = Load(configs)
	if err != nil {
		return
	}
	return Start()
}

// start previously loaded components
func Start() (err error) {
	lock_.Lock()
	ready := ready_
	ready_ = nil
	lock_.Unlock()

	for _, g := range ready {
		g.StopOld()
	}
	for _, g := range ready {
		err = g.Start()
		if err != nil {
			return
		}
	}
	log.Printf("G: Start complete")
	return
}

// Unload and stop the specified component
func Unload(name string) (unloaded bool) {
	g, ok := getGolum(name)
	if !ok {
		return false
	}
	delGolum(g)
	g.Stop()
	return true
}

// reload components, starting any new ones, stopping any deleted ones
func Reload(configs *uconfig.Array) (err error) {
	start := make([]*golum_, 0, configs.Len())

	defer func() {
		if err != nil {
			for _, g := range start {
				g.Restore()
			}
		}
		onFail_("config", err)
	}()

	//
	// create new reloadables for changed or new configs
	//
	log.Printf("G: Reload begin")
	present := make(map[string]struct{})
	err = configs.Each(func(config *uconfig.Section) (err error) {
		g, err := newGolum(config)
		if err != nil {
			return
		} else if g.disabled {
			log.Printf("G: Disabled %s", g.name)
			return
		}
		present[g.name] = struct{}{}
		existing, exists := getGolum(g.name)
		if exists {
			if existing.config.DiffersFrom(g.config) ||
				g.disabled != existing.disabled {

				log.Printf("G: Reloading %s", existing.name)
				existing.disabled = g.disabled
				err = existing.Rebuild(g.config)
				if err != nil {
					return
				}
				start = append(start, existing)
			}
		} else {
			err = g.Build()
			if err != nil {
				return
			}
			start = append(start, g)
		}
		return
	})
	if err != nil {
		//
		// abort the reload if any config problems - no harm no foul
		//
		return
	}

	//
	// stop and remove any that are not part of new config
	//
	golums_.Range(
		func(k, v any) (cont bool) {
			name := k.(string)
			if _, exists := present[name]; !exists {
				g := v.(*golum_)
				g.Stop()
				delGolum(g)
			}
			return true
		})

	//
	// out with the old
	//
	for _, g := range start {
		g.AfterBuild()
		g.StopOld()
	}

	//
	// start any new
	//
	for _, g := range start {
		putGolum(g)
		err = g.Start()
		if err != nil {
			//
			// unlike with a start, a reload needs to continue.
			//
			// hopefully, someone will see the problem, fix things, and do
			// a reload
			//
			g.failed = true
			onFail_(g.name, err)
			err = nil
		}
	}
	log.Printf("G: Reload complete")
	return
}

// build a Section suitable for loading a golum based on the provided info
func SectionFromConfig(
	name, gtype string,
	config map[string]any,
) (
	rv *uconfig.Section,
	err error,
) {
	m := map[string]any{
		"name":   name,
		"type":   gtype,
		"config": config,
	}
	return uconfig.NewSection(m)
}

// ensure the named component exists and is running
func ReloadOne(s *uconfig.Section) (err error) {
	g, err := newGolum(s)
	if err != nil {
		return
	}
	existing, exists := getGolum(g.name)
	if exists {
		log.Printf("G: Reloading %s", existing.name)
		err = existing.Rebuild(g.config)
		if err != nil {
			return
		}
		g = existing
	} else {
		err = g.Build()
		if err != nil {
			return
		}
		putGolum(g)
	}
	g.AfterBuild()
	g.StopOld()
	err = g.Start()
	return
}

func getGolum(name string) (g *golum_, found bool) {
	var it any
	it, found = golums_.Load(name)
	if found {
		g = it.(*golum_)
	}
	return
}

func putGolum(g *golum_) { golums_.Store(g.name, g) }
func delGolum(g *golum_) { golums_.Delete(g.name) }

func newGolum(config *uconfig.Section) (g *golum_, err error) {
	typ := ""
	g = &golum_{
		config:  &uconfig.Section{},
		timeout: MIN_TIMEOUT,
	}
	err = config.Chain().
		FailExtraKeys("name", "type", "disabled", "config", "hosts", "note",
			"timeout").
		GetString("name", &g.name, uconfig.StringNotBlank()).
		Then(func() { config.NameContext(g.name) }).
		GetString("type", &typ, uconfig.StringNotBlank()).
		GetBool("disabled", &g.disabled).
		GetStrings("hosts", &g.hosts).
		GetDuration("timeout", &g.timeout).
		GetSection("config", &g.config).
		Error
	if err != nil {
		return
	}

	if !g.disabled {
		//
		// if hosts specified, then disable unless we are on a listed host
		//
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
		g.prototype = getProto(typ)
		if nil == g.prototype {
			err = fmt.Errorf("No such type (%s) for %s", typ, g.name)
			return
		}

		//
		// check the config against the help
		//
		help := &uconfig.Help{}
		g.prototype.Help(g.name, help)
		g.config.WarnUnknown(help)
	}
	return
}

func (g *golum_) Build() (err error) {
	if g.disabled {
		log.Printf("G: Disabled %s", g.name)
		return
	} else if nil != g.old {
		panic(fmt.Sprintf("G: cannot build new %s when old exists!", g.name))
	}
	log.Printf("G: New %s", g.name)
	var r Reloadable
	r, err = g.prototype.Reload(g.name, g.config.Chain())
	if err != nil {
		err = uerr.Chainf(err, "Creating '%s'", g.name)
	} else {
		g.old = g.curr
		g.curr = r
		g.needStart = true
	}
	return
}

func (g *golum_) Rebuild(c *uconfig.Section) (err error) {
	saved := g.config
	defer func() {
		if err != nil {
			g.config = saved
		}
	}()
	g.config = c
	return g.Build()
}

func (g *golum_) AfterBuild() {
	if g.disabled {
		uregistry.Put(g.name, Disabled{})
	} else {
		uregistry.Put(g.name, g.curr)
	}
}

func (g *golum_) StopOld() {
	if nil != g.old {
		g.old.Stop()
		g.old = nil
	}
}

func (g *golum_) Restore() {
	if nil != g.old {
		log.Printf("G: Restored %s", g.name)
		g.curr = g.old
		g.old = nil
	}
}

func (g *golum_) Start() (err error) {
	if nil != g.old {
		panic("Cannot start golum if old one is still around")
	}
	if !g.needStart {
		return
	}
	g.needStart = false
	log.Printf("G: Starting %s", g.name)
	timer := time.NewTimer(g.timeout)
	startC := make(chan error)
	go func() {
		startC <- g.curr.Start()
	}()
	select {
	case err = <-startC:
		timer.Stop()
		if err != nil {
			err = uerr.Chainf(err, "Starting %s", g.name)
		}
	case <-timer.C:
		err = fmt.Errorf("Start of '%s' timed out after %s", g.name, g.timeout)
	}
	if err != nil {
		g.failed = true
		return
	}
	if g.failed {
		g.failed = false
		onFail_(g.name, nil)
	}
	return
}

func (g *golum_) Stop() {
	g.StopOld()
	uregistry.Remove(g.name)
	if g.disabled || nil == g.curr {
		return
	}
	log.Printf("G: Stopping %s", g.name)
	g.curr.Stop()
	g.curr = nil
}
