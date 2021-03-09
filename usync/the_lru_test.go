package usync

import (
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
)

func TestLruMap(t *testing.T) {
	const CAP = 5
	const KEY = "foo"
	const VAL = "bar"
	m := LruMap{Capacity: CAP}

	if 0 != m.Len() {
		t.Fatalf("should be empty")
	}

	result, found := m.GetString(KEY)
	if found {
		t.Fatalf("should not have found %s", KEY)
	} else if 0 != len(result) {
		t.Fatalf("should be 0 length")
	}

	result = m.GetOrAddString(KEY, VAL)
	if VAL != result {
		t.Fatalf("got back %s instead", result)
	}
	result, found = m.GetString(KEY)
	if !found {
		t.Fatalf("should have found %s", KEY)
	} else if 0 == len(result) {
		t.Fatalf("should not have zero length")
	} else if 1 != m.Len() {
		t.Fatalf("len should be 1")
	}

	invoked := false
	result = m.GetOrAddF(KEY, func() (k string, v interface{}) {
		invoked = true
		return KEY, VAL
	}).(string)
	if invoked {
		t.Fatalf("Should already be set!")
	} else if result != VAL {
		t.Fatalf("got back bad value: %s", result)
	}

}

// strictly for comparision
func BenchmarkLruLockedMapGetOrAdd(b *testing.B) {
	keys := getInts()
	var lock sync.Mutex
	m := make(map[string]string)
	for i := 0; i < b.N; i++ {
		key := keys[i%MAX_INT]
		lock.Lock()
		_, found := m[key]
		if !found {
			m[key] = key
		}
		lock.Unlock()
	}
}

// strictly for comparision
func BenchmarkLruSyncMapGetOrAdd(b *testing.B) {
	keys := getInts()
	m := sync.Map{}
	for i := 0; i < b.N; i++ {
		key := keys[i%MAX_INT]
		_, found := m.Load(key)
		if !found {
			m.LoadOrStore(key, key)
		}
	}
}

// strictly for comparision
func BenchmarkSingleGetOrAdd(b *testing.B) {
	m := LruMap{Capacity: 8192}
	for i := 0; i < b.N; i++ {
		m.GetOrAdd("foo", "foo")
	}
}

const MAX_INT = 8192

var the_ints []string

func getInts() []string {
	if 0 == len(the_ints) {
		the_ints = make([]string, MAX_INT)
		for i := 0; i < MAX_INT; i++ {
			the_ints[i] = strconv.Itoa(i)
		}
	}
	return the_ints
}

func BenchmarkLruGetOrAdd(b *testing.B) {
	keys := getInts()

	m := LruMap{Capacity: 2 * MAX_INT}

	unique := 0

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%MAX_INT]
		m.GetOrAddF(key,
			func() (k string, v interface{}) {
				unique++
				return key, key
			})
	}
	if unique > MAX_INT {
		b.Fatalf("too many unique")
	}
}

func BenchmarkLruEvict(b *testing.B) {
	keys := getInts()

	m := LruMap{Capacity: MAX_INT / 2}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%MAX_INT]
		m.GetOrAddF(key,
			func() (k string, v interface{}) {
				return key, key
			})
	}
}

func BenchmarkLruGetOrAddPar(b *testing.B) {
	keys := getInts()

	m := LruMap{Capacity: 2 * MAX_INT}

	var index int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(atomic.AddInt64(&index, 1))
			key := keys[i%MAX_INT]
			m.GetOrAddF(key,
				func() (k string, v interface{}) {
					return key, key
				})
		}
	})
}

func BenchmarkLruEvictPar(b *testing.B) {
	keys := getInts()

	m := LruMap{Capacity: MAX_INT / 2}

	var index int64

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(atomic.AddInt64(&index, 1))
			key := keys[i%MAX_INT]
			m.GetOrAddF(key,
				func() (k string, v interface{}) {
					return key, key
				})
		}
	})
}

func BenchmarkNoCache(b *testing.B) {

	compute := func(s string) (string, int) {
		return s, len(s)
	}

	for i := 0; i < b.N; i++ {
		key := strconv.Itoa(i % MAX_INT)
		compute(key)
	}
}

func BenchmarkNoCachePar(b *testing.B) {

	var index int64

	compute := func(s string) (string, int) {
		return s, len(s)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			i := int(atomic.AddInt64(&index, 1))
			key := strconv.Itoa(i % MAX_INT)
			compute(key)
		}
	})
}
