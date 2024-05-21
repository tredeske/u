// Package uboot manages process startup, setting up logging, signal handling,
// and loading of components through golum and uconfig.
//
// This works well for processes with minimal command line flags, and
// behavior defined by YAML config (see uconfig) (see golum).
//
//	func main() {
//	    _, err := uboot.SimpleBoot()
//	    if err != nil {
//	        ulog.Fatalf(..., err)
//	    }
//
//	    ...
//
//	    uexit.SimpleSignalHandling()
//	}
//
// From the command line, it is possible to see what components are supported:
//
//	program -show all
//	program -show [component]
//
// To run program:
//
//	program -config config.yml -log [stdout|logfile]
package uboot

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"syscall"
	"time"

	"github.com/tredeske/u/golum"
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/uexit"
	"github.com/tredeske/u/uinit"
	"github.com/tredeske/u/ulog"
)

var (
	Testing = func() bool {
		syscall.Umask(002)                            // cause prior to init()
		return strings.HasSuffix(os.Args[0], ".test") // detect 'go test'
	}()
	cpuProfile_ = flag.String("cpu-profile", "", "write cpu profile to `file`")
	memProfile_ = flag.String("mem-profile", "", "write memory profile to `file`")
)

// simple boot using default values
func SimpleBoot() (rv Boot, err error) {
	rv = Boot{}
	err = rv.Simple()
	return
}

// Control process boot
type Boot struct {
	Name     string           // what program calls itself
	InstallD string           // where program is installed
	ConfigF  string           // abs path to config file
	LogF     string           // path to log file, or empty/"stdout"
	LogSize  int64            // size of log file before rotate
	LogKeep  int              // logs to keep around
	Config   *uconfig.Section // the loaded config
	DryRun   bool             // is this a dry run (config check)?

	//
	// set by build system.  examples:
	// go build -ldflags '-X /import/path.Version=#{$stamp}-#{REV}'
	// go build -ldflags '-X main.Version=#{$stamp}-#{REV}'
	//
	Version string
}

// simple boot using supplied Boot
func (this *Boot) Simple() (err error) {

	err = this.Boot()
	if err != nil {
		return
	}

	err = this.Redirect()
	if err != nil {
		return
	}

	this.Config, err = this.Configure("components", nil)
	if err != nil {
		return
	}

	return
}

// bootstrap process to initial state.  order matters.
func (this *Boot) Boot() (err error) {

	if 0 == len(this.Name) {
		this.Name = filepath.Base(os.Args[0])
	}
	if 0 == len(this.InstallD) {
		this.InstallD = uconfig.InstallD
	}

	version := false
	show := ""
	logSzStr := "40Mi"
	logKeep := 4

	flag.BoolVar(&this.DryRun, "dry-run", this.DryRun,
		"Load but do not start components.  Useful to check config.")

	flag.StringVar(&this.ConfigF, "config", this.ConfigF,
		"Config file (config/[NAME].yml)")

	flag.BoolVar(&ulog.DebugEnabled, "debug", ulog.DebugEnabled,
		"Turn on debugging")

	flag.StringVar(&this.LogF, "log", "",
		"Set to 'stdout' or to path of log file (default: log/[NAME].log)")

	flag.StringVar(&logSzStr, "log-size", logSzStr,
		"Size of log file before rotation.  K,M,G,Ki,Mi,Gi suffixes supported.")

	flag.IntVar(&logKeep, "log-keep", logKeep, "Number of log files to keep around")

	flag.StringVar(&this.Name, "name", this.Name, "Name of program")

	flag.BoolVar(&version, "version", version, "Print version and exit")

	flag.StringVar(&show, "show", show,
		"Show settings for named component, or 'all'")

	flag.Parse()

	if version {
		fmt.Printf("Version %s\n", this.Version)
		os.Exit(0)
	} else if 0 != len(show) {
		golum.Show(show, os.Stdout)
		os.Exit(0)
	}

	if 0 == len(this.Name) {
		err = errors.New("Program name (-name param) not specified")
		return
	}
	uconfig.ThisProcess = this.Name
	uconfig.DryRun = this.DryRun

	//
	// verify we have a config file
	//
	if 0 == len(this.ConfigF) {
		this.ConfigF = path.Join(this.InstallD, "config", this.Name+".yml")
	}
	this.ConfigF, err = filepath.Abs(this.ConfigF)
	if err != nil {
		return
	}
	_, err = os.Stat(this.ConfigF)
	if err != nil {
		return fmt.Errorf("Config file missing?: %s", err)
	}

	//
	// Add this dir to PATH
	//
	err = os.Setenv("PATH", os.Getenv("PATH")+":"+uconfig.ThisD)
	if err != nil {
		return
	}

	//
	// get absolute path to log file
	// if we're running in 'go test' or if cmdline says so, then output to stdout
	//
	if this.DryRun {
		this.LogF = "stdout"
	}
	if "stdout" == this.LogF || Testing {
		this.LogF = "stdout"
	} else {
		if 0 == len(this.LogF) {
			this.LogF = path.Join(this.InstallD, "log", this.Name+".log")
		}
		if 0 != len(logSzStr) {
			err = uconfig.IntFromSiString(logSzStr, &this.LogSize)
			if err != nil {
				return
			}
		}
		if 2 > logKeep {
			logKeep = 2
		}
		this.LogKeep = logKeep
	}

	/*

		not needed when using systemd (KillMode=control-group)

		also, appears to hang in docker containers

		// Set the process group id of this process to be the same as the pid
		// so that a kill -pid (negative pid) will kill this process and all of
		// its children
		//
		if !Testing {
			if err = syscall.Setpgid(0, 0); err != nil {
				return err
			}
		}
	*/

	err = os.Chdir(this.InstallD)
	if err != nil {
		return
	}

	return uconfig.InitEnv()
}

// Continue boot process: redirect stdin, stdout, stderr and setup logging
//
// if logF is empty, then use configured setting, which may be "stdout"
//
// invoke after Boot() or program initialized
func (this *Boot) Redirect() (err error) {

	//syscall.Close(0) // don't need stdin

	//
	// ulog
	//
	if "stdout" != this.LogF {
		if 10_000_000 > this.LogSize {
			this.LogSize = 10_000_000
		}
		if 2 > this.LogKeep {
			this.LogKeep = 2
		}
	}
	err = ulog.Init(this.LogF, this.LogSize, this.LogKeep)
	if err != nil {
		return
	}

	//
	// redirect stdout, stderr to file, if necessary
	//

	/*
		if "stdout" != this.LogF {

			var absLogF string
			absLogF, err = filepath.Abs(this.LogF)
			if err != nil {
				return
			}
			dir := filepath.Dir(absLogF)
			stdoutF := path.Join(dir, this.Name+".stdout")

			var w *ulog.WriteManager
			w, err = ulog.NewWriteManager(stdoutF, this.LogSize, this.LogKeep)
			if err != nil {
				return
			}

			pipes := [2]int{}
			err = syscall.Pipe(pipes[:])
			if err != nil {
				return uerr.Chainf(err, "problem creating stdout pipe")
			}

			go stdouter(pipes[0], w)

			err = syscall.Dup2(pipes[1], 1)
			if err != nil {
				return
			}
			err = syscall.Dup2(pipes[1], 2)
			if err != nil {
				return err
			}
			// add a marker to the stdout so we can triage panics
			fmt.Printf("\n\n%s: Process started.  Name=%s, Version=%s\n\n",
				time.Now().UTC().Format("2006/01/02 15:04:05Z"), this.Name,
				this.Version)
		}
	*/

	log.Printf(`

=================================
Starting
    Name:       %s
    Version:    %s
    InstallDir: %s
    Config:     %s
=================================

`, this.Name, this.Version, this.InstallD, this.ConfigF)

	return nil
}

// output stdout / stderr to a managed file
func stdouter(fd int, w *ulog.WriteManager) {
	buff := make([]byte, 4096)
	_ = buff[4095] // bounds check elimination
	var nread, nwrote, pos int
	var err error
	for {
		nread, err = syscall.Read(fd, buff)
		if err != nil {
			ulog.Fatalf("stdout writer read failed: %T, %s", err, err)
		} else if 0 == nread {
			continue
		}
		pos = 0
	again:
		nwrote, err = w.Write(buff[pos:nread])
		if err != nil {
			ulog.Fatalf("stdout writer write failed: %T, %s", err, err)
		}
		pos += nwrote
		if pos != nread {
			goto again
		}
	}
}

// Continue boot process: configure from ConfigF (if avail)
//
// if cspec set, use golum to load the components listed in that section.
//
// if beforeStart set, invoke the function after loading golum components but
// before starting them.
//
// The following substitutions are automatically added for components:
// - name
// - configFile
// - logDir
//
// invoke after Redirect() or logging initialized
func (this *Boot) Configure(
	cspec string,
	beforeStart func(c *uconfig.Section) (err error),
) (
	config *uconfig.Section,
	err error,
) {

	profile()

	log.Printf("configuring from %s", this.ConfigF)
	config, err = uinit.InitConfig(this.ConfigF)
	if err != nil {
		return
	}

	concurrency := 0
	err = config.GetInt("concurrency", &concurrency)
	if err != nil {
		return
	} else if 0 < concurrency {
		runtime.GOMAXPROCS(int(concurrency))
	}

	log.Printf("GOMAXPROCS=%d", runtime.GOMAXPROCS(-1))
	log.Printf("Env: %#v", os.Environ())

	err = this.loadComponents(config, cspec, beforeStart)
	if err != nil {
		return
	}

	autoreload := false
	err = config.GetBool("autoreload", &autoreload)
	if err != nil {
		return

	} else if autoreload {

		config.Watch(7*time.Second,

			// always return false - we want to always keep retrying
			func(file string) (done bool) {
				config, err := uinit.InitConfig(this.ConfigF)
				if err != nil {
					ulog.Errorf("Unable to parse %s: %s", this.ConfigF, err)
					return false
				}

				//config.AddProp("logDir", ulog.Dir)
				config.AddProp("name", this.Name)
				var gconfig *uconfig.Array
				err = config.GetArray(cspec, &gconfig)
				if err != nil {
					ulog.Errorf("Getting '%s' from %s: %s", cspec, this.ConfigF, err)
					return false
				}
				err = golum.Reload(gconfig)
				if err != nil {
					ulog.Errorf("Unable to load components: %s", err)
				}
				return false
			},

			func(err error) (done bool) {
				ulog.Errorf("G: Problem checking config file: %s", err)
				return false
			})
	}
	return
}

func (this *Boot) loadComponents(
	config *uconfig.Section,
	cspec string,
	beforeStart func(c *uconfig.Section) (err error),
) (
	err error,
) {
	if 0 == len(cspec) {
		return // nothing left to do if no components to load
	}

	if this.DryRun {
		golum.DryRun.Store(true)
	}

	// generic component load
	//
	//config.AddProp("logDir", ulog.Dir)
	config.AddProp("name", this.Name)
	var gconfig *uconfig.Array
	err = config.GetArray(cspec, &gconfig)
	if err != nil {
		return
	}
	err = golum.Load(gconfig)
	if err != nil {
		return
	}

	if this.DryRun {
		ulog.Printf("Completed dry run successfully")
		os.Exit(0)
	}

	if nil != beforeStart {
		err = beforeStart(config)
		if err != nil {
			return
		}
	}

	err = golum.Start()
	return
}

func profile() {
	var err error
	var cpuF *os.File

	if *cpuProfile_ != "" {
		ulog.Printf("Saving CPU profile to " + *cpuProfile_)
		cpuF, err = os.Create(*cpuProfile_)
		if err != nil {
			log.Fatal("could not create CPU profile: ", err)
		}
		err = pprof.StartCPUProfile(cpuF)
		if err != nil {
			log.Fatal("could not start CPU profile: ", err)
		}
	}

	if nil != cpuF || *memProfile_ != "" {
		uexit.AtExitF(func(exitCode int) {
			if nil != cpuF {
				pprof.StopCPUProfile()
				cpuF.Close()
				ulog.Printf("Collected CPU profile to " + *cpuProfile_)
			}
			if *memProfile_ != "" {
				f, err := os.Create(*memProfile_)
				if err != nil {
					log.Fatal("could not create memory profile: ", err)
				}
				defer f.Close()
				runtime.GC() // get up-to-date statistics
				err = pprof.WriteHeapProfile(f)
				if err != nil {
					log.Fatal("could not write memory profile: ", err)
				}
				ulog.Printf("Collected memory profile to " + *memProfile_)
			}
		})
	}
}
