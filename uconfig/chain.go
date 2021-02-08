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

//
// Use with Chain.Build, Chain.BuildIf, Chain.BuildFrom
//
type Builder func(config *Chain) (rv interface{}, err error)

//
// Use with Chain.If, Chain.Must Chain.Each
//
type ChainVisitor func(config *Chain) (err error)

//
// Use with Chain.Construct, Chain.ConstructFrom, Chain.ConstructIf
// for things that construct themselves from config
//
type Constructable interface {
	FromConfig(config *Chain) (err error)
}

func (this *Chain) DumpProps() string {
	return this.Section.DumpProps()
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

func (this *Chain) GetArrayIf(key string, value **Array) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetArrayIf(key, value)
	}
	return this
}

//
// get the array specified by key and iterate through the contained sections
//
func (this *Chain) EachSection(key string, visitor Visitor) *Chain {
	if nil == this.Error {
		var arr *Array
		this.Error = this.Section.GetArray(key, &arr)
		if nil == this.Error {
			this.Error = arr.Each(visitor)
		}
	}
	return this
}

// get the array specified by key if it exists and
// iterate through the contained sections
//
func (this *Chain) EachSectionIf(key string, visitor Visitor) *Chain {
	if nil == this.Error {
		var arr *Array
		this.Error = this.Section.GetArrayIf(key, &arr)
		if nil == this.Error && nil != arr {
			this.Error = arr.Each(visitor)
		}
	}
	return this
}

//
// get the sub-section specified by key and process it
//
func (this *Chain) ASection(key string, visitor Visitor) *Chain {
	if nil == this.Error {
		var s *Section
		this.Error = this.Section.GetSection(key, &s)
		if nil == this.Error {
			this.Error = visitor(s)
		}
	}
	return this
}

//
// if the sub-section exists, process it
//
func (this *Chain) IfSection(key string, visitor Visitor) *Chain {
	if nil == this.Error {
		var s *Section
		this.Error = this.Section.GetSectionIf(key, &s)
		if nil == this.Error && nil != s {
			this.Error = visitor(s)
		}
	}
	return this
}

func (this *Chain) GetSection(key string, value **Section) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetSection(key, value)
	}
	return this
}

func (this *Chain) GetSectionIf(key string, value **Section) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetSectionIf(key, value)
	}
	return this
}

func (this *Chain) GetChain(key string, value **Chain) *Chain {
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

func (this *Chain) GetInt(
	key string,
	result interface{},
	validators ...IntValidator,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetInt(key, result, validators...)
	}
	return this
}

func (this *Chain) GetUInt(
	key string,
	result interface{},
	validators ...UIntValidator,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetUInt(key, result, validators...)
	}
	return this
}

func (this *Chain) GetValidInt(key string, invalid int, value *int) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetValidInt(key, invalid, value)
	}
	return this
}

func (this *Chain) GetPosInt(key string, value *int) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetPosInt(key, value)
	}
	return this
}

func (this *Chain) GetCreateDir(
	key string,
	value *string,
	perm os.FileMode,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetCreateDir(key, value, perm)
	}
	return this
}

func (this *Chain) GetRegexp(key string, value **regexp.Regexp) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetRegexp(key, value)
	}
	return this
}

func (this *Chain) GetRegexpIf(key string, value **regexp.Regexp) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetRegexpIf(key, value)
	}
	return this
}

func (this *Chain) GetUrl(key string, value **nurl.URL) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetUrl(key, value)
	}
	return this
}

func (this *Chain) GetUrlIf(key string, value **nurl.URL) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetUrlIf(key, value)
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

// if key resolves to string, then set result
func (this *Chain) GetString(
	key string,
	result *string,
	validators ...StringValidator,
) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetString(key, result, validators...)
	}
	return this
}

// if key resolves to []string, then set result
func (this *Chain) GetStrings(
	key string,
	result *[]string,
	validators ...StringValidator,
) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetStrings(key, result, validators...)
	}
	return this
}

// if key resolves to map[string]string, then set value
func (this *Chain) GetStringMap(key string, value *map[string]string) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetStringMap(key, value)
	}
	return this
}

// get raw string value with no detemplating or translation
func (this *Chain) GetRawString(
	key string,
	result *string,
	validators ...StringValidator,
) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetRawString(key, result, validators...)
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
func (this *Chain) ThenCheck(fn func() (err error)) *Chain {
	if nil == this.Error {
		this.Error = fn()
	}
	return this
}

// run the specified function as part of the chain
func (this *Chain) Then(fn func()) *Chain {
	if nil == this.Error {
		fn()
	}
	return this
}

//
// run builder with specified sub-section if section exists
//
func (this *Chain) If(key string, builder ChainVisitor) *Chain {

	if nil == this.Error {
		var s *Section
		this.Error = this.Section.GetSectionIf(key, &s)
		if nil == this.Error && nil != s {
			err := builder(s.Chain())
			if err != nil {
				this.Error = uerr.Chainf(err, "Unable to build '%s'", this.ctx(key))
			}
		}
	}
	return this
}

//
// run builder with specified sub-section if section exists, fail otherwise
//
func (this *Chain) Must(key string, builder ChainVisitor) *Chain {

	if nil == this.Error {
		var chain *Chain
		this.GetChain(key, &chain)
		if nil == this.Error {
			err := builder(chain)
			if err != nil {
				this.Error = uerr.Chainf(err, "Unable to build '%s'", this.ctx(key))
			}
		}
	}
	return this
}

//
// run builder against each sub section in named array
//
func (this *Chain) Each(key string, builder ChainVisitor) *Chain {

	if nil == this.Error {
		var arr *Array
		this.Error = this.Section.GetArray(key, &arr)
		if nil == this.Error {
			this.Error = arr.Each(
				func(s *Section) error {
					return builder(s.Chain())
				})
		}
	}
	return this
}

//
// run builder against each sub section in named array, if array exists
//
func (this *Chain) EachIf(key string, builder ChainVisitor) *Chain {

	if nil == this.Error {
		var arr *Array
		this.Error = this.Section.GetArrayIf(key, &arr)
		if nil == this.Error && nil != arr {
			this.Error = arr.Each(
				func(s *Section) error {
					return builder(s.Chain())
				})
		}
	}
	return this
}

//
// build value from named config section if it exists
//
// if builder returns nil, then no assignment is made to value
//
// value is typically a pointer to the thing that will be built.  If the
// thing to be built is a pointer, then it must be the addres of the pointer.
//
func (this *Chain) BuildIf(key string, value interface{}, builder Builder) *Chain {

	if nil == this.Error {
		var s *Section
		this.Error = this.Section.GetSectionIf(key, &s)
		if nil == this.Error && nil != s {
			chain := s.Chain()
			chain.Build(value, builder)
			this.Error = chain.Error
		}
	}
	return this
}

//
// build value from named config section
//
// if builder returns nil, then no assignment is made to value
//
// value is typically a pointer to the thing that will be built.  If the
// thing to be built is a pointer, then it must be the addres of the pointer.
//
func (this *Chain) BuildFrom(
	key string,
	value interface{},
	builder Builder,
) *Chain {

	if nil == this.Error {
		var chain *Chain
		this.GetChain(key, &chain)
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
// value is typically a pointer to the thing that will be built.  If the
// thing to be built is a pointer, then it must be the addres of the pointer.
//
func (this *Chain) Build(value interface{}, builder Builder) *Chain {

	if nil == this.Error {
		var it interface{}
		it, this.Error = builder(this)
		if nil == this.Error && nil != it {
			this.Error = Assign(this.Section.Context, value, it)
		}
	}
	return this
}

//
// Construct target from current config section.
//
func (this *Chain) Construct(target Constructable) *Chain {

	if nil == this.Error {
		this.Error = target.FromConfig(this)
	}
	return this
}

//
// Construct target from named config sub-section.
//
func (this *Chain) ConstructFrom(key string, target Constructable) *Chain {

	if nil == this.Error {
		var chain *Chain
		this.GetChain(key, &chain)
		chain.Construct(target)
		this.Error = chain.Error
	}
	return this
}

//
// Construct target from named config sub-section if sub-section exists.
//
func (this *Chain) ConstructIf(key string, target Constructable) *Chain {

	if nil == this.Error {
		var s *Section
		this.Error = this.Section.GetSectionIf(key, &s)
		if nil == this.Error && nil != s {
			chain := s.Chain()
			chain.Construct(target)
			this.Error = chain.Error
		}
	}
	return this
}
