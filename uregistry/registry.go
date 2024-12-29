// Package uregistry is a simple lookup service that allows objects to be
// registered and later looked up.
//
// The golum package will automatically register modules in the default case.
//
// It is particularly helpful to break up cyclic dependencies.
//
//	value := "a string"
//	var thing *Thing = ...
//	registry.Put( "name", value)
//	registry.Put( "thing", thing)
//
//	var rv string
//	var aThing *Thing
//	err := registry.Get( "name", &rv )
//	err := registry.Get( "thing", &aThing )
//	err := registry.GetValid( "thing", &aThing )
//	registry.MustGet( "thing", &aThing )
package uregistry

import (
	"fmt"
	"sync"

	"github.com/tredeske/u/uconfig"
)

var (
	lock_ sync.RWMutex
	map_  = make(map[string]any)
)

// some unit test situations require this between tests.
func TestClearAll() {
	lock_.Lock()
	map_ = make(map[string]any)
	lock_.Unlock()
}

// does the specified value exist in the registry?
func Exists(key string) (exists bool) {
	lock_.RLock()
	_, exists = map_[key]
	lock_.RUnlock()
	return
}

// Same as GetValid(), but panic instead of returning an error
//
// rv is the address of a variable you want to set to the result
//
//	var thing *Thing
//	uregistry.MustGet("thing", &thing)
func MustGet(key string, rv any) {
	err := GetValid(key, rv)
	if err != nil {
		panic(err)
	}
}

// Get item matching key, setting rv to item if found
//
// rv is the address of a variable you want to set to the result
//
//	var thing *Thing
//	err = uregistry.Get("thing", &thing)
func Get(key string, rv any) (err error) {
	_, err = GetOk(key, rv)
	return
}

// Simplest/fastest get.  Its on you to convert received thing.
func GetIt(key string) (rv any) {
	lock_.RLock()
	rv = map_[key]
	lock_.RUnlock()
	return
}

// Same as Get(), but also returns whether value was found or not
//
// rv is the address of a variable you want to set to the result
//
//	var thing *Thing
//	ok, err = uregistry.GetOk("thing", &thing)
func GetOk(key string, rv any) (found bool, err error) {
	var it any
	lock_.RLock()
	it, found = map_[key]
	lock_.RUnlock()
	if found {
		err = uconfig.Assign(key, rv, it)
	}
	return
}

// Same as Get(), but fails if not found
//
// rv is the address of a variable you want to set to the result
//
//	var thing *Thing
//	err = uregistry.GetValid("thing", &thing)
func GetValid(key string, rv any) (err error) {
	found := false
	found, err = GetOk(key, rv)
	if nil == err && !found {
		err = fmt.Errorf("Did not find '%s' in registry", key)
	}
	return
}

// put value into registry, overwriting any existing entry
//
//	var thing *Thing
//	uregistry.Put("thing", thing)
func Put(key string, value any) {
	lock_.Lock()
	map_[key] = value
	lock_.Unlock()
}

// put value into registry unless there is already a value there.
func PutSingletonOk(key string, value any) (ok bool) {
	lock_.Lock()
	_, found := map_[key]
	if !found {
		map_[key] = value
	}
	lock_.Unlock()
	ok = !found
	return
}

// put value into registry unless there is already a value there.
func PutSingleton(key string, value any) (err error) {
	ok := PutSingletonOk(key, value)
	if !ok {
		err = fmt.Errorf("Registry already contains '%s'", key)
	}
	return
}

// put value into registry unless there is already a value there.
// panic if unable
func MustPutSingleton(key string, value any) {
	err := PutSingleton(key, value)
	if err != nil {
		panic(err)
	}
}

// remove something from the registry
// if rv specified, set rv to what was there (if anything)
func Remove(key string, rv ...any) (err error) {
	lock_.Lock()
	it, ok := map_[key]
	if ok {
		delete(map_, key)
	}
	lock_.Unlock()
	if ok && 0 != len(rv) {
		err = uconfig.Assign(key, rv[0], it)
	}
	return
}
