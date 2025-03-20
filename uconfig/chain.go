package uconfig

import (
	"fmt"
	"math"
	nurl "net/url"
	"os"
	"regexp"
	"time"

	"github.com/tredeske/u/uerr"
)

// Chain provides a nicer means of accessing a config Section.
//
// Chain allows chaining of accessor functions to avoid error checking
// boilerplate.
//
// The accessor functions enable accessing the config settings and sub-sections.
//
// The accessors also perform sensible type coercion.  For example, if a setting
// is a string, but it is being accessed as an int, then it will be converted
// to an int if possible.
//
// Validation funcs are also provided, or you can make your own.
//
// A Section is usually provided to you in the golum lifecycle methods for
// creating or reloading your component.  It is simple to convert it to a Chain.
//
//	var s *uconfig.Section
//	var foo, bar, baz int
//
//	err = s.Chain().
//	    GetInt("foo", &foo).
//	    GetInt("bar", &bar, uconfig.MustBePos).
//	    GetInt("baz", &baz, uconfig.ValidRange(5, 20)).
//	    Each("array",
//	        func(c *Chain) (err error) {
//	            return c.
//	                ...
//	                Done()
//	        }
//	    Done()
type Chain struct {
	Section *Section
	Error   error
}

// open file and read config from it
func FromFile(f string) *Chain {
	s, err := NewSection(f)
	if err != nil {
		return &Chain{Error: err}
	}
	return s.Chain()
}

// Builder works with Chain.Build, Chain.BuildIf, Chain.BuildFrom to
// build rv from config.
type Builder func(config *Chain) (rv any, err error)

// ChainVisitor works with Chain.If, Chain.Must, Chain.Each.
type ChainVisitor func(config *Chain) (err error)

// Constructable works with Chain.Construct, Chain.ConstructFrom,
// Chain.ConstructIf for things that construct themselves from config.
type Constructable interface {
	FromConfig(config *Chain) (err error)
}

func (this *Chain) DumpProps() string {
	return this.Section.DumpProps()
}

func (this *Chain) ctx(key string) string {
	return this.Section.ctx(key)
}

// prefer Each()
func (this *Chain) GetArray(key string, value **Array) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetArray(key, value)
	}
	return this
}

// prefer EachIf()
func (this *Chain) GetArrayIf(key string, value **Array) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetArrayIf(key, value)
	}
	return this
}

/*
// deprecated - use Each().
//
// get the array specified by key and iterate through the contained sections
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

// deprecated - use EachIf().
//
// get the array specified by key if it exists and iterate through the sections.
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

// deprecated - use Must().
//
// get the sub-section specified by key and process it
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

// deprecated - use If().
//
// if the sub-section exists, process it
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
*/

// deprecated
func (this *Chain) GetSection(key string, value **Section) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetSection(key, value)
	}
	return this
}

/*
// deprecated
func (this *Chain) GetSectionIf(key string, value **Section) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetSectionIf(key, value)
	}
	return this
}
*/

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

func (this *Chain) GetMillis(key string, value *int64, validators ...IntValidator,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetMillis(key, value, validators...)
	}
	return this
}

func (this *Chain) GetFloat64(
	key string,
	value *float64,
	validators ...FloatValidator,
) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetFloat64(key, value, validators...)
	}
	return this
}

func (this *Chain) GetFloat32(
	key string,
	value *float32,
	validators ...FloatValidator,
) *Chain {
	var f64 float64
	if nil == this.Error {
		this.Error = this.Section.GetFloat64(key, &f64, validators...)
	}
	if nil == this.Error {
		if math.MaxFloat32 < f64 {
			this.Error = fmt.Errorf("value of %s too large for float32",
				this.Section.ctx(key))

		} else {
			*value = float32(f64)
		}
	}
	return this
}

func (this *Chain) GetBitRate(
	key string,
	result any,
	validators ...IntValidator,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetBitRate(key, result, validators...)
	}
	return this
}

func (this *Chain) GetByteSize(
	key string,
	result any,
	validators ...IntValidator,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetByteSize(key, result, validators...)
	}
	return this
}

func (this *Chain) GetInt(
	key string,
	result any,
	validators ...IntValidator,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetInt(key, result, validators...)
	}
	return this
}

func (this *Chain) GetInts(
	key string,
	result *[]int,
	validators ...IntValidator,
) *Chain {

	if nil == this.Error {
		this.Error = this.Section.GetInts(key, result, validators...)
	}
	return this
}

func (this *Chain) GetUInt(
	key string,
	result any,
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

func (this *Chain) GetUrl(
	key string,
	value **nurl.URL,
	validators ...StringValidator,
) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetUrl(key, value, validators...)
	}
	return this
}

func (this *Chain) GetUrlIf(
	key string,
	value **nurl.URL,
	validators ...StringValidator,
) *Chain {
	if nil == this.Error {
		this.Error = this.Section.GetUrlIf(key, value, validators...)
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
func (this *Chain) GetIt(key string, value *any) *Chain {
	if nil == this.Error {
		this.Section.GetIt(key, value)
	}
	return this
}

// if key resolves to some value, then set value, otherwise error
func (this *Chain) GetValidIt(key string, value *any) *Chain {
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

func (this *Chain) FailExtraKeys(allowedKeys ...string) *Chain {
	if nil == this.Error {
		this.Section.FailExtraKeys(allowedKeys...)
	}
	return this
}

// end the accessor chain, detecting invalid config, returning active error (if any)
func (this *Chain) Done() error {
	if nil == this.Error {
		this.Error = this.Section.OnlyKeys()
	}
	return this.Error
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

// if the section exists, add elements of it as properties to this section
func (this *Chain) AddPropsIf(key string) *Chain {
	if nil == this.Error {
		var props map[string]string
		this.Error = this.Section.GetStringMap(key, &props)
		if nil == this.Error && 0 != len(props) {
			this.Section.AddProps(props)
		}
	}
	return this
}

// run builder with specified sub-section if section exists
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

// run builder with specified sub-section if section exists, fail otherwise
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

// if any of the keys are present, call handler
func (this *Chain) IfHasKeysIn(f ChainVisitor, keys ...string) *Chain {
	if nil == this.Error && this.Section.AnyKeysIn(keys...) {
		this.Error = f(this)
	}
	return this
}

// if any of the keys are present, call handler
func (this *Chain) IfHasKeysMatching(f ChainVisitor, r *regexp.Regexp) *Chain {
	if nil == this.Error && this.Section.AnyKeysMatch(r) {
		this.Error = f(this)
	}
	return this
}

// run builder against each sub section in named array
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

// run builder against each sub section in named array, if array exists
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

// build value from named config section if it exists
//
// if builder returns nil, then no assignment is made to value
//
// value is typically a pointer to the thing that will be built.  If the
// thing to be built is a pointer, then it must be the addres of the pointer.
func (this *Chain) BuildIf(key string, value any, builder Builder) *Chain {

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

// build value from named config section
//
// if builder returns nil, then no assignment is made to value
//
// value is typically a pointer to the thing that will be built.  If the
// thing to be built is a pointer, then it must be the addres of the pointer.
func (this *Chain) BuildFrom(
	key string,
	value any,
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

// build value from current config section using builder
//
// if builder returns nil, then no assignment is made to value
//
// value is typically a pointer to the thing that will be built.  If the
// thing to be built is a pointer, then it must be the addres of the pointer.
func (this *Chain) Build(value any, builder Builder) *Chain {

	if nil == this.Error {
		var it any
		it, this.Error = builder(this)
		if nil == this.Error && nil != it {
			this.Error = Assign(this.Section.Context, value, it)
		}
	}
	return this
}

// Construct target from current config section.
func (this *Chain) Construct(target Constructable) *Chain {

	if nil == this.Error {
		this.Error = target.FromConfig(this)
	}
	return this
}

// Construct target from named config sub-section.
func (this *Chain) ConstructFrom(key string, target Constructable) *Chain {

	if nil == this.Error {
		var chain *Chain
		this.GetChain(key, &chain)
		chain.Construct(target)
		this.Error = chain.Error
	}
	return this
}

// Construct target from named config sub-section if sub-section exists.
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
