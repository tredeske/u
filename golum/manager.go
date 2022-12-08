package golum

import "github.com/tredeske/u/uconfig"

// Add a prototype to be used as the basis for the the named kind of components.
//
// kind corresponds to the 'type' field in the YAML
//
// the prototype does not need to be initialized - it just needs to be in
// a state where the Reload func is usable.
//
// example:
//
//	init() {
//	    golum.AddReloadable("name", &ReloadableThing{})
//	}
func AddReloadable(kind string, prototype Reloadable) {
	if _, loaded := prototypes_.LoadOrStore(kind, prototype); loaded {
		panic("Duplicate golum prototype installed for type: " + kind)
	}
}

// A reloadable component follows the prototype pattern
type Reloadable interface {
	//
	// Start is called after Reload
	//
	// Your Start method will need to know if this is an initial start or
	// if this is a later start due to a config change.  Think of Start as
	// saying, "apply the config created from Reload".
	//
	Start() (err error)

	//
	// Stop is only called if the component is no longer needed.
	//
	// That is, when Reload returns a different Reloadable, the previous
	// Reloadable is stopped before the new one is started.  Stop is also
	// called when the config changes to not have the Reloadable anymore.
	//
	Stop()

	//
	// Refresh existing Reloadable or create a new Reloadable using this as a
	// prototype.
	//
	// If the returned Reloadable is different, then the new reloadable obsoletes
	// the original.
	//
	// The reload sequence is:
	// * golum calls Reload on all changed components and new components
	// * golum calls Stop on all obsoleted components
	// * golum calls Start on all changed and new components
	//
	// Your Reload method should only be about loading the config, and should
	// not be changing any other state or starting anything.
	//
	// It is strongly recommended that implementers load a config struct
	// with the new values, and then apply that when Start is invoked.  This
	// will prevent problems when a config change is made, and some other
	// component has a bad config.
	//
	// If Reload returns a value (rv) different than itself, then the returned
	// value obsoletes the initial Reloadable, and the initial Reloadable will
	// be Stop()ed.
	//
	Reload(name string, config *uconfig.Chain) (rv Reloadable, err error)

	//
	// provide command line help (via -show)
	//
	Help(name string, help *uconfig.Help)
}

// mixin to disable help
type UnhelpfulReloadable struct{}

func (this *UnhelpfulReloadable) Help(name string, help *uconfig.Help) {
	help.Init(name, "This component is antisocial and has no help")
}

// placeholder for disabled components
type Disabled struct{}
