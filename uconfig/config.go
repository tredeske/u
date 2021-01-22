package uconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"math"
	nurl "net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"

	"gopkg.in/yaml.v2"
)

const (
	PROPS    = "properties"
	include_ = "include_"

	MaxUint = ^uint(0)
	MaxInt  = int(MaxUint >> 1)
	MinInt  = -MaxInt - 1
)

//
// Use with Chain.EachSection, Chain.EachSectionIf, Chain.ASection,
// Chain.IfSection, Array.Each
//
type Visitor func(*Section) error

// use with Section.GetInt to validate signed int
type IntValidator func(int64) error

// use with Section.GetUInt to validate unsigned int
type UIntValidator func(uint64) error

// use with Section.GetString to validate string
type StringValidator func(string) error

//
// A map of strings to values used for config.
//
// If the section has a key called "properties", then the keys in that sub
// section will be added as properties into the current one.
//
// As string values are accessed, the values also become properties
// for expanding later accessed string values.
//
// Expansion occurs when a string value contains ${...} or {{...}}.
//
// The ${...} will be filled in with ENV variables, if available.
//
// The {{...}} will be filled in with properties as per the
// go text/template package.  ***NOTE*** Make sure to add a '.' before
// the key.  As in: {{.key}}.  All golang text template rules apply.
//
// Child sections inherit the properties of their parents.
//
// The following properties are automatically added:
// - home 			- the home dir of the user
// - user			- the username of the user
// - host			- the hostname of the host
// - processName	- the process name of the process
// - installDir		- where the process is installed
//						{{installDir}} / bin / program     - or, if no bin -
//						{{installDir}} / program
// - initDir		- where the process is started from
//
// Other files can be included with the 'include_' directive, as in:
//
// include_:        /path/to/file.yml
//
type Section struct {
	Context  string
	expander expander_
	section  map[string]interface{}
	watch    *Watch
}

//
// create a new Section from nil, /path/to/yaml/file, YAML string,
// YAML []byte, map[string]interface{}, or map[string]string
//
func NewSection(it interface{}) (rv *Section, err error) {
	watch := &Watch{}
	tmp := Section{
		expander: newExpander(watch),
		watch:    watch,
	}
	return tmp.NewChild(it)
}

//
// create a new Section as a child of this one from nil, /path/to/yaml/file,
// YAML string, YAML []byte, map[string]interface{}, or map[string]string
//
func (this *Section) NewChild(it interface{}) (rv *Section, err error) {
	rv = &Section{
		expander: this.expander.clone(),
		watch:    this.watch,
	}
	rv.section, err = rv.getMap(it)
	if err != nil {
		return nil, err
	}
	err = rv.addProps()
	return
}

//
// watch files.  if there is a change, then call onChange.
// if there is an error and onError is set, then call it.
//
func (this *Section) Watch(
	period time.Duration,
	onChange func(changedFile string) (done bool),
	onError func(err error) (done bool),
) {
	this.watch.Start(period, onChange, onError)
}

//
// dump out the config section as a map, resolving all properties
//
func (this *Section) AsResolvedMap() (rv map[string]interface{}) {

	rv = make(map[string]interface{})
	for k, it := range this.section {
		switch v := it.(type) {
		case map[string]interface{}:
			this.resolveMap(v)
			rv[k] = v
		case []interface{}:
			this.resolveArray(v)
			rv[k] = v
		case string:
			rv[k] = this.expander.expand(v)
		default:
			rv[k] = it
		}
	}
	return
}

func (this *Section) resolveMap(m map[string]interface{}) {
	for k, it := range m {
		switch v := it.(type) {
		case map[string]interface{}:
			this.resolveMap(v)
		case []interface{}:
			this.resolveArray(v)
		case string:
			m[k] = this.expander.expand(v)
		}
	}
}

func (this *Section) resolveArray(a []interface{}) {
	for i, it := range a {
		switch v := it.(type) {
		case map[string]interface{}:
			this.resolveMap(v)
		case []interface{}:
			this.resolveArray(v)
		case string:
			a[i] = this.expander.expand(v)
		}
	}
}

//
// allow a map to be enriched by including another from file
//
func (this *Section) mapInclude(in map[string]interface{}) (err error) {

	include, found := in[include_]
	if !found {
		return
	}
	includeF, isString := include.(string)
	if !isString {
		return
	}
	var included map[string]interface{}
	err = YamlLoad(includeF, &included)
	if err != nil {
		return
	}
	this.watch.Add(includeF)

	recur := false
	for k, v := range included {
		if include_ == k {
			recur = true
		}
		_, found = in[k]
		if !found {
			in[k] = v
		}
	}
	if recur {
		err = this.mapInclude(in)
	}
	return
}

//
// coerce nil, string, []byte, or map into correct section map type
//
// if it is a nil or empty string, then this resolves to an empty map.
//
// if it is a []byte, then it is parsed as YAML.
//
// if it is a map, the map is coerced into the right type.
//
// if it is a string, it it looked up as a filename.  if no such file, then
// it is parsed as YAML.  otherwise, the file contents are parsed as YAML.
//
func (this *Section) getMap(it interface{}) (rv map[string]interface{}, err error) {

	rv, err = this.toMap(it)
	if err != nil {
		err = uerr.Chainf(err, this.Context)
		return
	} else if 0 != len(rv) {
		err = this.mapInclude(rv)
	}
	return
}

func (this *Section) toMap(it interface{}) (rv map[string]interface{}, err error) {

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
				if nil == err {
					this.watch.Add(val)
				}
			} else {
				err = yaml.Unmarshal([]byte(val), &rv)
			}
		}
	case map[interface{}]interface{}:
		rv = make(map[string]interface{}, len(val))
		for k, v := range val {
			if ks, ok := k.(string); !ok {
				err = errors.New("Non string key in map")
				break
			} else {
				rv[ks] = v
			}
		}
	case map[string]string:
		rv = make(map[string]interface{}, len(val))
		for k, v := range val {
			rv[k] = v
		}
	default:
		err = fmt.Errorf("value not a config map. is a %s",
			reflect.TypeOf(it))
	}
	return
}

func (this *Section) DumpProps() (rv string) {
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
func YamlLoad(file string, target interface{}) (err error) {
	content, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(content, target)
}

//
// read in the specified yaml file, performing properties on the text, then
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
	this.section[PROPS] = this.expander
	return yaml.Marshal(this.section)
}

// output contents to log as YAML
func (this *Section) Log() {
	content, err := this.asYaml()
	if err != nil {
		ulog.Printf("Unable to output config to log: %s", err)
	} else {
		ulog.Printf("Config:\n%s\n", content)
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
func (this *Section) EachString(fn func(key, val string) error) (err error) {
	for k, _ := range this.section {
		var v string
		var found bool
		v, found, err = this.getString(k)
		if err != nil {
			return
		}
		if found {
			err = fn(k, v)
			if err != nil {
				return
			}
		}
	}
	return
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

// add a property to the section.  the property will be expanded.
func (this *Section) AddProp(key, value string) {
	expanded := this.Expand(value)
	this.expander.Set(key, expanded)
}

// add the properties to the section.  the properties will be expanded.
func (this *Section) AddProps(props map[string]string) {
	for k, v := range props {
		this.AddProp(k, v)
	}
}

// get the property
func (this *Section) Prop(key string) string {
	return this.expander.Get(key)
}

// get the property map
func (this *Section) Props() map[string]string {
	return this.expander.mapping
}

// get a copy of the property map
func (this *Section) CloneProps() map[string]string {
	return this.expander.clone().mapping
}

// expand the text using the properties available in this section
func (this *Section) Expand(text string) string {
	return this.expander.expand(text)
}

// Get (using JSON conversion) the specified section into dst (a &struct).
// If key not found, dst is unmodified.
// May not be super performant, but ok for config type stuff.
func (this *Section) GetStruct(key string, dst interface{}) (err error) {
	it, ok := this.section[key]
	if !ok {
		return
	}
	m, err := this.getMap(it)
	if err != nil {
		return uerr.Chainf(err, "GetStruct: value of '%s'", this.ctx(key))
	}
	bytes, err := json.Marshal(m)
	if err != nil {
		return uerr.Chainf(err, "GetStruct: value of '%s'", this.ctx(key))
	}
	return json.Unmarshal(bytes, dst)
}

//
// add any properties for this section in
// - need to get this map specially as expansion rules are different
//
func (this *Section) addProps() (err error) {
	it, found := this.section[PROPS]
	if !found {
		return
	}
	var mit map[string]interface{}
	mit, err = this.toMap(it)
	if err != nil {
		return uerr.Chainf(err, "Unable to get '%s", PROPS)
	} else if 0 == len(mit) {
		return
	}
	props := make(map[string]string)
	for k, v := range mit {
		var str string
		str, err = this.asString(k, v)
		if err != nil {
			return
		}
		props[k] = str
	}
	if err != nil {
		return uerr.Chainf(err, "Unable to get '%s", PROPS)
	}
	err = this.expander.addAll(props)
	if err != nil {
		return uerr.Chainf(err, "Unable to get '%s", PROPS)
	}
	return
}

// if key maps to a sub-section, set val to it
func (this *Section) GetSectionIf(key string, val **Section) (err error) {
	it, ok := this.section[key]
	if ok {
		m, err := this.getMap(it)
		if err != nil {
			err = uerr.Chainf(err, "GetSection: value of '%s'", this.ctx(key))
		} else {
			rv := &Section{
				Context:  this.ctx(key),
				expander: this.expander.clone(),
				section:  m,
			}
			err = rv.addProps()
			if nil == err {
				*val = rv // success!
			}
		}
	}
	return
}

// Get the section or error if it does not exist or is invalid
func (this *Section) GetSection(key string, result **Section) (err error) {
	err = this.GetSectionIf(key, result)
	if nil == err && nil == *result {
		err = fmt.Errorf("parsing config: no such section: %s", this.ctx(key))
	}
	return
}

// if key is a Array, set result to it
func (this *Section) GetArrayIf(key string, result **Array) (err error) {
	it, ok := this.section[key]
	if !ok {
		return
	}
	raw, ok := it.([]interface{})
	if !ok {
		return fmt.Errorf("parsing config: value of %s not an array",
			this.ctx(key))
	}
	rv := &Array{
		Context:  this.ctx(key),
		expander: this.expander.clone(),
		sections: make([]map[string]interface{}, 0, len(raw)),
	}

	//
	// convert to maps and expand includes
	//
	isInclude := false
	var children []map[string]interface{}
	for i, v := range raw {
		var child map[string]interface{}
		child, err = this.toMap(v)
		if err != nil {
			return uerr.Chainf(err, "parsing config: value %d in %s array",
				i, this.ctx(key))
		} else if 0 == len(child) {
			continue
		}
		isInclude, err = this.arrayEntryInclude(child, &children)
		if err != nil {
			err = uerr.Chainf(err, "parsing config: value %d in %s array",
				i, this.ctx(key))
			return
		} else if !isInclude {
			children = append(children, child)
		}
	}

	//
	// convert to section maps
	//
	for _, v := range children {
		var section map[string]interface{}
		section, err = this.getMap(v)
		if err != nil {
			return uerr.Chainf(err, "parsing config: value in %s array",
				this.ctx(key))
		}
		rv.sections = append(rv.sections, section)
	}
	*result = rv
	return
}

//
// expand includes
//
// if a child only has { "include_": "path/to/file" }, then we need to
// incorporate the inclusion as additional children
//
// if a child has more than one key/value mapping, and one of them is
// "include_", then that is included in the child.  This is done elsewhere.
//
func (this *Section) arrayEntryInclude(
	entry map[string]interface{},
	addTo *[]map[string]interface{},
) (isInclude bool, err error) {

	if 1 != len(entry) {
		return
	}

	include, found := entry[include_]
	if !found {
		return
	}
	includeF, isString := include.(string)
	if !isString {
		return
	}
	isInclude = true

	includeF = this.Expand(includeF)

	var included []map[string]interface{}
	err = YamlLoad(includeF, &included)
	if err != nil {
		return
	}
	this.watch.Add(includeF)

	for _, v := range included {
		_, found = v[include_]
		if found {
			wasInclude := false
			wasInclude, err = this.arrayEntryInclude(v, addTo)
			if err != nil {
				return
			} else if !wasInclude {
				*addTo = append(*addTo, v)
			}
		} else {
			*addTo = append(*addTo, v)
		}
	}
	return
}

//
// Get the array or error if it does not exist or is invalid
//
func (this *Section) GetArray(key string, result **Array) (err error) {
	err = this.GetArrayIf(key, result)
	if nil == err && nil == *result {
		err = fmt.Errorf("parsing config: missing array for %s", this.ctx(key))
	}
	return
}

// change result to boolean value if found and convertible to bool
func (this *Section) GetBool(key string, result *bool) (err error) {
	it, found := this.getIt(key, false)
	if found {
		switch actual := it.(type) {
		case bool:
			*result = actual
		case string:
			*result, err = strconv.ParseBool(actual)
		default:
			err = fmt.Errorf("parsing config: value of %s not convertable "+
				" to bool.  Is %s", this.ctx(key), reflect.TypeOf(it))
		}
	}
	return
}

// if value is found and a string, then set result to absolute path of value.
//
// otherwise, if value is found but not a string or is blank, then error
//
func (this *Section) GetPath(key string, result *string) (err error) {

	it, ok := this.getIt(key, false)
	if ok {
		*result, ok = it.(string)
		if !ok {
			err = fmt.Errorf("parsing config: value of %s not convertable "+
				" to path.  Is %s", this.ctx(key), reflect.TypeOf(it))
			return
		}
	}
	if 0 == len(*result) {
		err = fmt.Errorf("parsing config: key='%s' no path set", this.ctx(key))
	} else {
		*result, err = filepath.Abs(*result)
		if err != nil {
			err = uerr.Chainf(err, "parsing config: key='%s'", this.ctx(key))
		}
	}
	return
}

// same as GetPath, except also errors if unable to stat path
func (this *Section) GetValidPath(key string, result *string) (err error) {

	err = this.GetPath(key, result)
	if err != nil {
		return
	}
	_, err = os.Stat(*result)
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
func (this *Section) GetDuration(key string, val *time.Duration) (err error) {

	raw, found, err := this.getString(key)
	if err != nil {
		return
	} else if found {
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

func (this *Section) validInt(
	key string,
	val int64,
	validators []IntValidator,
) (err error) {
	for _, validF := range validators {
		if nil != validF {
			err = validF(val)
			if err != nil {
				err = uerr.Chainf(err, this.ctx(key))
				return
			}
		}
	}
	return
}

//
// if found, update result with integral value
//
// result must be the address of some sort of signed int
//
// zero or more validators may be supplied.  they take the converted
// value as an int64.  this func will perform basic range checking
// based on the size of the int after validators are run
//
// handles strings with 0x (hex) or 0 (octal) prefixes
// handles strings with SI suffixes (G, M, K, Gi, Mi, Ki, ...)
//
func (this *Section) GetInt(
	key string,
	result interface{},
	validators ...IntValidator,
) (err error) {

	it, ok := this.getIt(key, false)
	if !ok {
		switch p := result.(type) { // must validate default value
		case *int:
			err = this.validInt(key, int64(*p), validators)
		case *int64:
			err = this.validInt(key, int64(*p), validators)
		case *int32:
			err = this.validInt(key, int64(*p), validators)
		case *int16:
			err = this.validInt(key, int64(*p), validators)
		case *int8:
			err = this.validInt(key, int64(*p), validators)
		}
		return // leave val unset (default val)
	}

	var val int64
	switch typed := it.(type) {
	case int:
		val = int64(typed)
	case int64:
		val = int64(typed)
	case float64:
		val = int64(typed)
	case string:
		val, err = Int64FromSiString(this.expander.expand(typed))
		if err != nil {
			err = uerr.Chainf(err, this.ctx(key))
			return
		}
	default:
		err = fmt.Errorf("parsing config: value of %s not convertable "+
			" to %s.  Is %s", this.ctx(key),
			reflect.TypeOf(result), reflect.TypeOf(it))
		return
	}

	err = this.validInt(key, val, validators)
	if err != nil {
		return
	}

	switch p := result.(type) {
	case (*int):
		if val > int64(MaxInt) {
			err = fmt.Errorf("value of %s (%d) is too large for int",
				this.ctx(key), val)
			return
		} else if val < int64(MinInt) {
			err = fmt.Errorf("value of %s (%d) is too small for int",
				this.ctx(key), val)
			return
		}
		*p = int(val)

	case *int64:
		*p = val
	case *int32:
		if val > math.MaxInt32 {
			err = fmt.Errorf("value of %s (%d) is too large for int32",
				this.ctx(key), val)
			return
		} else if val < math.MinInt32 {
			err = fmt.Errorf("value of %s (%d) is too small for int32",
				this.ctx(key), val)
			return
		}
		*p = int32(val)
	case *int16:
		if val > math.MaxInt16 {
			err = fmt.Errorf("value of %s (%d) is too large for int16",
				this.ctx(key), val)
			return
		} else if val < math.MinInt16 {
			err = fmt.Errorf("value of %s (%d) is too small for int16",
				this.ctx(key), val)
			return
		}
		*p = int16(val)
	case *int8:
		if val > math.MaxInt8 {
			err = fmt.Errorf("value of %s (%d) is too large for int8",
				this.ctx(key), val)
			return
		} else if val < math.MinInt8 {
			err = fmt.Errorf("value of %s (%d) is too small for int8",
				this.ctx(key), val)
			return
		}
		*p = int8(val)

	default:
		err = fmt.Errorf("result must be a type of signed integer.  is %s",
			reflect.TypeOf(result))
		return
	}

	return
}

func (this *Section) validUInt(
	key string,
	val uint64,
	validators []UIntValidator,
) (err error) {
	for _, validF := range validators {
		if nil != validF {
			err = validF(val)
			if err != nil {
				err = uerr.Chainf(err, this.ctx(key))
				return
			}
		}
	}
	return
}

//
// if found, update result with unsigned integral value
//
// result must be the address of some sort of unsigned int
//
// zero or more validators may be supplied.  they take the converted
// value as a uint64.  this func will perform basic range checking
// based on the size of the int after validators are run
//
// handles strings with 0x (hex) or 0 (octal) prefixes
// handles strings with SI suffixes (G, M, K, Gi, Mi, Ki, ...)
//
func (this *Section) GetUInt(
	key string,
	result interface{},
	validators ...UIntValidator,
) (err error) {

	it, ok := this.getIt(key, false)
	if !ok {
		switch p := result.(type) { // must validate default value
		case *uint:
			err = this.validUInt(key, uint64(*p), validators)
		case *uint64:
			err = this.validUInt(key, uint64(*p), validators)
		case *uint32:
			err = this.validUInt(key, uint64(*p), validators)
		case *uint16:
			err = this.validUInt(key, uint64(*p), validators)
		case *uint8:
			err = this.validUInt(key, uint64(*p), validators)
		}
		return // leave val unset (default val)
	}

	//
	// coerce the config value to a proper uint64
	//
	var val uint64
	switch typed := it.(type) {
	case uint:
		val = uint64(typed)
	case int:
		val = uint64(typed)
	case float64:
		val = uint64(typed)
	case string:
		val, err = UInt64FromSiString(this.expander.expand(typed))
		if err != nil {
			err = uerr.Chainf(err, this.ctx(key))
			return
		}
	default:
		err = fmt.Errorf("parsing config: value of %s not convertable "+
			" to %s.  Is %s", this.ctx(key),
			reflect.TypeOf(result), reflect.TypeOf(it))
		return
	}

	//
	// validate and return the value as the proper result type
	//
	err = this.validUInt(key, val, validators)
	if err != nil {
		return
	}

	switch p := result.(type) {
	case *uint:
		if val > uint64(MaxUint) {
			err = fmt.Errorf("value of %s (%d) is too large for uint",
				this.ctx(key), val)
			return
		}
		*p = uint(val)
	case *uint64:
		*p = uint64(val)
	case *uint32:
		if val > math.MaxUint32 {
			err = fmt.Errorf("value of %s (%d) is too large for uint32",
				this.ctx(key), val)
			return
		}
		*p = uint32(val)
	case *uint16:
		if val > math.MaxUint16 {
			err = fmt.Errorf("value of %s (%d) is too large for uint16",
				this.ctx(key), val)
			return
		}
		*p = uint16(val)
	case *uint8:
		if val > math.MaxUint8 {
			err = fmt.Errorf("value of %s (%d) is too large for uint8",
				this.ctx(key), val)
			return
		}
		*p = uint8(val)

	default:
		err = fmt.Errorf("result must be some type of integer.  is %s",
			reflect.TypeOf(result))
		return
	}
	return
}

// same as GetInt, but error if result == invalid
func (this *Section) GetValidInt(
	key string,
	invalid int,
	result *int,
) (err error) {

	return this.GetInt(key, result,
		func(v int64) (err error) {
			if invalid == int(v) {
				err = fmt.Errorf("int is set to invalid value: %d", invalid)
			}
			return
		})
}

// return a range validator for GetInt
func ValidRange(min, max int64) IntValidator {
	return func(v int64) (err error) {
		if v < min {
			err = fmt.Errorf("value (%d) less than min (%d)", v, min)
		} else if v > max {
			err = fmt.Errorf("value (%d) greater than max (%d)", v, max)
		}
		return
	}
}

// return a range validator for GetUInt
func ValidUIntRange(min, max uint64) UIntValidator {
	return func(v uint64) (err error) {
		if v < min {
			err = fmt.Errorf("value (%d) less than min (%d)", v, min)
		} else if v > max {
			err = fmt.Errorf("value (%d) greater than max (%d)", v, max)
		}
		return
	}
}

// validator to error if v is not positive
var MustBePos = func(v int64) (err error) {
	if v <= 0 {
		err = fmt.Errorf("int is not positive (is %d)", v)
	}
	return
}

// validator to error if v is negative
var MustBeNonNeg = func(v int64) (err error) {
	if v < 0 {
		err = fmt.Errorf("int is not positive (is %d)", v)
	}
	return
}

// same as GetInt, but error if result <= 0
func (this *Section) GetPosInt(key string, result *int) (err error) {

	return this.GetInt(key, result, MustBePos)
}

// if found, parse into []string and update val
func (this *Section) GetStrings(
	key string,
	result *[]string,
	validators ...StringValidator,
) (err error) {
	it, found := this.section[key]
	if found {
		*result, err = this.toStrings(key, it, validators)
	}
	return
}

func (this *Section) toStrings(
	key string,
	it interface{},
	validators []StringValidator,
) (rv []string, err error) {

	ok := false
	rv, ok = it.([]string)
	if !ok {
		raw, isArray := it.([]interface{})
		if isArray {
			rv = make([]string, len(raw))
			for i, v := range raw {
				var str string
				str, err = this.asString(key, v)
				if err != nil {
					rv = nil
					return
				}
				rv[i] = this.expander.expand(str)
			}

		} else { // not an array, so attempt to create an array

			var str string
			str, err = this.asString(key, it)
			if err != nil {
				return
			}
			rv = make([]string, 1)
			rv[0] = this.expander.expand(str)
		}
	}
	if 0 != len(validators) {
		for _, s := range rv {
			err = this.validString(key, s, validators)
			if err != nil {
				return
			}
		}
	}
	return
}

// if found, parse into map[string]string and update val
func (this *Section) GetStringMap(key string, val *map[string]string) (err error) {
	it, found := this.section[key]
	if found {
		var mit map[string]interface{}
		mit, err = this.getMap(it)
		if err != nil {
			return uerr.Chainf(err, "GetStringMap: value of '%s'", this.ctx(key))
		}
		*val = make(map[string]string, len(mit))
		for k, v := range mit {
			var str string
			str, err = this.asString(k, v)
			if err != nil {
				return
			}
			(*val)[this.expander.expand(k)] = this.expander.expand(str)
		}
		if err != nil {
			err = uerr.Chainf(err, "at %s", this.ctx(key))
			return
		}
	}
	return
}

func (this *Section) GetIt(key string, value *interface{}) {
	*value, _ = this.getIt(key, false)
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

// convert it to string
func (this *Section) asString(key string, it interface{}) (rv string, err error) {

	switch typ := it.(type) {
	case string:
		rv = typ
	case int, int16, int32, int64, uint, uint16, uint32, uint64,
		float32, float64, bool, time.Duration:
		rv = fmt.Sprint(it)
	default:
		err = fmt.Errorf("Unable to convert value of %s to string: %#v",
			this.ctx(key), it)
	}
	return
}

// convert it to string
// do NOT convert numeric, boolean, or other types to string
func asRawString(it interface{}) (rv string, ok bool) {

	switch typ := it.(type) {
	case string:
		rv = typ
		ok = true
	}
	return
}

func (this *Section) getString(key string) (rv string, found bool, err error) {
	var it interface{}
	it, found = this.getIt(key, false)
	if found {
		rv, err = this.asString(key, it)
	}
	return
}

func (this *Section) validString(
	key, val string,
	validators []StringValidator,
) (err error) {
	for _, validF := range validators {
		if nil != validF {
			err = validF(val)
			if err != nil {
				err = uerr.Chainf(err, this.ctx(key))
				return
			}
		}
	}
	return
}

// if found, parse to string and set result without detemplatizing
func (this *Section) GetRawString(
	key string,
	result *string,
	validators ...StringValidator,
) (err error) {

	it, gotit := this.getIt(key, true)
	if gotit {
		val, _ := asRawString(it)
		err = this.validString(key, val, validators)
		if nil == err {
			*result = val
		}
	}
	return
}

// if found, parse to string and set result, resolving any templating
func (this *Section) GetString(
	key string,
	result *string,
	validators ...StringValidator,
) (err error) {

	var val string
	var found bool
	val, found, err = this.getString(key)
	if err != nil {
		return
	}
	if !found && nil != result { // validate default value
		val = *result
	}
	err = this.validString(key, val, validators)
	if err != nil {
		return
	}
	if found {
		*result = val
	}
	return
}

// a StringValidator to verify string not blank
func StringNotBlank(v string) (err error) {
	if 0 == len(v) {
		err = errors.New("String value empty")
	}
	return
}

// create a StringValidator to verify value is blank or valid
func StringBlankOr(validator StringValidator) StringValidator {
	return func(v string) (err error) {
		if 0 != len(v) {
			err = validator(v)
		}
		return
	}
}

// create a StringValidator to verify value is one of listed
func StringOneOf(choices ...string) StringValidator {
	return func(v string) (err error) {
		for _, choice := range choices {
			if choice == v {
				return
			}
		}
		return fmt.Errorf("String (%s) not in %#v", v, choices)
	}
}

// create a StringValidator to verify value not one of listed
func StringNot(invalid ...string) StringValidator {
	return func(v string) (err error) {
		for _, iv := range invalid {
			if iv == v {
				return fmt.Errorf("String cannot be (%s)", v)
			}
		}
		return
	}
}

// if found and not blank, parse to regexp and set result
func (this *Section) GetRegexpIf(key string, result **regexp.Regexp) (err error) {

	raw, found, err := this.getString(key)
	if err != nil {
		return
	}
	if found && 0 != len(raw) {
		*result, err = regexp.Compile(raw)
		if err != nil {
			err = uerr.Chainf(err, "Unable to build regexp for '%s'", this.ctx(key))
		}
	}
	return
}

// get value as regexp
func (this *Section) GetRegexp(key string, result **regexp.Regexp) (err error) {

	err = this.GetRegexpIf(key, result)
	if nil == err && nil == *result {
		err = fmt.Errorf("No regexp value for '%s'", this.ctx(key))
	}
	return
}

// if found and not blank, parse to url and set result
func (this *Section) GetUrlIf(key string, result **nurl.URL) (err error) {
	raw, found, err := this.getString(key)
	if err != nil {
		return
	}
	if found && 0 != len(raw) {
		*result, err = nurl.Parse(raw)
		if err != nil {
			err = uerr.Chainf(err, "Unable to build URL for '%s'", this.ctx(key))
		}
	}
	return
}

// get and parse the url, setting result
func (this *Section) GetUrl(key string, result **nurl.URL) (err error) {

	err = this.GetUrlIf(key, result)
	if nil == err && nil == *result {
		err = fmt.Errorf("No URL value for '%s'", this.ctx(key))
	}
	return
}

///////////////////////////////////////////////////////

//
// Enable chaining of config calls
//
func (this *Section) Chain() *Chain {
	if nil == this {
		panic("chaining off of nil section")
	}
	return &Chain{Section: this}
}

//
// get named subsection as a chain
//
func (this *Section) GetChain(key string) (rv *Chain) {
	rv = &Chain{}
	rv.Error = this.GetSection(key, &rv.Section)
	return
}
