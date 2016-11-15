package ustrings

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

//
// modify contents of input slice for order, uniqueness, returning sliced result
// - minimizes allocation
//
func SortUnique(a []string) (rv []string) {

	sort.Strings(a)

	pos := 1
	last := a[0]
	for i := 1; i < len(a); i++ {
		s := a[i]
		if s != last {
			if pos != i {
				a[pos] = a[i]
			}
			pos++
		}
		last = s
	}
	rv = a[:pos]
	return
}

//
// remove duplicate strings, preserving order, not modifying source.
// return deduped slice of strings
// requires allocation of a map for deduping
//
func Dedup(a []string) (rv []string) {
	var space struct{}
	m := make(map[string]struct{})
	rv = make([]string, 0, len(a)/2)
	for _, s := range a {
		if _, found := m[s]; !found {
			rv = append(rv, s)
			m[s] = space
		}
	}
	return
}

//
// remove duplicate strings, preserving order, modifying source
//
func DedupInPlace(a *[]string) {
	sz := len(*a)
	if 2 > sz {
		return
	}
	last := sz - 1
	for i := 0; i < last; i++ {
		s := (*a)[i]
		for j := last; j > i; j-- {
			if s == (*a)[j] { // found a dup - remove it
				if j == last {
					(*a) = (*a)[:last]
				} else {
					(*a) = append((*a)[:j], (*a)[j+1:]...)
				}
				last--
			}
		}
	}
}

//
// create a new slice of strings by cutting the specified field out
// of the source strings
//
func Cut(a []string, field int) (rv []string) {
	rv = make([]string, len(a))
	for i, s := range a {
		fields := strings.Fields(s)
		if field <= len(fields) {
			rv[i] = fields[field]
		}
	}
	return
}

//
//----------------------------------------------------------------------------
//

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
//----------------------------------------------------------------------------
//

//
// delete element from string slice preserving order
//
func Delete(idx int, arr []string) []string {
	if idx+1 == len(arr) {
		return arr[:idx]
	} else {
		return append(arr[:idx], arr[idx+1:]...)
	}
}

//
// Is the string in the slice?
//
func Contains(s string, arr []string) bool {
	for _, el := range arr {
		if el == s {
			return true
		}
	}
	return false
}

//
// do any elements of the slice match the expression?
//
func Matches(re *regexp.Regexp, arr []string) bool {
	for _, el := range arr {
		if re.MatchString(el) {
			return true
		}
	}
	return false
}

//
// shallow copy of the strings
//
func Clone(in []string) (out []string) {
	out = make([]string, len(in))
	copy(out, in)
	return
}

//
// do the slices contain the same strings in the same order?
//
func Equal(lhs, rhs []string) (rv bool) {
	if len(lhs) == len(rhs) {
		for i, s := range lhs {
			if s != rhs[i] {
				return false
			}
		}
		rv = true
	}
	return
}

//
// do these two differ?  especially useful for *regexp.Regexp and a few others
//
func StringersDiffer(lhs, rhs fmt.Stringer) bool {
	return nil == lhs && nil != rhs ||
		nil != lhs && (nil == rhs || lhs.String() != rhs.String())
}
