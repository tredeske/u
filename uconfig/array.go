package uconfig

import (
	"strconv"

	"github.com/tredeske/u/uerr"
)

// an array of config sections
type Array struct {
	Context  string
	expander expander_
	sections []map[string]any
}

func ArrayFromSection(s *Section) (rv *Array) {
	return &Array{
		Context:  s.Context,
		expander: s.expander,
		sections: []map[string]any{s.section},
	}
}

func (this *Array) Append(s *Section) {
	this.sections = append(this.sections, s.section)
}

func (this *Array) Len() int {
	if nil == this {
		return 0
	}
	return len(this.sections)
}

func (this *Array) Empty() bool {
	return nil == this || 0 == len(this.sections)
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
func (this *Array) Each(visitor Visitor) (err error) {
	if nil != this {
		for i, _ := range this.sections {
			err = visitor(this.Get(i))
			if err != nil {
				err = uerr.Chainf(err, "section %d", i)
				break
			}
		}
	}
	return
}

func (this *Array) DumpSubs() string {
	return this.expander.Dump()
}
