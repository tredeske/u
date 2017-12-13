package uio

import (
	"fmt"
	"io"
)

//
// A SnipReader reads from R, but if the number of bytes exceeds N, then
// at most N bytes will be passed on.  The effect will be that the first
// N - NTail bytes will be passed on followed by the last NTail bytes.
//
// If Marker is set, its bytes are not included in the count, but its bytes
// will be placed between the head and tail bytes.
//
type SnipReader struct {
	R      io.Reader // underlying reader
	N      int       // max bytes to pass on
	NTail  int       // max bytes within N to pass on at tail of stream
	Marker []byte    // bytes to insert if/when snip point reached
	bb     []byte    // internal buffer
	eof    bool      // EOF reached - start draining internal buffer
	init   bool      //
}

//
// suggested Marker to use for SnipReader
//
var SnipMarker = []byte("\n...SNIP!...\n")

//
// implement io.Reader
//
func (this *SnipReader) Read(p []byte) (nread int, err error) {

	if 0 == this.N {
		return 0, io.EOF
	} else if 0 == len(p) {
		return
	} else if !this.init {
		if this.NTail >= this.N || 0 >= this.NTail {
			err = fmt.Errorf("NTail out of range.  It is %d, but N is %d",
				this.NTail, this.N)
			return
		}
		this.init = true
	}

	//
	// read direct for head of stream
	//
	if this.N > this.NTail {
		if len(p) > (this.N - this.NTail) {
			p = p[0:(this.N - this.NTail)]
		}
		nread, err = this.R.Read(p)
		this.N -= nread
		if io.EOF == err {
			this.N = 0
		}
		return
	}

	//
	// we're in the middle. read into our buffer until EOF
	//
	if !this.eof {

		temp := [512]byte{}

		for !this.eof && nil == err {

			slice := temp[:]
			amount := 0
			amount, err = this.R.Read(slice)
			if err != nil {
				this.eof = true
				if io.EOF == err {
					err = nil
					if 0 == amount && 0 == len(this.bb) {
						return 0, io.EOF
					}
				}
			}
			if 0 == amount {
				break
			}
			slice = slice[:amount]

			if nil == this.bb { // initialize buffer and add SNIP to output
				this.bb = make([]byte, 0, this.NTail)
				if 0 != len(this.Marker) && len(p) >= len(this.Marker) {
					nread = copy(p, this.Marker)
					p = p[nread:]
				}
			}

			rem := cap(this.bb) - len(this.bb)
			if amount < rem {
				this.bb = append(this.bb, slice...)

			} else if amount >= cap(this.bb) {
				if 0 != rem {
					this.bb = this.bb[:cap(this.bb)]
				}
				if amount > cap(this.bb) {
					slice = slice[amount-cap(this.bb):]
				}
				copy(this.bb, slice)

			} else {
				start := amount - rem
				copy(this.bb, this.bb[start:]) // make room
				if 0 != rem {
					this.bb = this.bb[:cap(this.bb)]
				}
				copy(this.bb[len(this.bb)-start:], slice) // add the new stuff
			}
		}
	}

	//
	// we've reached eof, so drain our tail buffer to reader
	//
	if this.eof {

		ncopied := copy(p, this.bb)
		if len(this.bb) > ncopied {
			this.bb = this.bb[ncopied:]
			this.N = len(this.bb)
		} else {
			this.N = 0
		}
		nread += ncopied

		if err != nil {
			this.N = 0
		}
	}

	return
}
