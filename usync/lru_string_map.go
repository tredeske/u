package usync

import (
	"math"
	"sync"
	"sync/atomic"
	"time"
)

//
// Least Recently Used (LRU) eviction map
//
type LruStringMap struct {
	m        sync.Map
	length   int64
	Capacity int
	running  AtomicBool
}

type lruVal_ struct {
	value string
	used  int64
}

//
// get the value from the map, setting ok to true if value found
//
func (this *LruStringMap) Get(key string) (rv string, ok bool) {
	var it interface{}
	it, ok = this.m.Load(key)
	if ok {
		lruv := it.(*lruVal_)
		lruv.used++
		rv = lruv.value
	}
	return
}

//
// get the value from the map, or add it if not found
//
func (this *LruStringMap) GetOrAdd(key, value string) (rv string) {
	var ok bool
	rv, ok = this.Get(key)
	if !ok {
		rv = this.add(key, value)
	}
	return
}

//
// get the value from the map, or add it if not found
//
func (this *LruStringMap) GetOrAddF(
	key string,
	add func() (key, value string),
) (
	rv string,
) {
	var ok bool
	rv, ok = this.Get(key)
	if !ok {
		k, v := add()
		rv = this.add(k, v)
	}
	return
}

func (this *LruStringMap) add(key, value string) (rv string) {

	if this.Capacity <= int(atomic.LoadInt64(&this.length)) &&
		this.running.SetUnlessSet() {
		go this.evict()
	}

	it, loaded := this.m.LoadOrStore(key, &lruVal_{value: value})
	if !loaded { // stored
		atomic.AddInt64(&this.length, 1)
		rv = value
	} else {
		rv = (it.(*lruVal_)).value
	}
	return
}

func (this *LruStringMap) evict() {
	var any bool
	for this.running.IsSet() {
		length := int(atomic.LoadInt64(&this.length))
		for this.Capacity < length {
			var least int64 = math.MaxInt64
			var evictIt interface{}
			this.m.Range(
				func(kIt, vIt interface{}) bool {
					v := vIt.(*lruVal_)
					if v.used < least {
						evictIt = kIt
					}
					return true
				})
			any = true
			this.m.Delete(evictIt)
			length = int(atomic.AddInt64(&this.length, -1))
		}
		if !any {
			this.running.Clear()
			break
		}
		time.Sleep(100 * time.Millisecond)
		any = false
	}
}

//
// get the size of the map
//
func (this *LruStringMap) Len() (size int) {
	return int(atomic.LoadInt64(&this.length))
}

/*

This impl works better single threaded, but not so great multi threaded

//
// Least Recently Used (LRU) eviction map
//
// Sync'd with a mutex, so better when not too many threads
//
type LruStringLockMap struct {
	lock     sync.Mutex
	m        map[string]*lruVal_
	Capacity int
}

//
// get the value from the map, setting ok to true if value found
//
func (this *LruStringLockMap) Get(key string) (rv string, ok bool) {
	this.lock.Lock()
	rv, ok = this.get(key)
	this.lock.Unlock()
	return
}

func (this *LruStringLockMap) get(key string) (rv string, ok bool) {
	var lruv *lruVal_
	lruv, ok = this.m[key]
	if ok {
		lruv.used++
		rv = lruv.value
	}
	return
}

//
// get the value from the map, or add it if not found
//
func (this *LruStringLockMap) GetOrAdd(key, value string) (rv string) {
	var ok bool
	this.lock.Lock()
	rv, ok = this.get(key)
	if !ok {
		rv = this.add(key, value)
	}
	this.lock.Unlock()
	return
}

//
// get the value from the map, or add it if not found
//
func (this *LruStringLockMap) GetOrAddF(
	key string,
	add func() (key, value string),
) (
	rv string,
) {
	var ok bool
	this.lock.Lock()
	rv, ok = this.get(key)
	if !ok {
		k, v := add()
		rv = this.add(k, v)
	}
	this.lock.Unlock()
	return
}

func (this *LruStringLockMap) add(key, value string) (rv string) {

	if nil == this.m {
		this.m = make(map[string]*lruVal_, this.Capacity)
	}

	lruv, ok := this.m[key]
	if ok {
		lruv.used++
		rv = lruv.value

	} else {
		if this.Capacity <= len(this.m) {
			evict(this.m)
		}
		this.m[key] = &lruVal_{
			value: value,
		}
		rv = value
	}
	return
}

func evict(m map[string]*lruVal_) {
	var least int64 = math.MaxInt64
	var evictK string
	for k, v := range m {
		if v.used < least {
			least = v.used
			evictK = k
		}
	}
	delete(m, evictK)
}

//
// get the size of the map
//
func (this *LruStringLockMap) Len() (size int) {
	this.lock.Lock()
	size = len(this.m)
	this.lock.Unlock()
	return
}
*/
