package ustrings

import (
	"fmt"
	"reflect"
	"regexp"
	"testing"
)

func TestSortUnique(t *testing.T) {

	source := []string{
		"c: hello",
		"b: hello",
		"a: hello",
		"a: hello",
		"b: hello",
		"c: hello",
	}
	expected := []string{
		"a: hello",
		"b: hello",
		"c: hello",
	}

	result := SortUnique(append([]string{}, source...))
	if !Equal(result, expected) {
		t.Fatalf("Not Equal!  %#v != %#v", result, expected)
	} else if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Not Equal (reflect)!  %#v != %#v", result, expected)
	}
}

func TestSortUnique2(t *testing.T) {

	source := []string{
		"9408ef1f51f978b89f978fc4acbd14f036f0b2cc",
		"52a649b0ea42371e09b46262fe0e3391ea931fb3",
		"cb637e085537e50a952793a3657dd84545f465ca",
		"717c96f9355f7a51272f9fdf30ccb4d8da90bf93",
		"720bd7f07f24d01cdc447a5453620cb1bdc0fb4f",
		"9408ef1f51f978b89f978fc4acbd14f036f0b2cc",
		"0d7c09245137f9a4708ce946ec142c0a21046b93",
		"186fdfd7ff37a980b1dcc12fdf2e8671f9d45888",
		"1a9f9d6dcb5ec93ba2ce74aaf8b550e90a9244b7",
		"1d9b8cc24c62ec7fb824f66f4a8e05ba2a4c5cd4",
		"8fa446321b69430378803fce188161363c07a17a",
		"e9b8f846817194927a87dfb849db5d3f229a76a3",
		"f997e090ceeab30f043bbe1f61049e8b2e2522a3",
		"22776712bcd3073fbc9d02abeaef6762c04c4bf4",
		"9a2146f7369f44f4f9dc7b226df44284e503925e",
		"a03d83d9e10e9d7c604357c9fede29b032403dc5",
	}

	expected := []string{
		"0d7c09245137f9a4708ce946ec142c0a21046b93",
		"186fdfd7ff37a980b1dcc12fdf2e8671f9d45888",
		"1a9f9d6dcb5ec93ba2ce74aaf8b550e90a9244b7",
		"1d9b8cc24c62ec7fb824f66f4a8e05ba2a4c5cd4",
		"22776712bcd3073fbc9d02abeaef6762c04c4bf4",
		"52a649b0ea42371e09b46262fe0e3391ea931fb3",
		"717c96f9355f7a51272f9fdf30ccb4d8da90bf93",
		"720bd7f07f24d01cdc447a5453620cb1bdc0fb4f",
		"8fa446321b69430378803fce188161363c07a17a",
		"9408ef1f51f978b89f978fc4acbd14f036f0b2cc",
		"9a2146f7369f44f4f9dc7b226df44284e503925e",
		"a03d83d9e10e9d7c604357c9fede29b032403dc5",
		"cb637e085537e50a952793a3657dd84545f465ca",
		"e9b8f846817194927a87dfb849db5d3f229a76a3",
		"f997e090ceeab30f043bbe1f61049e8b2e2522a3",
	}

	result := SortUnique(append([]string{}, source...))
	if !Equal(result, expected) {
		t.Fatalf("Not Equal!  %#v != %#v", result, expected)
	} else if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Not Equal (reflect)!  %#v != %#v", result, expected)
	}
}

func TestDedup(t *testing.T) {

	source := []string{
		"c: hello",
		"b: hello",
		"a: hello",
		"a: hello",
		"b: hello",
		"c: hello",
	}
	expected := []string{
		"c: hello",
		"b: hello",
		"a: hello",
	}

	result := Dedup(source)
	if !Equal(result, expected) {
		t.Fatalf("Not Equal!  %#v != %#v", result, expected)
	} else if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Not Equal (reflect)!  %#v != %#v", result, expected)
	}
}

func TestDedupInPlace(t *testing.T) {

	source := []string{
		"c: hello",
		"c: hello",
		"b: hello",
		"a: hello",
		"c: hello",
		"a: hello",
		"b: hello",
		"c: hello",
	}
	expected := []string{
		"c: hello",
		"b: hello",
		"a: hello",
	}

	DedupInPlace(&source)
	if !Equal(source, expected) {
		t.Fatalf("Not Equal!  %#v != %#v", source, expected)
	} else if !reflect.DeepEqual(source, expected) {
		t.Fatalf("Not Equal (reflect)!  %#v != %#v", source, expected)
	}
}

func TestCut(t *testing.T) {

	source := []string{
		"c: hello",
		"b: hello",
		"a: hello",
		"a: hello",
		"b: hello",
		"c: hello",
	}
	expected := []string{
		"c:",
		"b:",
		"a:",
		"a:",
		"b:",
		"c:",
	}

	result := Cut(source, 0)
	if !Equal(result, expected) {
		t.Fatalf("Not Equal!  %#v != %#v", result, expected)
	} else if !reflect.DeepEqual(result, expected) {
		t.Fatalf("Not Equal (reflect)!  %#v != %#v", result, expected)
	}
}

func TestMatch(t *testing.T) {

	source := []string{
		"c: hello",
		"b: jello",
		"a: bello",
		"a: mello",
		"b: vello",
		"c: yello",
	}

	exists := regexp.MustCompile("mello")
	existsNot := regexp.MustCompile("blah")

	if !Matches(source, exists) {
		t.Fatalf("Should exist!")
	}
	if Matches(source, existsNot) {
		t.Fatalf("Should NOT exist!")
	}

	idx, match := FindFirstSubmatch(source, exists)
	if nil == match {
		t.Fatalf("No match found!")
	} else if 3 != idx {
		t.Fatalf("Index should be 3, is %d", idx)
	} else if "mello" != match[0] {
		t.Fatalf("match should be 'mello', is '%s'", match[0])
	}

}

func TestSet(t *testing.T) {
	set := Set{}

	//
	// test set
	//
	set.Set("wow")
	if !set.IsSet("wow") {
		t.Fatalf("'wow' not set")
	}
	if 1 != len(set) {
		t.Fatalf("len not 1")
	}

	//
	// test clear
	//
	set.Clear("wow")
	if set.IsSet("wow") {
		t.Fatalf("'wow' is set")
	}

	//
	// test reset
	//
	set.Set("1")
	if !set.IsSet("1") {
		t.Fatalf("'1' not set")
	}
	set.Set("2")
	if !set.IsSet("2") {
		t.Fatalf("'2' not set")
	}
	if 2 != len(set) {
		t.Fatalf("len not correct")
	}
	set.Reset()

	if set.IsSet("1") {
		t.Fatalf("'1' not reset")
	}
	if set.IsSet("2") {
		t.Fatalf("'2' not reset")
	}
	if 0 != len(set) {
		t.Fatalf("len not reset")
	}
}

func TestStringSplit(t *testing.T) {
	s := "::a:b:c:d"
	split := SplitNonEmpty(s, ":")
	if 4 != len(split) {
		t.Fatalf("not correct number of split: %d", len(split))
	}

	s = "a:b:::c:"
	split = SplitNonEmpty(s, ":")
	if 3 != len(split) {
		t.Fatalf("not correct number of split: %d", len(split))
	}
}

func TestStringersDiffer(t *testing.T) {
	var isNil fmt.Stringer
	var containsNil fmt.Stringer = (*regexp.Regexp)(nil)
	var notNil fmt.Stringer = regexp.MustCompile(".")

	if !ItIsNil(nil) {
		t.Fatalf("nil should be nil")
	} else if !ItIsNil(isNil) {
		t.Fatalf("isNil should be nil")
	} else if !ItIsNil(containsNil) {
		t.Fatalf("containsNil should be nil")
	} else if ItIsNil(notNil) {
		t.Fatalf("notNil should not be nil")
	}

	if StringersDiffer(nil, nil) {
		t.Fatalf("nils should be the same")
	} else if StringersDiffer(nil, isNil) {
		t.Fatalf("nils should be the same")
	} else if StringersDiffer(nil, containsNil) {
		t.Fatalf("nils should be the same")
	} else if !StringersDiffer(nil, notNil) {
		t.Fatalf("nils should be the same")
	}

	if StringersDiffer(isNil, nil) {
		t.Fatalf("nils should be the same")
	} else if StringersDiffer(isNil, isNil) {
		t.Fatalf("nils should be the same")
	} else if StringersDiffer(isNil, containsNil) {
		t.Fatalf("nils should be the same")
	} else if !StringersDiffer(isNil, notNil) {
		t.Fatalf("nils should be the same")
	}

	if StringersDiffer(containsNil, nil) {
		t.Fatalf("nils should be the same")
	} else if StringersDiffer(containsNil, isNil) {
		t.Fatalf("nils should be the same")
	} else if StringersDiffer(containsNil, containsNil) {
		t.Fatalf("nils should be the same")
	} else if !StringersDiffer(containsNil, notNil) {
		t.Fatalf("nils should be the same")
	}

	if !StringersDiffer(notNil, nil) {
		t.Fatalf("nils should differ")
	} else if !StringersDiffer(notNil, isNil) {
		t.Fatalf("nils should differ")
	} else if !StringersDiffer(notNil, containsNil) {
		t.Fatalf("nils should differ")
	} else if StringersDiffer(notNil, notNil) {
		t.Fatalf("nils should be the same")
	}
}
