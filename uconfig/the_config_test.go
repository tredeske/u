package uconfig

import (
	"fmt"
	nurl "net/url"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/tredeske/u/ulog"
)

func TestSource(t *testing.T) {

	source := map[string]string{
		"hello": "there",
	}

	s, err := NewSection(source)
	if err != nil {
		t.Fatalf("Unable to create section from map[string]string: %s", err)
	}

	there := ""
	err = s.GetString("hello", &there)
	if err != nil {
		t.Fatalf("Unable to retrieve 'hello': %s", err)
	} else if there != "there" {
		t.Fatalf("Did not get 'there', got '%s'", there)
	}
}

func TestProps(t *testing.T) {

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
properties:
    one:        oneVal
    two:        twoVal
    three:      "{{.one}}"
    noEscape:   "no+escape"
    includeD:   "."
    include_:   "{{.includeD}}/include_props.yml"

home:           "${HOME}"
subbed:         "{{.one}}"
doubleSubbed:   "{{.three}}"
partial:        "{{.one}} {{.notSubbed}} {{.two}} caboose"
noMethod:       "{{.NoSuchMethod arg}}"
include_:       include.yml
includedSub:    "{{.five}}"
array:
- A:            A_VAL
- include_:     array_include.yml
- Z:            Z_VAL
noEscapeCheck:  "{{.noEscape}}"
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

	err = config.GetString("doubleSubbed", &s)
	if err != nil {
		t.Fatal(err)
	} else if "oneVal" != s {
		t.Fatalf("doubleSubbed: %s != oneVal", s)
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

	err = config.GetString("includedSub", &s)
	if err != nil {
		t.Fatal(err)
	} else if "fiveV" != s {
		t.Fatalf("includedSub: '%s' != fiveV : %#v", s, config.Props())
	}

	s = "unset"
	err = config.GetString("foo", &s)
	if err != nil {
		t.Fatal(err)
	} else if "bar" != s {
		t.Fatalf("include: %s != bar", s)
	}

	var array *Array
	err = config.GetArray("array", &array)
	if err != nil {
		t.Fatal(err)
	} else if 4 != array.Len() {
		t.Fatalf("not 4 in len: %#v", array)
	}

	child := array.Get(0)
	s = "unset"
	err = child.GetString("A", &s)
	if err != nil {
		t.Fatal(err)
	} else if "A_VAL" != s {
		t.Fatalf("array: %s != A_VAL", s)
	}

	child = array.Get(2)
	s = "unset"
	err = child.GetString("foo", &s)
	if err != nil {
		t.Fatal(err)
	} else if "bar" != s {
		t.Fatalf("array entry include: %s != bar", s)
	}

	child = array.Get(3)
	s = "unset"
	err = child.GetString("Z", &s)
	if err != nil {
		t.Fatal(err)
	} else if "Z_VAL" != s {
		t.Fatalf("array: %s != Z_VAL", s)
	}

	//
	// ensure we are using text/template instead of html/template to avoid
	// changing chars like '+' into escape sequences
	//
	s = "unset"
	err = config.GetString("noEscapeCheck", &s)
	if err != nil {
		t.Fatal(err)
	} else if "no+escape" != s {
		t.Fatalf("include: %s != 'no+escape'", s)
	}

	//
	//
	//
	m := config.AsResolvedMap()
	ulog.Printf("%#v", m)
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

func TestGetString(t *testing.T) {
	success := map[string]interface{}{
		"string":     "stringV",
		"bool":       true,
		"int":        1,
		"float":      2.0,
		"intString1": "1",
		"intString2": "1M",
	}
	s, err := NewSection(success)
	if err != nil {
		t.Fatal(err)
	}

	for k, _ := range success {
		result := ""
		err = s.Chain().
			GetString(k, &result, StringNotBlank).
			Error
		if err != nil {
			t.Fatalf("Unable to get string: %s", err)
		} else if 0 == len(result) {
			t.Fatalf("string result not set")
		}
	}
	result := ""
	err = s.GetString("string", &result, StringOneOf("stringV"))
	if err != nil {
		t.Fatalf("Unable to get string with validation: %s", err)
	} else if 0 == len(result) {
		t.Fatalf("string result not set")
	}
}

func TestGetStringFails(t *testing.T) {

	m := map[string]interface{}{
		"nil":   nil,
		"array": []string{"foo", "bar"},
		"map":   map[string]string{"foo": "bar"},
	}
	s, err := NewSection(m)
	if err != nil {
		t.Fatal(err)
	}
	for k, _ := range m {
		result := ""
		err = s.Chain().
			GetString(k, &result).
			Error
		if nil == err {
			t.Fatalf("Should have failed to get string %s. got (%s)", k, result)
		} else if 0 != len(result) {
			t.Fatalf("Result should not be set")
		}
	}

	m = map[string]interface{}{
		"blank":  "",
		"choice": "choice_5",
	}
	s, err = NewSection(m)
	if err != nil {
		t.Fatal(err)
	}
	result := ""
	err = s.GetString("blank", &result, StringNotBlank)
	if nil == err {
		t.Fatalf("Should be blank")
	} else if 0 != len(result) {
		t.Fatalf("Result should not be set")
	}

	err = s.GetString("choice", &result, StringOneOf("choice_1", "choice_2"))
	if nil == err {
		t.Fatalf("Should have failed due to bad choice")
	} else if 0 != len(result) {
		t.Fatalf("Result should not be set")
	}
}

func TestGetInt(t *testing.T) {
	const BAD_VALUE int = 2020
	validIntF := func(v int64) (err error) {
		if int64(BAD_VALUE) == v {
			err = fmt.Errorf("value did not change")
		}
		return
	}
	validUIntF := func(v uint64) (err error) {
		if uint64(BAD_VALUE) == v {
			err = fmt.Errorf("value did not change")
		}
		return
	}

	m := map[string]interface{}{
		"string":     "stringV",
		"bool":       true,
		"duration":   "2s",
		"int":        1,
		"float":      2.0,
		"intString1": "1",
		"intString2": "1M",
	}
	failKeys := []string{"string", "bool", "duration"}
	type Checker struct {
		key   string
		value interface{}
	}
	anInt := 0
	var anInt64 int64
	var anInt32 int32
	var anInt16 int16
	var aUInt uint
	var aUInt64 uint64
	var aUInt32 uint32
	var aUInt16 uint16
	success := []Checker{
		Checker{"int", &anInt},
		Checker{"int", &anInt64},
		Checker{"int", &anInt32},
		Checker{"int", &anInt16},
		Checker{"int", &aUInt},
		Checker{"int", &aUInt64},
		Checker{"int", &aUInt32},
		Checker{"int", &aUInt16},
		Checker{"float", &anInt},
		Checker{"intString1", &anInt},
		Checker{"intString2", &anInt},
	}

	s, err := NewSection(m)
	if err != nil {
		t.Fatal(err)
	}

	//
	// these should fail
	//
	result := BAD_VALUE
	for _, key := range failKeys {
		err = s.GetInt(key, &result)
		if nil == err {
			t.Fatalf("Should fail (key=%s): %s", key, err)
		} else if BAD_VALUE != result {
			t.Fatalf("Should not have changed (key=%s)", key)
		}
	}

	//
	// int8
	//
	const I8_BAD int8 = 20
	var i8 int8 = I8_BAD
	err = s.GetInt("intString1", &i8)
	if err != nil {
		t.Fatalf("int8 failed! - %s", err)
	} else if I8_BAD == i8 {
		t.Fatalf("Should have changed int8")
	}
	i8 = I8_BAD
	err = s.GetInt("intString2", &i8)
	if nil == err {
		t.Fatalf("int8: Should fail") // too large
	} else if I8_BAD != i8 {
		t.Fatalf("Should NOT have changed int8")
	}

	//
	// these should succeed
	//
	for _, checker := range success {
		key := checker.key
		Assign("test", checker.value, BAD_VALUE)
		//fmt.Printf("HERE %d\n", checker.value)
		switch checker.value.(type) {
		case *int, *int64, *int32, *int16, *int8:
			err = s.GetInt(key, checker.value, validIntF, ValidRange(0, 2000000))
		default:
			err = s.GetUInt(key, checker.value, validUIntF)
		}
		if nil != err {
			t.Fatalf("Should succeed (key=%s): %s", key, err)
		}
	}

}

func TestGet(t *testing.T) {
	m := map[string]interface{}{
		"string":   "stringV",
		"int":      1,
		"float64":  2.0,
		"bool":     true,
		"duration": "2s",
		"regexp":   "foo.*bar",
		"url":      "http://host/path",
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
	var re *regexp.Regexp
	var reExists *regexp.Regexp
	var reNotSet *regexp.Regexp
	var url *nurl.URL
	var urlExists *nurl.URL
	var urlNotExists *nurl.URL

	err = s.Chain().
		GetString("string", &stringV).
		GetInt("int", &intV, nil).
		GetBool("bool", &boolV).
		GetIt("bool", &itV).
		GetFloat64("float64", &floatV).
		GetDuration("duration", &durV).
		GetRegexp("regexp", &re).
		GetRegexpIf("regexp", &reExists).
		GetRegexpIf("regexp-does-not-exist", &reNotSet).
		GetUrl("url", &url).
		GetUrlIf("url", &urlExists).
		GetUrlIf("url-does-not-exist", &urlNotExists).
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
	} else if nil == re {
		t.Fatalf("regexp did not get set")
	} else if nil == reExists {
		t.Fatalf("regexp Exists did not get set")
	} else if nil != reNotSet {
		t.Fatalf("regexp !Exists did not get set")
	} else if nil == url {
		t.Fatalf("url did not get set")
	} else if nil == urlExists {
		t.Fatalf("url Exists did not get set")
	} else if nil != urlNotExists {
		t.Fatalf("url !Exists did not get set")
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
		t.Fatalf("Should have errored since '%s' empty", doesNotExist)
	}
}
