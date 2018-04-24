package uconfig

import (
	"fmt"
	nurl "net/url"
	"os"
	"regexp"
	"time"

	"github.com/tredeske/u/uerr"
)

//
// Enable chaining of config calls
//
type Chain struct {
	Section *Section
	Error   error
}

func (this *Chain) DumpSubs() string {
	return this.Section.DumpSubs()
}

func (this *Chain) ctx(key string) string {
	return this.Section.ctx(key)
}

func (this *Chain) GetArray(key string, value **Array) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetArray(key, value)
	}
	return this
}

func (this *Chain) GetValidArray(key string, value **Array) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetValidArray(key, value)
	}
	return this
}

//
// get the array specified by key and iterate through the contained sections
//
func (this *Chain) EachSection(
	key string,
	visitor func(int, *Section) error,
) *Chain {
	if nil == this.Error {
		var arr *Array
		this.Error = this.Section.GetValidArray(key, &arr)
		if nil == this.Error {
			this.Error = arr.Each(visitor)
		}
	}
	return this
}

// get the array specified by key if it exists and
// iterate through the contained sections
//
func (this *Chain) EachSectionIf(
	key string,
	visitor func(int, *Section) error,
) *Chain {
	if nil == this.Error {
		var arr *Array
		this.Error = this.Section.GetArray(key, &arr)
		if nil == this.Error && nil != arr {
			this.Error = arr.Each(visitor)
		}
	}
	return this
}

//
// get the section specified by key and process it
//
func (this *Chain) ASection(
	key string,
	visitor func(*Section) error,
) *Chain {
	if nil == this.Error {
		var s *Section
		this.Error = this.Section.GetValidSection(key, &s)
		if nil == this.Error {
			this.Error = visitor(s)
		}
	}
	return this
}

//
// if the section exists, process it
//
func (this *Chain) IfSection(
	key string,
	visitor func(*Section) error,
) *Chain {
	if nil == this.Error {
		var s *Section
		this.Error = this.Section.GetSection(key, &s)
		if nil == this.Error && nil != s {
			this.Error = visitor(s)
		}
	}
	return this
}

// if key maps to section, set value to section
func (this *Chain) GetSection(key string, value **Section) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetSection(key, value)
	}
	return this
}

// same as GetSection, but err is set if section is nil
func (this *Chain) GetValidSection(key string, value **Section) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetValidSection(key, value)
	}
	return this
}

func (this *Chain) GetValidChain(key string, value **Chain) *Chain {
	if nil == this.Error {
		*value = this.Section.GetChain(key)
		this.Error = (*value).Error
		if nil == this.Error && nil == *value {
			this.Error = fmt.Errorf("Missing '%s' section", this.ctx(key))
		}
	}
	return this
}

func (this *Chain) GetBool(key string, value *bool) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetBool(key, value)
	}
	return this
}

func (this *Chain) GetDuration(key string, value *time.Duration) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetDuration(key, value)
	}
	return this
}

func (this *Chain) GetFloat64(key string, value *float64) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetFloat64(key, value)
	}
	return this
}

func (this *Chain) GetInt64(key string, value *int64) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetInt64(key, value)
	}
	return this
}

func (this *Chain) GetValidInt64(key string, invalid int64, value *int64,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetValidInt64(key, invalid, value)
	}
	return this
}

func (this *Chain) GetInt(key string, value *int) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetInt(key, value)
	}
	return this
}

func (this *Chain) GetValidInt(key string, invalid int, value *int,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetValidInt(key, invalid, value)
	}
	return this
}

func (this *Chain) GetPosInt(key string, value *int,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetPosInt(key, value)
	}
	return this
}

func (this *Chain) GetCreateDir(key string, value *string, perm os.FileMode,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetCreateDir(key, value, perm)
	}
	return this
}

func (this *Chain) GetRegexp(key string, value **regexp.Regexp,
) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetRegexp(key, value)
	}
	return this
}

func (this *Chain) GetValidRegexp(key string, value **regexp.Regexp,
) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetValidRegexp(key, value)
	}
	return this
}

func (this *Chain) GetUrl(key string, value **nurl.URL) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetUrl(key, value)
	}
	return this
}

func (this *Chain) GetValidUrl(key string, value **nurl.URL) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetValidUrl(key, value)
	}
	return this
}

func (this *Chain) GetPath(key string, value *string) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetPath(key, value)
	}
	return this
}

func (this *Chain) GetValidPath(key string, value *string) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetValidPath(key, value)
	}
	return this
}

// if key resolves to some value, then set value
func (this *Chain) GetIt(key string, value *interface{}) *Chain {
	if nil == this.Error {
		this.Section.GetIt(key, value)
	}
	return this
}

// if key resolves to some value, then set value, otherwise error
func (this *Chain) GetValidIt(key string, value *interface{}) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetValidIt(key, value)
	}
	return this
}

// if key resolves to string, then set value
func (this *Chain) GetString(key string, value *string) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetString(key, value)
	}
	return this
}

// if key resolves to []string, then set value
func (this *Chain) GetStrings(key string, value *[]string) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetStrings(key, value)
	}
	return this
}

// if key resolves to map[string]string, then set value
func (this *Chain) GetStringMap(key string, value *map[string]string,
) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetStringMap(key, value)
	}
	return this
}

// get raw string value with no detemplating or translation
func (this *Chain) GetRawString(key string, value *string) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetRawString(key, value)
	}
	return this
}

// get raw string value with no detemplating or translation
// set error if value is same as invalid value
func (this *Chain) GetValidRawString(key, invalid string, value *string,
) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetValidRawString(key, invalid, value)
	}
	return this
}

// get string into value.  default value is value in value before call
// set error if value is same as invalid value
func (this *Chain) GetValidString(key, invalid string, value *string,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetValidString(key, invalid, value)
	}
	return this
}

func (this *Chain) OnlyKeys(allowedKeys ...string) *Chain {
	if nil == this.Error {
		this.Error = this.Section.OnlyKeys(allowedKeys...)
	}
	return this
}

func (this *Chain) WarnExtraKeys(allowedKeys ...string) *Chain {
	if nil == this.Error {
		this.Section.WarnExtraKeys(allowedKeys...)
	}
	return this
}

// run the specified checking function as part of the chain
func (this *Chain) Check(fn func() (err error)) *Chain {
	if nil == this.Error {
		this.Error = fn()
	}
	return this
}

// run the specified function as part of the chain if no preceding error
func (this *Chain) Then(fn func()) *Chain {
	if nil == this.Error {
		fn()
	}
	return this
}

//
// run func with specified section if section exists
//
func (this *Chain) If(
	key string,
	builder func(config *Chain) (err error),
) *Chain {

	if nil == this.Error {
		var s *Section
		this.Error = this.Section.GetSection(key, &s)
		if nil == this.Error && nil != s {
			chain := s.Chain()
			err := builder(chain)
			if err != nil {
				this.Error = uerr.Chainf(err, "Unable to build '%s'", this.ctx(key))
			}
		}
	}
	return this
}

//
// run func with specified section if section exists, fail otherwise
//
func (this *Chain) Must(
	key string,
	builder func(config *Chain) (err error),
) *Chain {

	if nil == this.Error {
		var chain *Chain
		this.GetValidChain(key, &chain)
		if nil == this.Error {
			err := builder(chain)
			if err != nil {
				this.Error = uerr.Chainf(err, "Unable to build '%s'", this.ctx(key))
			}
		}
	}
	return this
}

// build value from named config section if it exists
func (this *Chain) BuildIf(
	key string,
	value interface{},
	builder func(config *Chain) (rv interface{}, err error),
) *Chain {

	if nil == this.Error {
		var s *Section
		this.Error = this.Section.GetSection(key, &s)
		if nil == this.Error && nil != s {
			chain := s.Chain()
			chain.Build(value, builder)
			this.Error = chain.Error
		}
	}
	return this
}

// build value from named config section
func (this *Chain) BuildFrom(
	key string,
	value interface{},
	builder func(config *Chain) (rv interface{}, err error),
) *Chain {

	if nil == this.Error {
		var chain *Chain
		this.GetValidChain(key, &chain)
		chain.Build(value, builder)
		this.Error = chain.Error
	}
	return this
}

//
// build value from current config section using builder
//
// if builder returns nil, then no assignment is made to value
//
func (this *Chain) Build(
	value interface{},
	builder func(config *Chain) (rv interface{}, err error),
) *Chain {

	if nil == this.Error {
		var it interface{}
		it, this.Error = builder(this)
		if nil == this.Error && nil != it {
			this.Error = Assign(this.Section.Context, value, it)
		}
	}
	return this
}
