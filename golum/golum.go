//
// Package golum is used to load components into the process
// based on YAML configuration from uconfig.
//
// A component registers a service type manager with golum:
//
//     golum.AddReloadable("serviceType", reloadable)
//
// or (old school)
//
//     golum.AddManager("serviceType", manager)
//
// Later, golum is able to provide the YAML config to create the component
//
//     components:
//       - name:     instanceName
//         type:     serviceType
//         config:
//           foo:    bar
//           ...
//
// Other components can lookup and rendezvous with it using uregistry:
//
//     var instance *Service
//     err = uregistry.Get("instanceName", &instance)
//
// Test methods are also provided.  Take a look at some of the test cases in
// this package for how they can be used.
//
// In an actual process, this is all managed from main with uboot.
//
// You can see the available components and their settings from the command line.
//
//     program -show all
//     program -show [component]
//
package golum

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/uregistry"
)

//
// handle to a loaded service
//
type Loaded struct {
	ready []*golum_
}

//
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
	golums_   sync.Map                                      // *golum_ by comp name
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
	err = configs.Each(func(config *uconfig.Section) (err error) {

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
		_, exists := golums_.Load(g.name)
		if exists {
			err = fmt.Errorf("Duplicate golum instance '%s' not allowed", g.name)
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
		rv = nil
	}
	return
}

func getGolum(name string) (g *golum_, found bool) {

	var it interface{}
	it, found = golums_.Load(name)
	if found {
		g = it.(*golum_)
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
	g, ok := getGolum(name)
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

	//
	// start each thing in a goroutine and timeout if things take too long
	//
	// we really should add the context to the StartGolum
	//
	startC := make(chan error)
	timeout := time.Duration(len(this.ready)*50) * time.Millisecond
	if timeout < time.Second {
		timeout = time.Second
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	for i, g := range this.ready {
		log.Printf("G: Starting %s", g.name)
		go func() {
			err := g.manager.StartGolum(g.name)
			startC <- err
		}()
		select {
		case err = <-startC:
		case <-ctx.Done():
			err = fmt.Errorf("Start of '%s' timed out", g.name)
		}
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
	err = configs.Each(func(config *uconfig.Section) (err error) {
		g, err := loadGolum(config)
		if err != nil {
			return
		}
		if g.disabled {
			log.Printf("G: Disabled %s", g.name)
			return
		}
		present[g.name] = true
		existing, exists := getGolum(g.name)
		if exists {
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
	golums_.Range(
		func(itK, itV interface{}) (cont bool) {
			name := itK.(string)
			if _, exists := present[name]; !exists {
				stopGolum(itV.(*golum_))
			}
			return true
		})

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
	golums_.Delete(g.name)
}

//
// build a Section suitable for loading a golum based on the provided info
//
func SectionFromConfig(
	name, gtype string,
	config map[string]interface{},
) (
	rv *uconfig.Section,
	err error,
) {
	m := map[string]interface{}{
		"name":   name,
		"type":   gtype,
		"config": config,
	}
	return uconfig.NewSection(m)
}

//
// ensure the named component exists and is running
//
func ReloadOne(s *uconfig.Section) (err error) {
	g, err := loadGolum(s)
	if err != nil {
		return
	}
	existing, exists := getGolum(g.name)
	if exists {
		log.Printf("G: Reloading %s", existing.name)
		err = existing.manager.ReloadGolum(existing.name, g.config)
		if nil == err {
			existing.config = g.config
		}
	} else {
		err = addGolum(g)
		if err != nil {
			return
		}
		log.Printf("G: Starting %s", g.name)
		err = g.manager.StartGolum(g.name)
		if err != nil {
			err = uerr.Chainf(err, "Starting %s", g.name)
			return
		}
	}
	return
}

func loadGolum(config *uconfig.Section) (g *golum_, err error) {
	manager := ""
	g = &golum_{
		config: &uconfig.Section{},
	}
	err = config.Chain().
		WarnExtraKeys("name", "type", "disabled", "config", "hosts", "note").
		GetString("name", &g.name, uconfig.StringNotBlank()).
		Then(func() { config.NameContext(g.name) }).
		GetString("type", &manager, uconfig.StringNotBlank()).
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
		// check the config against the help
		//
		help := &uconfig.Help{}
		g.manager.HelpGolum(g.name, help)
		g.config.WarnUnknown(help)
	}
	return
}

func addGolum(g *golum_) (err error) {
	log.Printf("G: New %s", g.name)
	err = g.manager.NewGolum(g.name, g.config)
	if err != nil {
		err = uerr.Chainf(err, "Creating '%s'", g.name)
	} else {
		golums_.Store(g.name, g)
	}
	return
}
