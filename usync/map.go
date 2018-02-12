package usync

import (
	"sync"
	"sync/atomic"
)

//
// Map for read heavy use.
//
type Map struct {
	lock   sync.Mutex   // synchronizes writes
	theMap atomic.Value // holds a map[string]interface{}
}

//
// get the size of the map
//
func (this *Map) Len() (size int) {
	it := this.theMap.Load()
	if nil != it {
		m := it.(map[string]interface{})
		size = len(m)
	}
	return
}

//
// get the value from the map
//
func (this *Map) Get(key string) (value interface{}) {
	it := this.theMap.Load()
	if nil != it {
		m := it.(map[string]interface{})
		value = m[key]
	}
	return
}

//
// get the value from the map, setting ok to true if value found
//
func (this *Map) GetOk(key string) (value interface{}, ok bool) {
	it := this.theMap.Load()
	if nil != it {
		m := it.(map[string]interface{})
		value, ok = m[key]
	}
	return
}

//
// add the value to the map, replacing any existing value
//
func (this *Map) Add(key string, value interface{}) {

	this.lock.Lock()
	defer this.lock.Unlock()

	var m map[string]interface{}
	it := this.theMap.Load()
	if nil != it {
		old := it.(map[string]interface{})
		m = make(map[string]interface{}, len(old))
		for k, v := range old {
			m[k] = v
		}
	} else {
		m = make(map[string]interface{})
	}
	m[key] = value
	this.theMap.Store(m)
}

//
// remove the value from the map, returning the value if found
//
func (this *Map) Remove(key string) (value interface{}) {

	this.lock.Lock()
	defer this.lock.Unlock()

	it := this.theMap.Load()
	if nil != it {
		old := it.(map[string]interface{})
		value = old[key]
		if nil != value {
			m := make(map[string]interface{}, len(old))
			for k, v := range old {
				if k != key {
					m[k] = v
				}
			}
			this.theMap.Store(m)
		}
	}
	return
}

//
// remove values from the map using the provided func
//
func (this *Map) RemoveUsing(rmf func(key string, value interface{}) bool) {

	this.lock.Lock()
	defer this.lock.Unlock()

	it := this.theMap.Load()
	if nil != it {
		var m map[string]interface{}
		changed := false
		old := it.(map[string]interface{})
		for k, v := range old {
			if rmf(k, v) {
				changed = true
			} else {
				if nil == m {
					m = make(map[string]interface{}, len(old))
				}
				m[k] = v
			}
		}
		if changed {
			this.theMap.Store(m)
		}
	}
	return
}

//
// Iterate through each entry
//
func (this *Map) Each(visit func(key string, value interface{})) {

	it := this.theMap.Load()
	if nil != it {
		m := it.(map[string]interface{})
		for k, v := range m {
			visit(k, v)
		}
	}
	return
}
