package uregistry

import (
	"fmt"
	"sync"

	"github.com/tredeske/u/uconfig"
)

//
// A registry is a simple lookup service that allows objects to be registered
// and later looked up.
//
// It is particularly helpful to break up cyclic dependencies.
//
// value := "a string"
// registry.Put( "name", value)
//
// var rv string
// err := registry.Get( "name", &rv )
//

var (
	lock_ sync.RWMutex
	map_  = make(map[string]interface{})
)

//
// does the specified value exist in the registry?
//
func Exists(key string) (exists bool) {
	lock_.RLock()
	_, exists = map_[key]
	lock_.RUnlock()
	return
}

//
// Same as GetValid(), but panic instead of returning an error
//
func MustGet(key string, rv interface{}) (rvAgain interface{}) {
	err := GetValid(key, rv)
	if err != nil {
		panic(err)
	}
	rvAgain = rv
	return
}

//
// Get item matching key, setting rv to item if found
//
// rv must be a pointer to the type of item you're looking for
//
func Get(key string, rv interface{}) (err error) {
	_, err = GetOk(key, rv)
	return
}

//
// Simplest/fastest get.  Its on you to convert received thing.
//
func GetIt(key string) (rv interface{}) {
	lock_.RLock()
	rv = map_[key]
	lock_.RUnlock()
	return
}

//
// Same as Get(), but also returns whether value was found or not
//
func GetOk(key string, rv interface{}) (found bool, err error) {
	var it interface{}
	lock_.RLock()
	it, found = map_[key]
	lock_.RUnlock()
	if found {
		err = uconfig.Assign(key, rv, it)
	}
	return
}

//
// Same as Get(), but fails if not found
//
func GetValid(key string, rv interface{}) (err error) {
	found := false
	found, err = GetOk(key, rv)
	if nil == err && !found {
		err = fmt.Errorf("Did not find '%s' in registry", key)
	}
	return
}

//
// put value into registry, overwriting any existing entry
//
func Put(key string, value interface{}) {
	lock_.Lock()
	map_[key] = value
	lock_.Unlock()
}

//
// put value into registry unless there is already a value there.
//
func PutSingletonOk(key string, value interface{}) (ok bool) {
	lock_.Lock()
	_, found := map_[key]
	if !found {
		map_[key] = value
	}
	lock_.Unlock()
	ok = !found
	return
}

//
// put value into registry unless there is already a value there.
//
func PutSingleton(key string, value interface{}) (err error) {
	ok := PutSingletonOk(key, value)
	if !ok {
		err = fmt.Errorf("Registry already contains '%s'", key)
	}
	return
}

//
// put value into registry unless there is already a value there.
// panic if unable
//
func MustPutSingleton(key string, value interface{}) {
	err := PutSingleton(key, value)
	if err != nil {
		panic(err)
	}
}

//
// remove something from the registry
// if rv specified, set rv to what was there (if anything)
//
func Remove(key string, rv ...interface{}) (err error) {
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
