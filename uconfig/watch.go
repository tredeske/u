package uconfig

import (
	"os"
	"sync"
	"time"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/ulog"
)

// used to watch for changes to files
type Watch struct {
	lock   sync.Mutex
	files  []string
	filesC chan string
}

// add a file to watch
//
// we may get files to add prior to being started
func (this *Watch) Add(file string) {
	this.lock.Lock()
	if nil == this.filesC {
		this.files = append(this.files, file)
	} else {
		this.filesC <- file
	}
	this.lock.Unlock()
}

// stop watching
func (this *Watch) Stop() {
	if nil != this.filesC {
		uerr.IgnorePanicIn(func() { close(this.filesC) })
	}
}

// watch files.  if there is a change, then call onChange.
// if there is an error and onError is set, then call it.
//
// if either func returns true, then watching will be stopped
func (this *Watch) Start(
	period time.Duration,
	onChange func(changedFile string) (done bool),
	onError func(err error) (done bool),
) {

	this.lock.Lock()
	defer this.lock.Unlock()

	if nil != this.filesC {
		panic("should not happen: watcher already started")
	}

	filesC := make(chan string, 2)

	//
	// start watching
	//
	go func() {
		updated := time.Now()
		ticker := time.NewTicker(period)
		files := []string{}

		defer func() {
			ulog.Warnf("Config Watch terminated for %v", files)
			ticker.Stop()
		}()

		for {
			select {
			case f, ok := <-filesC:
				if !ok {
					return //////////////////// time to stop
				}
				ulog.Println("Watching", f)
				files = append(files, f)

			case <-ticker.C: // time to check

				for _, f := range files {
					stat, err := os.Stat(f)
					if err != nil {
						if nil != onError {
							err = uerr.Chainf(err, "checking %s", f)
							if onError(err) {
								return ///////////////////////// time to stop
							}
						}
						break

					} else if stat.ModTime().After(updated) {

						ulog.Println("Changed:", f)
						updated = stat.ModTime()
						if onChange(files[0]) {
							return ///////////////////////////// time to stop
						}
						break
					}
				}
			}
		}
	}()

	//
	// tell about previously registered files
	//
	this.filesC = filesC
	for _, f := range this.files {
		filesC <- f
	}
	this.files = nil
}
