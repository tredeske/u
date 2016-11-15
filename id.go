package u

import (
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
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

func (this *IdBuilder) NewId() (rv string) {
	u := atomic.AddUint64(&this.counter, 1)
	arr := [4]byte{}
	s := arr[:]
	binary.BigEndian.PutUint32(s, uint32(u))
	return time.Now().UTC().Format("060102150405") + hex.EncodeToString(s)
}
