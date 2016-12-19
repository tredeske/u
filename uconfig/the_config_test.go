package uconfig

import (
	"os"
	"testing"
	"time"
)

func TestSubs(t *testing.T) {

	globals, err := NewSection(`
substitutions:
    one:        gVal

subbed:         "{{.one}}"
`)
	if err != nil {
		t.Fatal(err)
	}

	//
	// watch out for tabs in this string - no tabs!
	//
	config, err := globals.NewChild(
		`
substitutions:
    one:        oneVal
    two:        twoVal

home:           "${HOME}"
subbed:         "{{.one}}"
partial:        "{{.one}} {{.notSubbed}} {{.two}} caboose"
noMethod:       "{{.NoSuchMethod arg}}"
`)
	if err != nil {
		t.Fatal(err)
	}

	home, ok := os.LookupEnv("HOME")
	if !ok {
		t.Fatalf("Unable to lookup HOME env")
	}

	var s string
	err = config.GetString("home", &s)
	if err != nil {
		t.Fatal(err)
	} else if home != s {
		t.Fatalf("HOME: %s != %s", home, s)
	}

	err = config.GetString("subbed", &s)
	if err != nil {
		t.Fatal(err)
	} else if "oneVal" != s {
		t.Fatalf("subbed: %s != oneVal", s)
	}

	err = config.GetString("partial", &s)
	if err != nil {
		t.Fatal(err)
	} else if "oneVal {{.notSubbed}} twoVal caboose" != s {
		t.Fatalf("partial: %s != oneVal {{.notSubbed}} twoVal caboose", s)
	}

	err = config.GetString("noMethod", &s)
	if err != nil {
		t.Fatal(err)
	} else if "{{.NoSuchMethod arg}}" != s {
		t.Fatalf("noMethod: %s != {{.NoSuchMethod arg}}", s)
	}
}

func TestDiff(t *testing.T) {
	one, err := NewSection(map[string]interface{}{
		"hello": "world",
	})
	if err != nil {
		t.Fatal(err)
	}
	two, err := NewSection(map[string]interface{}{
		"hello": "world",
	})
	if err != nil {
		t.Fatal(err)
	}

	if one.DiffersFrom(two) {
		t.Fatal("Sections should be same")
	}

	one.Add("hello", "usa")
	if !one.DiffersFrom(two) {
		t.Fatal("Sections should not be the same")
	}
}

func TestGet(t *testing.T) {
	m := map[string]interface{}{
		"string":   "stringV",
		"int":      1,
		"float64":  2.0,
		"bool":     true,
		"duration": "2s",
	}
	s, err := NewSection(m)
	if err != nil {
		t.Fatal(err)
	}

	// look for things that should be there
	//
	stringV := ""
	intV := 0
	floatV := float64(0)
	boolV := false
	durV := time.Second
	var itV interface{}

	err = s.Chain().
		GetString("string", &stringV).
		GetInt("int", &intV).
		GetBool("bool", &boolV).
		GetIt("bool", &itV).
		GetFloat64("float64", &floatV).
		GetDuration("duration", &durV).
		Error
	if err != nil {
		t.Fatal(err)
	} else if stringV != "stringV" {
		t.Fatal("Strings did not match")
	} else if intV != 1 {
		t.Fatal("Ints did not match")
	} else if boolV != true {
		t.Fatal("Bools did not match")
	} else if floatV != 2.0 {
		t.Fatal("Floats did not match")
	} else if durV != 2*time.Second {
		t.Fatalf("Durations did not match: %s", durV)
	}
	if val, ok := itV.(bool); !ok {
		t.Fatal("unable to get bool val as interface{}")
	} else if val != boolV {
		t.Fatal("incorrect bool val when got as interface{}")
	}

	// look for things that should not be there
	// - defaults should be preserved
	//
	stringV = "default"
	intV = 7
	floatV = float64(77)
	boolV = true
	durV = 7 * time.Second

	err = s.Chain().
		GetString("!!string", &stringV).
		GetInt("!!int", &intV).
		GetBool("!!bool", &boolV).
		GetFloat64("!!float64", &floatV).
		GetDuration("!!duration", &durV).
		Error
	if err != nil {
		t.Fatal(err)
	} else if stringV != "default" {
		t.Fatal("Strings default overridden")
	} else if intV != 7 {
		t.Fatal("Ints default overridden")
	} else if boolV != true {
		t.Fatal("Bools default overridden")
	} else if floatV != float64(77) {
		t.Fatal("Floats default overridden")
	} else if durV != 7*time.Second {
		t.Fatal("Duration default overridden")
	}
}

func TestPath(t *testing.T) {
	thePath := "/tmp/configTestDir"
	os.RemoveAll(thePath)
	m := map[string]interface{}{
		"create":    thePath,
		"mustExist": thePath,
	}
	s, err := NewSection(m)
	if err != nil {
		t.Fatal(err)
	}
	var created, exists string
	err = s.Chain().
		GetCreateDir("create", &created, 02775).
		GetValidPath("mustExist", &exists).
		Error
	if err != nil {
		t.Fatal(err)
	}

	doesNotExist := ""
	err = s.GetCreateDir("doesNotExist", &doesNotExist, 02775)
	if nil == err {
		t.Fatal("Should have errored since '%s' empty", doesNotExist)
	}
}