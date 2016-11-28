package ulog

import (
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	//
	// the output to use for logging
	// may be set by uboot
	//
	W io.Writer = os.Stdout

	//
	// Can't get at the default log.Logger, so this wraps W
	//
	L *log.Logger

	//
	// file location where log files go
	// may be set by uboot
	//
	Dir = ""

	UseStdout = strings.HasSuffix(os.Args[0], ".test") // detect 'go test'

	maxSz_ = int64(40 * 1024 * 1024)
)

//
// Get name of log file to use, or 'stdout'
//
func GetLogName(base string) (rv string) {

	if UseStdout {
		rv = "stdout"

	} else if 0 == len(base) {
		rv = filepath.Base(os.Args[0]) + ".log"
		if 0 != len(Dir) {
			rv = filepath.Join(Dir, rv)
		}

	} else if strings.ContainsRune(base, '/') {
		rv = base

	} else {
		rv = filepath.Join(Dir, base)
	}
	return
}

//
// Get the named writer
//
func GetWriter(base string, maxSz int64) (name string, rv io.Writer, err error) {

	if 0 >= maxSz {
		maxSz = maxSz_
	}

	name = GetLogName(base)
	if "stdout" == name {
		rv = os.Stdout

	} else {
		rv, err = NewWriteManager(name, maxSz)
	}
	return
}

//
//
//
func NewLogger(base string, maxSz int64) (name string, rv *log.Logger, err error) {

	var w io.Writer
	name, w, err = GetWriter(base, maxSz)
	if nil == err {
		rv = log.New(w, "", log.LstdFlags)
	}
	return
}

//
// Initialize log output to go to logF.  If logF is empty, then use
// ulog.Dir/[prog name] setting.  If that is empty, then use stdout.
//
// maxSz is the maximum output file size.  if unset, we use default of 40M.
//
// used by uboot
//
func Init(logF string, maxSz int64) (err error) {

	if 0 < maxSz {
		maxSz_ = maxSz
	}

	if 0 == len(logF) {
		logF = GetLogName(logF)
	}

	if 0 != len(logF) && "stdout" != logF && !UseStdout {
		logD := Dir
		if strings.ContainsRune(logF, '/') || 0 == len(Dir) {
			logF, err = filepath.Abs(logF)
			if err != nil {
				return
			}
			logD = path.Dir(logF)
			if 0 == len(Dir) {
				Dir = logD
			}
		}

		_, W, err = GetWriter(logF, maxSz_)
		if nil == err {
			L = log.New(W, "", log.LstdFlags)
			log.SetOutput(W)
		}
	} else {
		L = log.New(os.Stdout, "", log.LstdFlags)
		UseStdout = true
	}
	return
}
