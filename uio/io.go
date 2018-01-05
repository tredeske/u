package uio

import (
	"errors"
	"io"
)

//
// implements io.Reader
// reports progress
//
type ObservableReader struct {
	R io.Reader         // underlying reader
	F func(nread int64) // used to report metrics
	//Meter metrics.Meter // if set, record rate
}

// implement io.Reader
func (this *ObservableReader) Read(p []byte) (nread int, err error) {

	nread, err = this.R.Read(p)

	if nil != this.F && 0 != nread {
		this.F(int64(nread))
	}
	return
}

//
// A StopLossReader reads from R but errors if the amount of data
// being read exceeds N bytes.
// Each call to Read updates N to reflect the new amount remaining.
//
type StopLossReader struct {
	R          io.Reader    // underlying reader
	N          int64        // max bytes remaining
	OnExceeded func() error // if set, produces error on bounds exceeded
}

var StopLossExceededError = errors.New("stop loss exceeded")

func (this *StopLossReader) Read(p []byte) (n int, err error) {
	if this.N <= 0 {
		err = StopLossExceededError
		if nil != this.OnExceeded {
			err = this.OnExceeded()
		}
		return 0, err
	}
	if int64(len(p)) > this.N {
		p = p[0:this.N]
	}
	n, err = this.R.Read(p)
	this.N -= int64(n)
	return
}

//
// counts bytes written
//
type CountingWriter struct {
	N int64
	W io.Writer
}

func (this *CountingWriter) Write(b []byte) (rv int, err error) {
	rv, err = this.W.Write(b)
	this.N += int64(rv)
	return
}

//
// throws everything away
//
type NullWriter struct{}

func (this *NullWriter) Write(b []byte) (rv int, err error) {
	rv = len(b)
	return
}

//
// read until no more data to read
//
func Drain(r io.Reader) (nread int64, err error) {
	w := NullWriter{}
	return CopyTo(&w, r, 0)
}

//
// read until n bytes read or no more data to read.
//
// if eof reached prior to n bytes read, no error is returned
//
func DrainN(r io.Reader, n int64) (nread int64, err error) {
	w := NullWriter{}
	limitR := io.LimitedReader{
		R: r,
		N: n,
	}
	return CopyTo(&w, &limitR, 0)
}
