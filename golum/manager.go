package golum

// placeholder for disabled components
type Disabled struct{}

// add a component lifecycle manager for the named realoadable type
//
// typ corresponds to the 'type' field in the YAML
//
// the prototype does not need to be initialized - it just needs to be in
// a state where the Reload func is usable.
//
// example:
//
//	init() {
//	    golum.AddReloadable("name", &ReloadableThing{})
//	}
func AddReloadable(typ string, prototype Reloadable) {
	mgr := &reloadableMgr_{
		Prototype: prototype,
	}
	if _, loaded := managers_.LoadOrStore(typ, mgr); loaded {
		panic("Duplicate golum manager installed: " + typ)
	}
}
