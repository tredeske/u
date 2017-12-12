package ustrings

import (
	"fmt"
	"strings"
	"unsafe"
)

//
// If using this, if the initial []byte is mutated, so will the string!  So,
// use only with care.
//
// Normal conversion allocates a new string.  This has no allocations.
//
// Most efficient way to convert to string.  See:
//
// https://syslog.ravelin.com/byte-vs-string-in-go-d645b67ca7ff
//
// This is copied from runtime. It relies on the string
// header being a prefix of the slice header!
//
func UnsafeBytesToString(bs []byte) string {
	return *(*string)(unsafe.Pointer(&bs))
}

//
// Split the provided string, creating new strings in an array.
//
// None of the created strings will be empty
//
func SplitNonEmpty(s, sep string) (rv []string) {
	rv = make([]string, strings.Count(s, sep)+1) // guess at size
	n := 0
	StringEach(s, sep, func(s string) {
		if 0 != len(s) {
			rv[n] = s
			n++
		}
	})
	return rv[:n]
}

//
// Invoke the func for each string in s found between sep.
//
func StringEach(s, sep string, f func(string)) {
	c := sep[0]
	start := 0
	for i := 0; i+len(sep) <= len(s); i++ {
		if s[i] == c && (len(sep) == 1 || s[i:i+len(sep)] == sep) {
			f(s[start:i])
			start = i + len(sep)
			i += len(sep) - 1
		}
	}
	if start != len(s) {
		f(s[start:])
	}
}

//
// Get the part of the string before the specified rune
//
func BeforeRune(s string, r rune) (rv string) {
	idx := strings.IndexRune(s, r)
	if -1 != idx {
		rv = s[:idx]
	}
	return
}

//
//----------------------------------------------------------------------------
//

//
// do these two differ?  especially useful for *regexp.Regexp and a few others
//
func StringersDiffer(lhs, rhs fmt.Stringer) (rv bool) {
	lNil, rNil := ItIsNil(lhs), ItIsNil(rhs)
	return lNil && !rNil || !lNil && (rNil || lhs.String() != rhs.String())
}
