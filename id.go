package u

import (
	"crypto/rand"
	"encoding/binary"
	"sync/atomic"
	"time"
)

//
// a simple, fast 64 bit unique ID generator
//
type IdBuilder struct {
	counter uint64
}

//
// create an IdBuilder
//
func NewIdBuilder() (rv IdBuilder) {
	arr := [8]byte{}
	s := arr[:]
	if _, err := rand.Read(s); err != nil {
		panic(err)
	}
	return IdBuilder{
		counter: binary.BigEndian.Uint64(s),
	}
}

//
// produce one digit value from byte value 0 - 61
//
func base62(b byte) byte {
	if b < 10 {
		return byte('0' + b)
	} else if b < 36 {
		return byte('a' - 10 + b)
	} else if b < 62 {
		return byte('A' - 36 + b)
	}
	panic("integer out of range for base 62 encode")
}

//
// produce 2 digit from byte value of 0 - 99
//
func base10(b byte) (one, two byte) {

	if b < 10 {
		one = '0'
		two = b + '0'
	} else if b < 20 {
		one = '1'
		two = b - 10 + '0'
	} else if b < 30 {
		one = '2'
		two = b - 20 + '0'
	} else if b < 40 {
		one = '3'
		two = b - 30 + '0'
	} else if b < 50 {
		one = '4'
		two = b - 40 + '0'
	} else if b < 60 {
		one = '5'
		two = b - 50 + '0'
	} else if b < 70 {
		one = '6'
		two = b - 60 + '0'
	} else if b < 80 {
		one = '7'
		two = b - 70 + '0'
	} else if b < 90 {
		one = '8'
		two = b - 80 + '0'
	} else if b < 100 {
		one = '9'
		two = b - 90 + '0'
	} else {
		panic("out of range for base10")
	}
	return
}

//
// creates a 24 rune string id using base62 character set
//
// top 11 runes are date/time
//
// bottom 13 runes are counter with random base
//
func (this *IdBuilder) NewId() (rv string) {
	arr := [24]byte{}
	s := arr[:]

	//
	// base32 encode all 64 bits of counter
	// - encode 5 bits at a time allows us to use base62 routine to get base32
	// - we stop at 10 because that's where date/time begins
	//
	u := atomic.AddUint64(&this.counter, 1)
	var shift uint = 0
	for i := len(arr) - 1; i > 10; i-- {
		b := byte(u >> shift)
		shift += 5
		s[i] = base62(b & 0x1f)
	}

	//
	// encode YMMDDHHMMSS
	//
	// we need the year as these can get databased
	//
	// we want the MMDDHHMMSS to be easily human readable
	//
	now := time.Now().UTC()
	year, month, day := now.Date()
	hour, minute, second := now.Clock()
	s[0] = base62(byte(year - 2016))
	s[1], s[2] = base10(byte(month))
	s[3], s[4] = base10(byte(day))
	s[5], s[6] = base10(byte(hour))
	s[7], s[8] = base10(byte(minute))
	s[9], s[10] = base10(byte(second))

	return string(s)
}
