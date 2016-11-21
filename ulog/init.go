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
	Dir  = ""
	File = ""

	Testing = strings.HasSuffix(os.Args[0], ".test") // detect 'go test'

	maxSz_ = int64(40 * 1024 * 1024)
)

//
// Get name of log file to use, or 'stdout'
//
func GetLogName(base string) (rv string) {

	if 0 == len(File) || "stdout" == File || Testing {
		rv = "stdout"

	} else if 0 == len(base) {
		rv = File
		if 0 != len(Dir) {
			rv = filepath.Join(Dir, File)
		}

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

	} else if 0 == len(base) {
		if nil == W {
			W, err = NewWriteManager(name, maxSz)
		}
		rv = W

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
		if nil == L {
			L = rv
			log.SetOutput(w)
			if nil == W {
				W = w
			}
		}
	}
	return
}

//
// Initialize log output to go to logF.  If logF is empty, then use
// ulog.File setting.  If that is empty, then use stdout.
//
// maxSz is the maximum output file size.  if unset, we use default of 40M.
//
// used by uboot
//
func Init(logF string, maxSz int64) (err error) {

	if 0 >= maxSz {
		maxSz_ = maxSz
	}

	if 0 == len(logF) {
		logF = File
		if 0 != len(Dir) {
			logF = filepath.Join(Dir, File)
		}
	}

	if 0 != len(logF) && "stdout" != logF {
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

		_, _, err = NewLogger(logF, maxSz_)
	}
	return
}
