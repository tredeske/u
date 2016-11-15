package ustrings

//
// a set of strings
//
type Set map[string]struct{}

var tokenStruct_ struct{}

//
// remove all strings from the set
//
func (this Set) Reset() {
	for s, _ := range this {
		delete(this, s)
	}
}

//
// remove a specific string from the set
//
func (this Set) Clear(s string) {
	delete(this, s)
}

//
// remove a specific string from the set, returning whether it was set
//
func (this Set) ClearIfSet(s string) (rv bool) {
	rv = this.IsSet(s)
	if rv {
		delete(this, s)
	}
	return
}

//
// add a string to the set
//
func (this Set) Set(s string) {
	this[s] = tokenStruct_
}

//
// add a bunch of strings to the set
//
func (this Set) SetAll(arr []string) {
	for _, s := range arr {
		this[s] = tokenStruct_
	}
}

//
// is the string in the set?
//
func (this Set) IsSet(s string) (rv bool) {
	_, rv = this[s]
	return
}
