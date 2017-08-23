package uio

import (
	"fmt"
	"io"
)

//
// copy bytes from src io.Reader to dst io.Writer using provided buffer.
// if srcSz is a positive number, ensure srcSz bytes were copied
// if no provided buffer, then use a default buffer
//
func CopyBufferTo(dst io.Writer, src io.Reader, srcSz int64, buf []byte,
) (amount int64, err error) {

	if 0 == len(buf) {
		b := DefaultPool.Get()
		buf = b.B()
		defer b.Return()
	}

	amount, err = io.CopyBuffer(dst, src, buf)
	if err != nil {
		return
	} else if 0 < srcSz && amount != srcSz {
		err = fmt.Errorf("Copy failed: missing bytes: srcSize=%d, copied=%d",
			srcSz, amount)
	}
	return
}

//
// Same as CopyBufferTo, but use buffer from pool
//
func CopyTo(dst io.Writer, src io.Reader, srcSz int64) (amount int64, err error) {
	return CopyBufferTo(dst, src, srcSz, nil)
}
