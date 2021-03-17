package usync

import (
	"reflect"
	"unsafe"

	"github.com/dchest/siphash"
)

//
// compute a unique hash for a short byte slice
//
func HashBytes(b []byte) uintptr {
	return uintptr(siphash.Hash(sipHashKey1_, sipHashKey2_, b))
}

//
// compute a unique hash for a short string
//
func HashString(s string) uintptr {
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	buf := *(*[]byte)(unsafe.Pointer(&bh))
	return uintptr(siphash.Hash(sipHashKey1_, sipHashKey2_, buf))
}
