package uio

//
// protect the buffer from common mistakes
//
type Buffer struct {
	bb   []byte
	pool *BufferPool
}

func (this *Buffer) Clear() *Buffer {
	for i, _ := range this.bb {
		this.bb[i] = 0
	}
	return this
}

func (this *Buffer) Return() {
	p := this.pool
	this.pool = nil
	p.put(this)
}

func (this *Buffer) B() []byte {
	if !this.IsValid() {
		panic("accessing invalid buffer")
	}
	return this.bb
}

func (this *Buffer) Len() int {
	if !this.IsValid() {
		panic("accessing invalid buffer")
	}
	return len(this.bb)
}

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

func NewBufferPool(size, buffs int) (rv *BufferPool) {
	return (&BufferPool{}).Construct(size, buffs)
}

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

func (this BufferPool) put(b *Buffer) {
	select { // try to put the buf into the chanel, discard otherwise
	case this.bufC <- b:
	default:
	}
}
