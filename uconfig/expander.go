package uconfig

import (
	"bytes"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/template"
)

//
// expander expands ${...} with ENV vars and {{...}} with properties
//
type expander_ struct {
	watch   *Watch
	mapping map[string]string
}

func (this *expander_) Set(key, value string) {
	this.mapping[key] = value
}

func (this expander_) Get(key string) (rv string) {
	return this.mapping[key]
}

func (this *expander_) expand(value string) (rv string) {
	if strings.Contains(value, "${") {
		value = os.ExpandEnv(value)
	}
	if strings.Contains(value, "{{") && strings.Contains(value, "}}") {
		//
		// any errors - just return the unresolved text
		//
		t, err := template.New("").Option("missingkey=error").Parse(value)
		if nil == err {
			var buff bytes.Buffer
			buff.Grow(len(value))
			err = t.Execute(&buff, *this)
			if err != nil { // try again, more carefully
				buff.Reset()
				this.carefully(&buff, value)
			}
			return strings.TrimSpace(buff.String())
		}
	}
	return strings.TrimSpace(value)
}

//
// carefully expand each individual {{...}} group.  if one doesn't expand,
// then put it in unexpanded.
//
func (this *expander_) carefully(buff *bytes.Buffer, value string) {

	pos := 0
	for {
		slice := value[pos:]
		beg := strings.Index(slice, "{{")
		if -1 == beg {
			break
		}
		end := strings.Index(slice[beg+2:], "}}")
		if -1 == end {
			break
		}
		end += beg + 4
		pos += end
		buff.WriteString(slice[:beg])

		tstring := slice[beg:end]
		t, err := template.New("").Option("missingkey=error").Parse(tstring)
		if err != nil { // template text not really template text - put it in
			buff.WriteString(tstring)
			continue
		}
		pos := buff.Len()
		err = t.Execute(buff, this.mapping)
		if err != nil { // not resolved - just put in template text
			buff.Truncate(pos)
			buff.WriteString(tstring)
		}
	}
	buff.WriteString(value[pos:])
}

func newExpander(watch *Watch) (rv expander_) {
	if nil == watch {
		watch = &Watch{}
	}
	rv = expander_{
		mapping: make(map[string]string),
		watch:   watch,
	}
	if user, home := UserInfo(); "" != user {
		rv.mapping["user"] = user
		rv.mapping["home"] = home
	}
	rv.mapping["installDir"] = InstallD
	rv.mapping["initDir"] = InitD
	rv.mapping["host"] = ThisHost
	rv.mapping["processName"] = ThisProcess
	return
}

func (this *expander_) clone() (rv expander_) {
	rv = newExpander(this.watch)
	rv.addAll(this.mapping)
	return
}

func (this *expander_) addAll(m map[string]string) (err error) {

	//
	// add in any regular entries and record any includes
	//
	// we support include_, include_1, include_2, .... include_N
	//
	var includes []string
	for k, v := range m {
		if strings.HasPrefix(k, include_) {
			includes = append(includes, v)
		} else {
			this.mapping[k] = v
		}
	}

	//
	// try to do some expansion
	//
	this.expandAll()
	this.expandAll()

	//
	// add in any includes using depth first
	//
	if 0 != len(includes) {
		sort.Strings(includes)
		for _, include := range includes {
			includeF := this.expand(include)
			err = this.loadInclude(includeF)
			if err != nil {
				return
			}
		}
	}

	//
	// just in case
	//
	this.expandAll()
	this.expandAll()
	return
}

func (this *expander_) expandAll() {
	for k, v := range this.mapping {
		this.mapping[k] = this.expand(v)
	}
}

func (this *expander_) loadInclude(includeF string) (err error) {
	this.watch.Add(includeF)
	var included map[string]interface{}
	err = YamlLoad(includeF, &included)
	if err != nil {
		return
	}
	m := make(map[string]string)
	for k, v := range included {
		str, converted := asString(v, false)
		if !converted {
			err = fmt.Errorf("Unable to convert value to string: %#v", v)
			return
		}
		m[k] = str
	}
	err = this.addAll(m) ////////// recurse
	return
}

func (this expander_) Dump() (rv string) {
	return fmt.Sprintf("%#v", this.mapping)
}
