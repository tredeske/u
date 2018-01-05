package usync

import "testing"

func TestMap(t *testing.T) {
	var m Map

	key := "key"
	value := "wow"

	if 0 != m.Len() {
		t.Fatalf("Initial size not zero!")
	}

	m.Add(key, value)

	rv, ok := m.Get(key).(string)
	if !ok {
		t.Fatalf("Did not get back a string")
	} else if rv != value {
		t.Fatalf("Did not get correct value")
	}

	if 1 != m.Len() {
		t.Fatalf("Add size not 1!")
	}

	it := m.Remove(key)
	if nil == it {
		t.Fatalf("Unable to remove value")
	}

	rv, ok = m.Get(key).(string)
	if ok {
		t.Fatalf("It was still there!")
	} else if 0 != m.Len() {
		t.Fatalf("Remove size not zero!")
	}

	m.Add(key, value)
	m.Add("hello", "there")

	m.RemoveUsing(

		func(k string, v interface{}) bool {
			return key == k
		})

	rv, ok = m.Get(key).(string)
	if ok {
		t.Fatalf("RemoveUsing: It was still there!")
	} else if 1 != m.Len() {
		t.Fatalf("RemoveUsing size not 1!")
	}

}
