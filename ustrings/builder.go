package ustrings

import (
	"bytes"
	"strconv"
)

//
// more convenient interface for building strings
//
type StringBuilder struct {
	Buff bytes.Buffer
}

func NewStringBuilder(size int) (rv *StringBuilder) {
	rv = &StringBuilder{}
	rv.Buff.Grow(size)
	return
}

func (this *StringBuilder) Append(s string) (self *StringBuilder) {
	this.Buff.WriteString(s)
	return this
}

func (this *StringBuilder) AppendInt(i int) (self *StringBuilder) {
	this.Buff.WriteString(strconv.Itoa(i))
	return this
}

func (this *StringBuilder) AppendArray(arr ...string) (self *StringBuilder) {
	for _, s := range arr {
		this.Buff.WriteString(s)
	}
	return this
}

func (this *StringBuilder) String() (rv string) {
	return this.Buff.String()
}

func (this *StringBuilder) Reset() (self *StringBuilder) {
	this.Buff.Reset()
	return this
}

func (this *StringBuilder) Truncate(to int) (self *StringBuilder) {
	this.Buff.Truncate(to)
	return this
}

func (this *StringBuilder) Len() (rv int) {
	return this.Buff.Len()
}
