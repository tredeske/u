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
// this saves about 10% compared to HashBytes([]byte(s)) and is more convenient
//
func HashString(s string) uintptr {
	/* go vet does not like this anymore
	sh := (*reflect.StringHeader)(unsafe.Pointer(&s))
	bh := reflect.SliceHeader{
		Data: sh.Data,
		Len:  sh.Len,
		Cap:  sh.Len,
	}
	buf := *(*[]byte)(unsafe.Pointer(&bh))
	*/

	var buf []byte
	hdr := (*reflect.SliceHeader)(unsafe.Pointer(&buf))
	hdr.Data = uintptr(unsafe.Pointer(
		(*reflect.StringHeader)(unsafe.Pointer(&s)).Data))
	hdr.Len = len(s)
	hdr.Cap = len(s)
	return uintptr(siphash.Hash(sipHashKey1_, sipHashKey2_, buf))
}
