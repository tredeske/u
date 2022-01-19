package usync

import (
	"github.com/dchest/siphash"
	"github.com/tredeske/u/ustrings"
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
// this saves about 80% compared to HashBytes([]byte(s)) and is more convenient
//
func HashString(s string) uintptr {
	return uintptr(siphash.Hash(sipHashKey1_, sipHashKey2_,
		ustrings.UnsafeStringToBytes(s)))
}
