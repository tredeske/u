package ulog

import (
	"os"
	"path/filepath"
	"sync"

	"github.com/tredeske/u/uerr"
)

// an io.Writer to manage log output file rotation
type WriteManager struct {
	name string
	max  int64
	size int64
	w    *os.File
	lock sync.Mutex
}

// create a new writer to manage a log file, with max bytes per log file
func NewWriteManager(file string, max int64) (*WriteManager, error) {
	rv := &WriteManager{
		name: file,
		max:  max,
	}
	return rv, rv.next()
}

func (this *WriteManager) Close() error {
	this.lock.Lock()
	defer this.lock.Unlock()
	if nil != this.w {
		w := this.w
		this.w = nil
		return w.Close()
	}
	return nil
}

func (this *WriteManager) Write(bb []byte) (n int, err error) {
	n = len(bb)
	if 0 == n {
		return 0, nil
	}
	this.lock.Lock()
	defer this.lock.Unlock()
	if nil == this.w {
		if err = this.next(); err != nil {
			return 0, err
		}
	}
	if _, err = this.w.Write(bb); err != nil {
		return 0, err
	}
	this.size += int64(n)
	if this.size >= this.max {
		if err = this.next(); err != nil {
			return 0, err
		}
	}
	return n, nil
}

func (this *WriteManager) next() (err error) {
	if nil != this.w {
		this.w.Close()
		this.w = nil
		this.size = 0
	}
	if fi, err := os.Stat(this.name); err == nil {
		this.size = fi.Size()
		if this.size >= this.max {
			dst := this.name + fi.ModTime().Format(".060102.150405") +
				filepath.Ext(this.name)
			os.Remove(dst)
			os.Rename(this.name, dst)
			this.size = 0
		}
	}
	if err = os.MkdirAll(filepath.Dir(this.name), 02775); err != nil {
		return uerr.Chainf(err, "problem creating dir for %s", this.name)
	}
	this.w, err = os.OpenFile(this.name, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
	if err != nil {
		return uerr.Chainf(err, "problem creating %s", this.name)
	}
	return nil
}
