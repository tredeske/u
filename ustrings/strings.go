package ustrings

import (
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
func DedupInPlace(a *[]string) (dups int) {
	sz := len(*a)
	if 2 > sz {
		return
	}
	last := sz - 1
	for i := 0; i < last; i++ {
		s := (*a)[i]
		for j := last; j > i; j-- {
			if s == (*a)[j] { // found a dup - remove it
				dups++
				if j == last {
					(*a) = (*a)[:last]
				} else {
					(*a) = append((*a)[:j], (*a)[j+1:]...)
				}
				last--
			}
		}
	}
	return
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
// make a slice without the specified element, preserving order
//
func Delete(arr []string, idx int) []string {
	if 0 == idx {
		return arr[1:]
	} else if idx+1 == len(arr) {
		return arr[:idx]
	} else {
		return append(arr[:idx], arr[idx+1:]...)
	}
}

//
// Is the string in the slice?
//
func Contains(arr []string, s string) bool {
	for _, el := range arr {
		if el == s {
			return true
		}
	}
	return false
}

//
// Index of string in slice or -1
//
func ContainsIndex(arr []string, s string) int {
	for idx, el := range arr {
		if el == s {
			return idx
		}
	}
	return -1
}

//
// do any elements of the slice match the expression?
//
func Matches(arr []string, re *regexp.Regexp) bool {
	for _, el := range arr {
		if re.MatchString(el) {
			return true
		}
	}
	return false
}

//
// Find first element that matches and return submatch info, or nil
//
func FindFirstSubmatch(arr []string, re *regexp.Regexp,
) (idx int, match []string) {
	for i, el := range arr {
		match = re.FindStringSubmatch(el)
		if nil != match {
			idx = i
			break
		}
	}
	return
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

// based on std lib strings.Join
func Concat(in []string) (rv string) {

	switch len(in) {

	case 0:
		return ""

	case 1:
		return in[0]

	case 2:
		return in[0] + in[1]

	case 3:
		return in[0] + in[1] + in[2]
	}

	n := 0

	for i := 0; i < len(in); i++ {
		n += len(in[i])
	}

	b := make([]byte, n)

	bp := copy(b, in[0])

	for _, s := range in[1:] {
		bp += copy(b[bp:], s)
	}

	return string(b)
}
