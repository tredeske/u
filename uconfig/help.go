package uconfig

import (
	"github.com/tredeske/u/ulog"

	"gopkg.in/yaml.v2"
)

// accumulates ordered help info - see golum.Show
type Help yaml.MapSlice

// Initialize the help info, returning a Help to use for parameters
func (this *Help) Init(name, note string) (rv *Help) {
	params := &Help{}
	*this = append(*this,
		yaml.MapItem{Key: name, Value: note},
		yaml.MapItem{Key: "params", Value: params})
	return params
}

// produce the help contents in YAML format
func (this *Help) AsYaml() (content []byte, err error) {
	return yaml.Marshal(this)
}

// Add an item to the help info
func (this *Help) NewItem(name, theType, note string) (rv *Help) {
	rv = &Help{
		{Key: "type", Value: theType},
		{Key: "note", Value: note},
	}
	*this = append(*this, yaml.MapItem{Key: name, Value: rv})
	return
}

// Mark this as optional
func (this *Help) SetOptional() (rv *Help) { return this.Optional() }

// Mark this item as optional
func (this *Help) Optional() (rv *Help) {
	*this = append(*this, yaml.MapItem{Key: "optional", Value: true})
	return this
}

// Set default value for this
func (this *Help) SetDefault(value any) (rv *Help) { return this.Default(value) }

// Set default value for this
func (this *Help) Default(value any) (rv *Help) {
	*this = append(*this, yaml.MapItem{Key: "default", Value: value})
	return this
}

// Set something on the current help
func (this *Help) Set(key string, value any) (rv *Help) {
	*this = append(*this, yaml.MapItem{Key: key, Value: value})
	return this
}

// Start a sub section for this
func (this *Help) AddSub(title string) (sub *Help) {
	if 0 == len(title) {
		title = "sub"
	}
	sub = &Help{}
	*this = append(*this, yaml.MapItem{Key: title, Value: sub})
	return sub
}

func (this *Help) Contains(key string) bool {
	for _, item := range *this {
		if item.Key == key {
			return true
		}
	}
	return false
}

func (this *Help) Get(key string) (rv any) {
	for _, item := range *this {
		if item.Key == key {
			return item.Value
		}
	}
	return nil
}

func (this *Help) GetHelp(key string) (rv *Help) {
	it := this.Get(key)
	if nil != it {
		rv, _ = it.(*Help)
	}
	return
}

// issue warnings for any undocumented parameters
func (this *Section) WarnUnknown(h *Help) {
	params := h.GetHelp("params")
	if nil == params {
		ulog.Warnf("Unable to get 'params' for %s", this.Context)
		return
	}

	for k, _ := range this.section {
		if !params.Contains(k) {
			ulog.Warnf("Unknown parameter (%s) found in %s", k, this.Context)
		}
	}
}
