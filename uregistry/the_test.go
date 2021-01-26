package uregistry

import "testing"

func TestRegistry(t *testing.T) {
	putM := map[string]string{
		"one": "hello",
		"two": "there",
	}
	for k, v := range putM {
		Put(k, v)
	}

	// test found
	//
	for k, v := range putM {
		s := ""

		err := Get(k, &s)
		if err != nil {
			t.Fatalf("Unable to get '%s': %s", k, err)
		} else if v != s {
			t.Fatalf("Value of '%s' is '%s', should be '%s'", k, s, v)
		}

		err = GetValid(k, &s)
		if err != nil {
			t.Fatalf("Unable to get '%s': %s", k, err)
		} else if v != s {
			t.Fatalf("Value of '%s' is '%s', should be '%s'", k, s, v)
		}

		found := false
		found, err = GetOk(k, &s)
		if err != nil {
			t.Fatalf("Unable to get '%s': %s", k, err)
		} else if v != s {
			t.Fatalf("Value of '%s' is '%s', should be '%s'", k, s, v)
		} else if !found {
			t.Fatalf("Should have found '%s", k)
		}
	}

	// test not found
	//
	val := ""
	err := Get("three", &val)
	if err != nil {
		t.Fatalf("Unable to get 'three': %s", err)
	} else if 0 != len(val) {
		t.Fatalf("No value expected for 'three', got '%s'", val)
	}

	err = GetValid("three", &val)
	if nil == err {
		t.Fatalf("Should have errored since should be no 'three'")
	}

	// test type
	//
	bval := true
	err = Get("bval", &bval)
	if err != nil {
		t.Fatalf("Unable to get 'bval': %s", err)
	} else if !bval {
		t.Fatalf("No value expected for 'bval', got '%t'", bval)
	}

	err = Get("one", &bval) // wrong type
	if nil == err {
		t.Fatalf("Should have got an error for wrong type")
	}

	// test remove
	//
	for k, v := range putM {
		s := ""

		err := Remove(k, &s)
		if err != nil {
			t.Fatalf("Unable to remove '%s': %s", k, err)
		} else if v != s {
			t.Fatalf("Value of '%s' is '%s', should be '%s'", k, s, v)
		}

		found := false
		found, err = GetOk(k, &s)
		if err != nil {
			t.Fatalf("GetOk failed: %s", err)
		} else if found {
			t.Fatalf("Value of '%s' should have been removed", k)
		}

	}
}
