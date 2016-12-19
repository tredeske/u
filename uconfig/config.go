package uconfig

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	nurl "net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"

	"gopkg.in/yaml.v2"
)

const SUBS = "substitutions"

//
// A map of strings to values used for config.
//
// If the section has a key called "substitions", then the keys in that sub
// section will be added as substitutions into the current one.
//
// As string values are accessed, the values also become substitution parameters
// for expanding later accessed string values.
//
// Expansion occurs when a string value contains ${...} or {{...}}.
//
// The ${...} will be filled in with ENV variables, if available.
//
// The {{...}} will be filled in with substitution parameters as per the
// go text/template package.  ***NOTE*** Make sure to add a '.' before
// the key.  As in: {{.key}}.  All golang text template rules apply.
//
// Child sections inherit the substitutions of their parents.
//
// The following substitutions are automatically added:
// - home
// - user
// - host
// - processName
// - installDir
// - initDir
//
type Section struct {
	Context  string
	expander expander_
	section  map[string]interface{}
}

//
// an array of config sections
//
type Array struct {
	Context  string
	expander expander_
	sections []map[string]interface{}
}

//
// create a new Section from nil, /path/to/yaml/file, YAML string,
// or map[string]interface{}
//
func NewSection(it interface{}) (rv *Section, err error) {
	tmp := Section{}
	return tmp.NewChild(it)
}

//
// create a new Section as a child of this one from nil, /path/to/yaml/file,
// YAML string, or map[string]interface{}
//
func (this *Section) NewChild(it interface{}) (rv *Section, err error) {
	rv = &Section{
		expander: this.expander.clone(),
	}
	rv.section, err = rv.getMap(it)
	if err != nil {
		return nil, err
	}
	err = rv.addSubs()
	return
}

//
// coerce nil, string, []byte, or map into correct section map type
//
func (this *Section) getMap(it interface{}) (rv map[string]interface{}, err error) {

	if nil == it {
		rv = make(map[string]interface{})
		return
	}
	switch val := it.(type) {
	case map[string]interface{}:
		rv = val
	case []byte:
		err = yaml.Unmarshal(val, &rv)
	case string:
		if 0 == len(val) { // empty string: treat same as nil
			rv = make(map[string]interface{})
		} else {
			_, err = os.Stat(val)
			if nil == err {
				err = YamlLoad(val, &rv)
			} else {
				err = yaml.Unmarshal([]byte(val), &rv)
			}
		}
	case map[interface{}]interface{}:
		rv = make(map[string]interface{}, len(val))
		for k, v := range val {
			if ks, ok := k.(string); !ok {
				err = fmt.Errorf("%s contains non string key", this.Context)
				break
			} else {
				rv[ks] = v
			}
		}
	default:
		err = fmt.Errorf("value not a config map. is a %s",
			reflect.TypeOf(it))
	}
	return
}

func (this *Section) DumpSubs() (rv string) {
	return this.expander.Dump()
}
func (this *Section) DumpVals() (rv string) {
	return fmt.Sprintf("%#v", this.section)
}

func (this *Section) NameContext(key string) {
	this.Context = this.Context + "." + key
}

func (this *Section) ctx(key string) string {
	if 0 == len(this.Context) {
		return key
	} else {
		return strings.Join([]string{this.Context, key}, ".")
	}
}

//
// load the YAML file into target, which may be a ptr to map or ptr to struct
//
func YamlLoad(file string, target interface{}) error {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(content, target)
}

//
// read in the specified yaml file, performing substitutions on the text, then
// unmarshal it into target (a ptr to struct)
//
func (this *Section) StructFromYaml(file string, target interface{}) error {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	expanded := this.Expand(string(content))
	return yaml.Unmarshal([]byte(expanded), target)
}

// write contents to yaml file
func (this *Section) ToYaml(file string) error {
	content, err := this.asYaml()
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, content, 0664)
}

func (this *Section) asYaml() ([]byte, error) {
	this.section[SUBS] = this.expander
	return yaml.Marshal(this.section)
}

// output contents to log as YAML
func (this *Section) Log() {
	content, err := this.asYaml()
	if err != nil {
		log.Printf("Unable to output config to log: %s", err)
	} else {
		log.Printf("Config:\n%s\n", content)
	}
}

func (this *Section) Len() int {
	return len(this.section)
}

func (this *Section) Contains(key string) (found bool) {
	_, found = this.getIt(key, true)
	return
}

// return a list of any keys found that are not on allowed list
func (this *Section) ExtraKeys(allowedKeys []string) (rv []string) {
	for k, _ := range this.section {
		found := false
		for _, allowed := range allowedKeys {
			if allowed == k {
				found = true
				break
			}
		}
		if !found {
			rv = append(rv, k)
		}
	}
	return
}

// return error if any other keys are specfied
func (this *Section) OnlyKeys(allowedKeys ...string) (err error) {
	extra := this.ExtraKeys(allowedKeys)
	if 0 != len(extra) {
		err = fmt.Errorf("section %s has extra keys: %v", this.Context, extra)
	}
	return
}

func (this *Section) WarnExtraKeys(allowedKeys ...string) {
	extra := this.ExtraKeys(allowedKeys)
	if 0 != len(extra) {
		ulog.Warnf("section %s has extra keys: %v", this.Context, extra)
	}
}

// iterate through config items in section, aborting if visitor returns error
func (this *Section) Each(fn func(key string, val interface{}) error) error {
	for k, v := range this.section {
		err := fn(k, v)
		if err != nil {
			return err
		}
	}
	return nil
}

// iterate through config items in section, aborting if visitor returns error
func (this *Section) EachString(fn func(key, val string) error) error {
	for k, _ := range this.section {
		v, found := this.getString(k, false)
		if found {
			err := fn(k, v)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

// compare this section to another one
func (this *Section) DiffersFrom(that *Section) (differs bool) {
	return this.Len() != that.Len() ||
		!reflect.DeepEqual(this.section, that.section)
}

// add a key/value pair to the section
func (this *Section) Add(key string, value interface{}) {
	this.section[key] = value
}

// add a substitution to the section.  the substitution will be expanded.
func (this *Section) AddSub(key, value string) {
	expanded := this.Expand(value)
	//	if _, present := this.section[key]; !present {
	//		this.section[key] = expanded
	//	}
	this.expander[key] = expanded
}

// add the substitutions to the section.  the substitutions will be expanded.
func (this *Section) AddSubs(substs map[string]string) {
	for k, v := range substs {
		this.AddSub(k, v)
	}
}

// get the current substitution map
func (this *Section) Subs() map[string]string {
	return this.expander
}

// get a copy of the current substitution map
func (this *Section) CloneSubs() map[string]string {
	return this.expander.clone()
}

// expand the text using the substitutions available in this section
func (this *Section) Expand(text string) string {
	return this.expander.expand(text)
}

// Get (using JSON conversion) the specified section into dst (a &struct).
// If key not found, dst is unmodified.
// May not be super performant, but ok for config type stuff.
func (this *Section) GetStruct(key string, dst interface{}) error {
	if it, ok := this.section[key]; ok {
		m, err := this.getMap(it)
		if !ok {
			return uerr.Chainf(err, "parsing config: value of %s", this.ctx(key))
		}
		bytes, err := json.Marshal(m)
		if err != nil {
			return err
		}
		return json.Unmarshal(bytes, dst)
	}
	return nil
}

// add any substitutions for this section in
//
func (this *Section) addSubs() (err error) {
	if 0 == len(this.expander) {
		if user, home := UserInfo(); "" != user {
			this.AddSub("user", user)
			this.AddSub("home", home)
		}
		this.AddSub("installDir", InstallD)
		this.AddSub("initDir", InitD)
		this.AddSub("host", ThisHost)
		this.AddSub("processName", ThisProcess)
	}
	var subs *Section
	err = this.GetSection(SUBS, &subs)
	if err == nil && nil != subs {
		err = subs.Each(func(k string, _ interface{}) error {
			v := ""
			err = subs.GetString(k, &v)
			if nil == err {
				this.AddSub(k, v)
			}
			return nil
		})
	}
	return
}

// if key maps to a section, set val to it
func (this *Section) GetSection(key string, val **Section,
) (err error) {
	it, ok := this.section[key]
	if ok {
		m, err := this.getMap(it)
		if err != nil {
			err = uerr.Chainf(err, "parsing config: value of %s", this.ctx(key))
		} else {
			rv := &Section{
				Context:  this.ctx(key),
				expander: this.expander.clone(),
				section:  m,
			}
			err = rv.addSubs()
			if nil == err {
				*val = rv // success!
			}
		}
	}
	return
}

// same as GetSection, but val must be non nil when done
func (this *Section) GetValidSection(key string, val **Section,
) (err error) {
	err = this.GetSection(key, val)
	if nil == err && nil == *val {
		err = fmt.Errorf("parsing config: no such section: %s", this.ctx(key))
	}
	return
}

// if key is a Array, set val to it
func (this *Section) GetArray(key string, val **Array) (err error) {
	if it, ok := this.section[key]; ok {
		raw, ok := it.([]interface{})
		if !ok {
			return fmt.Errorf("parsing config: value of %s not an array",
				this.ctx(key))
		}
		rv := &Array{
			Context:  this.ctx(key),
			expander: this.expander.clone(),
			sections: make([]map[string]interface{}, len(raw)),
		}
		for i, v := range raw {
			rv.sections[i], err = this.getMap(v)
			if err != nil {
				return uerr.Chainf(err, "parsing config: value in %s array",
					this.ctx(key))
			}
		}
		*val = rv
	}
	return
}

// Same as GetArray, but if val ends up being nil, then error
func (this *Section) GetValidArray(key string, val **Array) (err error) {
	err = this.GetArray(key, val)
	if nil == err && nil == *val {
		err = fmt.Errorf("parsing config: missing array for %s", this.ctx(key))
	}
	return
}

// change val to boolean value if found and convertible to bool
func (this *Section) GetBool(key string, val *bool) (err error) {
	it, found := this.getIt(key, false)
	if found {
		switch it.(type) {
		case bool:
			*val = it.(bool)
		case string:
			*val, err = strconv.ParseBool(it.(string))
		default:
			err = fmt.Errorf("parsing config: value of %s not convertable "+
				" to bool.  Is %s", this.ctx(key), reflect.TypeOf(it))
		}
	}
	return
}

// if value is found and a string, then set val to it.
//
// otherwise, if value is found but not a string, then error
//
// if val turns out to be empty, then error
//
// convert val to absolute path.
//
func (this *Section) GetPath(key string, val *string) (err error) {

	it, ok := this.getIt(key, false)
	if ok {
		*val, ok = it.(string)
		if !ok {
			err = fmt.Errorf("parsing config: value of %s not convertable "+
				" to path.  Is %s", this.ctx(key), reflect.TypeOf(it))
			return
		}
	}
	if 0 == len(*val) {
		err = fmt.Errorf("parsing config: key='%s' no path set", this.ctx(key))
	} else {
		*val, err = filepath.Abs(*val)
		if err != nil {
			err = uerr.Chainf(err, "parsing config: key='%s'", this.ctx(key))
		}
	}
	return
}

// same as GetPath, except also errors if unable to stat path
func (this *Section) GetValidPath(key string, val *string) (err error) {

	err = this.GetPath(key, val)
	if err != nil {
		return
	}
	_, err = os.Stat(*val)
	if err != nil {
		err = uerr.Chainf(err, "parsing config: key='%s'", this.ctx(key))
	}
	return
}

// Same as GetPath, but also ensures directory exists, creating it if necessary
func (this *Section) GetCreateDir(key string, val *string, perm os.FileMode,
) (err error) {

	if 0 == perm {
		perm = 02775
	}
	err = this.GetPath(key, val)
	if err != nil {
		return
	}
	err = os.MkdirAll(*val, perm)
	if err != nil {
		err = uerr.Chainf(err, "parsing config: key='%s'", this.ctx(key))
	}
	return
}

// if found, parse to duration and update val
func (this *Section) GetDuration(key string, val *time.Duration,
) (err error) {

	raw, found := this.getString(key, false)
	if found {
		*val, err = time.ParseDuration(raw)
		if err != nil {
			err = uerr.Chainf(err, "parsing config: %s=%s", this.ctx(key), raw)
		}
	}
	return
}

// if found, parse to float64 and update val
func (this *Section) GetFloat64(key string, val *float64) (err error) {
	it, ok := this.getIt(key, false)
	if !ok {
		return // leave val unset (default val)
	}
	*val, ok = it.(float64)
	if ok {
		// then done
	} else if raw, ok := it.(int); ok {
		*val = float64(raw)
	} else if raw, ok := it.(int64); ok {
		*val = float64(raw)
	} else if raw, ok := it.(string); ok {
		*val, err = Float64FromSiString(this.expander.expand(raw))
	} else {
		err = fmt.Errorf("parsing config: value of %s not convertable "+
			" to int64.  Is %s", this.ctx(key), reflect.TypeOf(it))
	}
	return
}

// if found, parse to int64 and update val
// handles strings with 0x (hex) or 0 (octal) prefixes
// handles strings with SI suffixes
func (this *Section) GetInt64(key string, val *int64) (err error) {
	it, ok := this.getIt(key, false)
	if !ok {
		return // leave val unset (default val)
	}
	*val, ok = it.(int64)
	if ok {
		// done
	} else if raw, ok := it.(float64); ok {
		*val = int64(raw)
	} else if raw, ok := it.(int); ok {
		*val = int64(raw)
	} else if raw, ok := it.(string); ok {
		*val, err = Int64FromSiString(this.expander.expand(raw))
	} else {
		err = fmt.Errorf("parsing config: value of %s not convertable "+
			" to int64.  Is %s", this.ctx(key), reflect.TypeOf(it))
	}
	return
}

// same as GetInt64, but error if val == invalid
func (this *Section) GetValidInt64(key string, invalid int64, val *int64,
) (err error) {

	err = this.GetInt64(key, val)
	if nil == err && invalid == *val {
		err = fmt.Errorf("Invalid value (%d) for '%s'", *val, this.ctx(key))
	}
	return
}

// if found, parse to int and update val
// handles strings with SI suffixes
func (this *Section) GetInt(key string, val *int) (err error) {
	i64 := int64(*val)
	err = this.GetInt64(key, &i64)
	if nil == err {
		*val = int(i64)
	}
	return
}

// same as GetInt, but error if val == invalid
func (this *Section) GetValidInt(key string, invalid int, val *int,
) (err error) {

	err = this.GetInt(key, val)
	if nil == err && invalid == *val {
		err = fmt.Errorf("Invalid value (%d) for '%s'", *val, this.ctx(key))
	}
	return
}

// same as GetInt, but error if val <= 0
func (this *Section) GetPosInt(key string, val *int) (err error) {

	err = this.GetInt(key, val)
	if nil == err && 0 >= *val {
		err = fmt.Errorf("Invalid value (%d) for '%s'", *val, this.ctx(key))
	}
	return
}

// if found, parse to uint64 and update val
// handles strings with 0x (hex) or 0 (octal) prefixes
// handles strings with SI suffixes
func (this *Section) GetUInt64(key string, val *uint64) (err error) {

	it, found := this.getIt(key, false)
	if !found {
		return // leave val as is
	}
	rv, ok := it.(uint64)
	if ok {
		*val = rv
	} else if raw, ok := it.(float64); ok {
		*val = uint64(raw)
	} else if raw, ok := it.(int); ok {
		*val = uint64(raw)
	} else if raw, ok := it.(string); ok {
		*val, err = UInt64FromSiString(this.expander.expand(raw))
	} else {
		err = fmt.Errorf("parsing config: value of %s not convertable "+
			" to uint64.  Is %s", this.ctx(key), reflect.TypeOf(it))
	}
	return
}

// if found, parse into []string and update val
func (this *Section) GetStrings(key string, val *[]string) (err error) {
	it, found := this.section[key]
	if found {
		*val, err = this.toStrings(key, it)
	}
	return
}

func (this *Section) toStrings(key string, it interface{},
) (rv []string, err error) {

	ok := false
	rv, ok = it.([]string)
	if ok {
		return
	}
	if raw, isArray := it.([]interface{}); isArray {
		rv = make([]string, len(raw))
		for i, v := range raw {
			str, ok := this.asString(v, false)
			if !ok {
				err = fmt.Errorf("parsing config: value in %s array not a string",
					this.ctx(key))
				break
			}
			rv[i] = this.expander.expand(str)
		}

	} else { // not an array, so attempt to create an array

		rv = make([]string, 1)
		rv[0], ok = this.asString(it, false)
		if !ok {
			err = fmt.Errorf("parsing config: %s not convertable to string array",
				this.ctx(key))
			rv = nil
		}
	}
	return
}

// if found, parse into map[string]string and update val
func (this *Section) GetStringMap(key string, val *map[string]string,
) (err error) {
	it, found := this.section[key]
	if found {
		mit, err := this.getMap(it)
		if err != nil {
			return uerr.Chainf(err, "parsing config: value of %s", this.ctx(key))
		}
		m := make(map[string]string)
		for k, v := range mit {
			str, ok := this.asString(v, false)
			if !ok {
				return fmt.Errorf("parsing config: value for %s in %s not a string",
					k, this.ctx(key))
			}
			m[k] = str
		}
		*val = m
	}
	return
}

func (this *Section) GetIt(key string, value *interface{}) {
	*value, _ = this.getIt(key, false)
	return
}

func (this *Section) GetValidIt(key string, value *interface{}) (err error) {
	found := false
	*value, found = this.getIt(key, false)
	if !found {
		err = fmt.Errorf("parsing config: did not find value for %s of %s",
			key, this.ctx(key))
	}
	return
}

//
// get the thing by key
// - if raw, then perform no expansion
// - otherwise, if the thing is a string, perform all expansions
//
func (this *Section) getIt(key string, raw bool) (rv interface{}, found bool) {
	rv, found = this.section[key]
	if found {
		if !raw {
			s, ok := rv.(string)
			if ok {
				rv = this.expander.expand(s)
			}
		}
	}
	return
}

func (this *Section) asString(it interface{}, raw bool,
) (rv string, ok bool) {

	switch it.(type) {
	case string:
		rv, ok = it.(string)
	case int, int32, int64, uint, uint32, uint64, float32, float64, bool,
		time.Duration:
		if !raw {
			rv = fmt.Sprint(it)
			ok = true
		}
	}
	return
}

func (this *Section) getString(key string, raw bool) (rv string, found bool) {
	it, gotit := this.getIt(key, raw)
	if gotit {
		rv, found = this.asString(it, raw)
	}
	return
}

// if found, parse to string and set val without detemplatizing
func (this *Section) GetRawString(key string, val *string) (err error) {
	it, gotit := this.getIt(key, true)
	if gotit {
		*val, _ = this.asString(it, true)
	}
	return
}

// if found, parse to string and set val without detemplatizing
func (this *Section) GetValidRawString(key, invalid string, val *string,
) (err error) {
	err = this.GetRawString(key, val)
	if nil == err && *val == invalid {
		err = fmt.Errorf("parsing config: invalid string value (%s) of %s",
			invalid, this.ctx(key))
	}
	return
}

// if found, parse to string and set val, resolving any templating
func (this *Section) GetString(key string, val *string) (err error) {
	it, gotit := this.getIt(key, false)
	if gotit {
		*val, _ = this.asString(it, false)
	}
	return
}

// Same as GetString, but error if val == invalid when done
func (this *Section) GetValidString(key, invalid string, val *string,
) (err error) {

	err = this.GetString(key, val)
	if nil == err && *val == invalid {
		err = fmt.Errorf("parsing config: invalid string value (%s) of %s",
			invalid, this.ctx(key))
	}
	return
}

// if found, parse to regexp and set val
func (this *Section) GetRegexp(key string, val **regexp.Regexp) (err error) {

	raw, found := this.getString(key, false)
	if found && 0 != len(raw) {
		*val, err = regexp.Compile(raw)
		if err != nil {
			err = uerr.Chainf(err, "Unable to build regexp for '%s'", this.ctx(key))
		}
	}
	return
}

// Same as GetRegexp, but error if val == nil
func (this *Section) GetValidRegexp(key string, val **regexp.Regexp) (err error) {

	err = this.GetRegexp(key, val)
	if nil == *val {
		err = fmt.Errorf("No value for '%s'", this.ctx(key))
	}
	return
}

// if found, parse to url and set val
func (this *Section) GetUrl(key string, val **nurl.URL) (err error) {
	raw, found := this.getString(key, false)
	if found && 0 != len(raw) {
		*val, err = nurl.Parse(raw)
		if err != nil {
			err = uerr.Chainf(err, "Unable to build URL for '%s'", this.ctx(key))
		}
	}
	return
}

// Same as GetUrl, but error if val == nil
func (this *Section) GetValidUrl(key string, val **nurl.URL) (err error) {

	err = this.GetUrl(key, val)
	if nil == *val {
		err = fmt.Errorf("No value for '%s'", this.ctx(key))
	}
	return
}

// get the named string, setting ok value to true if string found and set
func (this *Section) GetStringOk(key string) (string, bool) {
	return this.getString(key, false)
}

///////////////////////////////////////////////////////

//
// Array
//

func (this *Array) Len() int {
	return len(this.sections)
}

func (this *Array) Empty() bool {
	return 0 == len(this.sections)
}

// get the i'th section from this
func (this *Array) Get(i int) *Section {
	return &Section{
		Context:  this.Context + "." + strconv.Itoa(i),
		expander: this.expander.clone(),
		section:  this.sections[i],
	}
}

// iterate through the sections, aborting of visitor returns an error
func (this *Array) Each(visitor func(int, *Section) error) error {
	for i, _ := range this.sections {
		if err := visitor(i, this.Get(i)); err != nil {
			return err
		}
	}
	return nil
}

func (this *Array) DumpSubs() string {
	return this.expander.Dump()
}

///////////////////////////////////////////////////////

//
// expander
//

type expander_ map[string]string

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
func (this expander_) carefully(buff *bytes.Buffer, value string) {

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
		err = t.Execute(buff, this)
		if err != nil { // not resolved - just put in template text
			buff.Truncate(pos)
			buff.WriteString(tstring)
		}
	}
	buff.WriteString(value[pos:])
}

func (this expander_) clone() expander_ {
	rv := make(map[string]string)
	for k, v := range this {
		rv[k] = v
	}
	return rv
}

func (this expander_) Dump() (rv string) {
	return fmt.Sprintf("%#v", this)
}

///////////////////////////////////////////////////////

//
// Enable chaining of config calls
//
func (this *Section) Chain() *Chain {
	return &Chain{Section: this}
}

//
// get named subsection as a chain
//
func (this *Section) GetChain(key string) (rv *Chain) {
	rv = &Chain{}
	rv.Error = this.GetValidSection(key, &rv.Section)
	return
}