// Package golum is used to load components into the process
// based on YAML configuration from uconfig.
//
// A component registers a service type manager with golum:
//
//	golum.AddReloadable("serviceType", reloadable)
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

// handle to a loaded service
type Loaded struct {
	ready []*golum_
}

// track a component
type golum_ struct {
	name     string
	manager  *reloadableMgr_
	disabled bool
	hosts    []string
	timeout  time.Duration
	config   *uconfig.Section
}

const (
	MIN_TIMEOUT time.Duration = 2 * time.Second
)

var (
	managers_ sync.Map    // by type
	golums_   sync.Map    // *golum_ by comp name
	creating_ sync.Map    // temp registry after NewGolum, before other calls
	DryRun    atomic.Bool //
)

func creatingPut(name string, it any) { creating_.Store(name, it) }

func creatingCommit() {
	creating_.Range(func(k, v any) (cont bool) {
		uregistry.Put(k.(string), v)
		creating_.Delete(k)
		return true
	})
}

// Load components using the available lifecycle managers
//
// The components are loaded from a config array, where each element must
// have a 'name', 'type', and 'config' value.
//
// The 'type' corresponds to the name of the registered Manager.
//
// The 'config' is provided to the manager to create the component.
//
// The 'name' is the unique name of the component.
func Load(configs *uconfig.Array) (rv *Loaded, err error) {
	if nil == configs || 0 == configs.Len() {
		return
	}
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

	for _, g := range ready {
		if g.disabled {
			log.Printf("G: Disabled %s", g.name)
			uregistry.Put(g.name, Disabled{})
			continue
		}
		err = g.Load()
		if err != nil {
			return
		}
	}

	//
	// only commit changes when all is ok
	//
	if !DryRun.Load() {
		creatingCommit()
		for _, g := range ready {
			if !g.disabled {
				g.Store()
			}
		}
	}
	rv = &Loaded{ready: ready}
	return
}

// load and start the components
func LoadAndStart(configs *uconfig.Array) (err error) {
	loaded, err := Load(configs)
	if err != nil {
		return
	}
	return loaded.Start()
}

// Unload and stop the specified component
func Unload(name string) (unloaded bool) {
	g, ok := getGolum(name)
	if !ok {
		return false
	}
	g.Stop()
	return true
}

// start the loaded components
func (this *Loaded) Start() (err error) {
	log.Printf("G: Start begin")

	for i, g := range this.ready {
		log.Printf("G: Starting %s", g.name)
		err = g.Start()
		if err != nil {
			return
		}
		this.ready[i] = nil
	}

	log.Printf("G: Start complete")
	return
}

// reload components, starting any new ones, stopping any deleted ones
func Reload(configs *uconfig.Array) (err error) {
	log.Printf("G: Reload begin")
	start := make([]*golum_, 0, configs.Len())
	present := make(map[string]struct{})
	err = configs.Each(func(config *uconfig.Section) (err error) {
		g, err := newGolum(config)
		if err != nil {
			return
		}
		if g.disabled {
			log.Printf("G: Disabled %s", g.name)
			return
		}
		present[g.name] = struct{}{}
		existing, exists := getGolum(g.name)
		if exists {
			if existing.config.DiffersFrom(g.config) {
				log.Printf("G: Reloading %s", existing.name)
				err = existing.manager.ReloadGolum(existing.name, g.config)
				if err != nil {
					return
				}
				existing.config = g.config
				start = append(start, existing)
			}
		} else {
			err = g.Load()
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
	golums_.Range(
		func(k, v any) (cont bool) {
			name := k.(string)
			if _, exists := present[name]; !exists {
				(v.(*golum_)).Stop()
			}
			return true
		})

	creatingCommit()

	// start any new
	//
	for _, g := range start {
		log.Printf("G: Starting %s", g.name)
		g.Store()
		err = g.Start()
		if err != nil {
			return
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
		err = existing.manager.ReloadGolum(existing.name, g.config)
		if err != nil {
			return
		}
		existing.config = g.config
		err = existing.Start()
		if err != nil {
			return
		}
	} else {
		err = g.LoadAndStore()
		if err != nil {
			return
		}
		creatingCommit()
		log.Printf("G: Starting %s", g.name)
		err = g.Start()
		if err != nil {
			return
		}
	}
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

func newGolum(config *uconfig.Section) (g *golum_, err error) {
	manager := ""
	g = &golum_{
		config:  &uconfig.Section{},
		timeout: MIN_TIMEOUT,
	}
	err = config.Chain().
		WarnExtraKeys("name", "type", "disabled", "config", "hosts", "note",
			"timeout").
		GetString("name", &g.name, uconfig.StringNotBlank()).
		Then(func() { config.NameContext(g.name) }).
		GetString("type", &manager, uconfig.StringNotBlank()).
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
		it, ok := managers_.Load(manager)
		if !ok {
			err = fmt.Errorf("No such manager (%s) for %s", manager, g.name)
			return
		}
		g.manager = it.(*reloadableMgr_)

		//
		// check the config against the help
		//
		help := &uconfig.Help{}
		g.manager.HelpGolum(g.name, help)
		g.config.WarnUnknown(help)
	}
	return
}

func (g *golum_) LoadAndStore() (err error) {
	err = g.Load()
	if err != nil {
		return
	}
	g.Store()
	return
}

func (g *golum_) Load() (err error) {
	log.Printf("G: New %s", g.name)
	err = g.manager.NewGolum(g.name, g.config)
	if err != nil {
		err = uerr.Chainf(err, "Creating '%s'", g.name)
	}
	return
}

func (g *golum_) Store() {
	golums_.Store(g.name, g)
}

func (g *golum_) Start() (err error) {
	timer := time.NewTimer(g.timeout)
	startC := make(chan error)
	go func() {
		startC <- g.manager.StartGolum(g.name)
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
	return
}

func (g *golum_) Stop() {
	log.Printf("G: Stopping %s", g.name)
	g.manager.StopGolum(g.name)
	golums_.Delete(g.name)
}
