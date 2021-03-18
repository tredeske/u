package ulog

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var (

	//
	// file location where log files go
	//
	dir_ = ""

	testing_ = strings.HasSuffix(os.Args[0], ".test") // detect 'go test'
	stdout_  = true

	maxSz_ = int64(40 * 1024 * 1024)
)

//
// Get name of log file to use, or 'stdout'
//
// This name is based on what was set in Init, so if stdout was configured
// there, then the output will be to stdout.
//
func getLogName(base string) (rv string) {

	if testing_ || stdout_ || 0 == len(base) || "stdout" == base {
		rv = "stdout"

	} else if strings.ContainsRune(base, '/') {
		rv = base

	} else if 0 != len(dir_) {
		rv = filepath.Join(dir_, base)

	} else {
		rv = base
	}
	return
}

//
// Create a new logger based on provided info and on what was set in Init
//
func NewLogger(base string, maxSz int64) (rv *log.Logger, err error) {

	var w io.Writer
	output := getLogName(base)
	if "stdout" == output {
		w = os.Stdout
	} else {
		if 0 >= maxSz {
			maxSz = maxSz_
		}
		w, err = NewWriteManager(output, maxSz)
	}
	if nil == err {
		rv = log.New(w, "", log.LstdFlags)
	}
	return
}

//
// Initialize log output (ulog and golang standard log) to go to logF.
//
// If logF is empty or set to 'stdout', then use stdout
//
// Otherwise, the log file will be written to and managed (rotated when
// maxSz reached).
//
// maxSz is the maximum output file size.  if unset, we use default of 40M.
//
// used by uboot
//
func Init(logF string, maxSz int64) (err error) {

	var w io.Writer

	if 0 == len(logF) || "stdout" == logF {
		stdout_ = true
		w = os.Stdout

	} else { // file
		stdout_ = false
		if 0 < maxSz {
			maxSz_ = maxSz
		}
		var wMgr *WriteManager
		wMgr, err = NewWriteManager(logF, maxSz_)
		if nil != wMgr {
			dir_ = wMgr.Dir()
			w = wMgr
		}
	}
	if nil == err {
		log.SetOutput(w)
	}
	return
}
