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

/*
func (this *IdBuilder) NewId() (rv string) {
	u := atomic.AddUint64(&this.counter, 1)
	arr := [4]byte{}
	s := arr[:]
	binary.BigEndian.PutUint32(s, uint32(u))
	return time.Now().UTC().Format("060102150405") + hex.EncodeToString(s)
}
*/

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
// creates a 16 rune string id
//
// top 9 runes are date/time
//
// bottom 7 runes are counter with random base
//
func (this *IdBuilder) NewId() (rv string) {
	arr := [16]byte{}
	s := arr[:]

	//
	// base32 encode (encode 5 bits at a time) 35 bits of counter
	//
	u := atomic.AddUint64(&this.counter, 1)
	b := byte(u)
	s[15] = base62(b & 0x1f)
	b = byte(u >> 5)
	s[14] = base62(b & 0x1f)
	b = byte(u >> 10)
	s[13] = base62(b & 0x1f)
	b = byte(u >> 15)
	s[12] = base62(b & 0x1f)
	b = byte(u >> 20)
	s[11] = base62(b & 0x1f)
	b = byte(u >> 25)
	s[10] = base62(b & 0x1f)
	b = byte(u >> 30)
	s[9] = base62(b & 0x1f)

	/*
		binary.LittleEndian.PutUint64(arr[:8], u)
		for i, j := 0, len(s)-1; i < 8; i++ {
			b := s[i]
			s[j] = base62(b & 0xf)
			j--
			s[j] = base62(b >> 4)
			j--
		}
	*/

	//
	// encode YMDHHMMSS
	//
	// we need the year as these can get databased
	//
	// we want the HHMMSS to be easily human readable
	//
	now := time.Now().UTC()
	year, month, day := now.Date()
	hour, minute, second := now.Clock()
	s[0] = base62(byte(year - 2016))
	s[1] = base62(byte(month))
	s[2] = base62(byte(day))
	s[3], s[4] = base10(byte(hour))
	s[5], s[6] = base10(byte(minute))
	s[7], s[8] = base10(byte(second))

	return string(s)
}
