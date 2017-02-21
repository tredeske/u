package uio

import (
	"io"
	"sync"
)

//
// default BufferPool to use
//
var DefaultPool = BufferPool{
	size: 64 * 1024,
	bufC: make(chan *Buffer, 64),
}

//
// protect the buffer from common mistakes
//
type Buffer struct {
	bb   []byte
	pool *BufferPool
}

// set all bytes to 0
func (this *Buffer) Clear() *Buffer {
	for i, _ := range this.bb {
		this.bb[i] = 0
	}
	return this
}

// return it to the pool
func (this *Buffer) Return() {
	p := this.pool
	this.pool = nil
	p.put(this)
}

// get the slice
func (this *Buffer) B() []byte {
	if !this.IsValid() {
		panic("accessing invalid buffer")
	}
	return this.bb
}

// get the len
func (this *Buffer) Len() int {
	if !this.IsValid() {
		panic("accessing invalid buffer")
	}
	return len(this.bb)
}

// ok to use this?
func (this *Buffer) IsValid() bool {
	return nil != this.pool
}

//
// A pool of buffers
//
type BufferPool struct {
	size int          // required size of each buffer
	bufC chan *Buffer //
}

// create a new pool
func NewBufferPool(size, buffs int) (rv *BufferPool) {
	return (&BufferPool{}).Construct(size, buffs)
}

// construct an already allocated pool
func (this *BufferPool) Construct(size, buffs int) (rv *BufferPool) {
	if 0 >= size {
		size = 64 * 1024
	}
	if 0 >= buffs {
		buffs = 32
	}
	this.size = size
	this.bufC = make(chan *Buffer, buffs)
	return this
}

// Preallocate the specified number of Buffers
func (this *BufferPool) Prealloc(buffs int) (rv *BufferPool) {
	for i := 0; i < buffs; i++ {
		this.Get().Return()
	}
	return this
}

// get a buffer from the pool, or created if none available
func (this BufferPool) Get() (rv *Buffer) {
	select {
	case rv = <-this.bufC:
		rv.pool = &this
	default:
		rv = &Buffer{
			pool: &this,
			bb:   make([]byte, this.size),
		}
	}
	return
}

// get a buffer from the pool, waiting forever until one is available
func (this BufferPool) GetForever() (rv *Buffer) {
	rv = <-this.bufC
	rv.pool = &this
	return
}

func (this BufferPool) put(b *Buffer) {
	select { // try to put the buf into the chanel, discard otherwise
	case this.bufC <- b:
	default:
	}
}

//
// copy to dst from src using a buffer from this pool
//
// just like io.Copy() or io.CopyBuffer
//
func (this BufferPool) Copy(dst io.Writer, src io.Reader) (nwrote int64, err error) {
	b := this.Get()
	nwrote, err = io.CopyBuffer(dst, src, b.B())
	b.Return()
	return
}

//
//
// ------------------------------------------------------------------
//
//

var DefaultBytesPool = (&BytesPool{}).Construct(64 * 1024)

//
// implement httputil.BufferPool
//
type BytesPool struct {
	size int       // of each slice
	pool sync.Pool //
}

//
// construct the pool. a size of 0 or less will use default size for the
// size of each []byte slice
//
func (this *BytesPool) Construct(size int) *BytesPool {
	if 0 >= size {
		size = 64 * 1024
	}
	this.size = size
	this.pool.New = func() interface{} {
		return make([]byte, size)
	}
	return this
}

//
// get a byte slice
//
func (this *BytesPool) Get() (rv []byte) {
	return this.pool.Get().([]byte)
}

//
// return a byte slice
//
func (this *BytesPool) Put(bb []byte) {
	this.pool.Put(bb[:this.size])
}
