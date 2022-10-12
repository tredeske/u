package ulog

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/uio"
)

// an io.Writer to manage log output file rotation
type WriteManager struct {
	lock        sync.Mutex
	max         int64
	size        int64
	w           *os.File
	keep        int
	name        string
	dir         string
	base        string
	ext         string
	baseNoExt   string
	nameNoExt   string
	rotateMatch *regexp.Regexp
}

// create a new writer to manage a log file, with max bytes per log file
func NewWriteManager(
	file string,
	max int64,
	keep int,
) (
	rv *WriteManager,
	err error,
) {
	ext := filepath.Ext(file) // has the dot!
	base := filepath.Base(file)
	base = base[:len(base)-len(ext)]
	if 1 > keep || 1024 < keep {
		panic("log keep must be between 1 and 1024")
	} else if 1000 > max {
		panic("log max must be at least 1000")
	} else if 0 == len(ext) {
		panic("log file name must have an extension: " + file)
	} else if 0 == len(base) {
		panic("log file name must have a base: " + file)
	}
	absFile, err := filepath.Abs(file)
	if err != nil {
		return
	}
	rv = &WriteManager{
		name:      absFile,
		max:       max,
		keep:      int(keep),
		dir:       filepath.Dir(absFile),
		ext:       ext, // has the dot!
		baseNoExt: base,
		nameNoExt: absFile[:len(absFile)-len(ext)],
		// must match format in rotate()
		rotateMatch: regexp.MustCompile(`^` + base + `\.\d{6}\.\d{6}\` + ext + `$`),
	}
	err = rv.next()
	return
}

// implement io.Closer
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

func (this *WriteManager) Dir() string { return this.dir }

// implement io.Writer
func (this *WriteManager) Write(bb []byte) (n int, err error) {
	n = len(bb)
	if 0 == n {
		return
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
		err = this.next()
	}
	return
}

func (this *WriteManager) next() (err error) {
	if nil != this.w {
		this.w.Close()
		this.w = nil
		this.size = 0
	}
	if fi, ignore := os.Stat(this.name); ignore == nil {
		this.size = fi.Size()
		if this.size >= this.max {
			this.size = 0
			this.rotate(fi.ModTime())
		}
	}
	err = os.MkdirAll(this.dir, 02775)
	if err != nil {
		return uerr.Chainf(err, "problem creating dir for %s", this.name)
	}

	this.w, err = os.OpenFile(this.name, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0664)
	if err != nil {
		return uerr.Chainf(err, "problem creating %s", this.name)
	}
	return nil
}

func (this *WriteManager) rotate(when time.Time) {
	// must match regexp in this.rotateMatch
	dst := this.nameNoExt + when.Format(".060102.150405") + this.ext
	os.Remove(dst)
	os.Rename(this.name, dst)

	//
	// only keep so many log files around
	//
	dirF, err := os.Open(this.dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Unable to open dir %s: %s\n", this.dir, err)
		return
	}
	defer dirF.Close()

	files, err := dirF.Readdir(0)
	if err != nil {
		fmt.Fprintf(os.Stderr, "ERROR: Unable to list dir %s: %s\n", this.dir, err)
		return
	}

	matching := make([]os.FileInfo, 0, len(files))
	for i := range files {
		if files[i].Mode().IsRegular() &&
			this.rotateMatch.MatchString(files[i].Name()) {
			matching = append(matching, files[i])
		}
	}
	if this.keep > len(matching) {
		return
	}

	uio.SortByModTime(matching) // oldest to newest

	last := len(matching) - this.keep

	for i := range matching[:last] {
		rm := filepath.Join(this.dir, matching[i].Name())
		os.Remove(rm)
	}
}
