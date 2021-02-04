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
	"strings"
	"syscall"
	"time"

	"github.com/tredeske/u/golum"
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/uinit"
	"github.com/tredeske/u/uio"
	"github.com/tredeske/u/ulog"
)

var (
	Testing = func() bool {
		syscall.Umask(002)                            // cause prior to init()
		return strings.HasSuffix(os.Args[0], ".test") // detect 'go test'
	}()
)

//
// simple boot using default values
//
func SimpleBoot() (rv Boot, err error) {
	boot := Boot{}
	err = boot.Boot()
	if err != nil {
		return
	}

	err = boot.Redirect(0)
	if err != nil {
		return
	}

	boot.Config, err = boot.Configure("components", nil)
	if err != nil {
		return
	}

	rv = boot
	return
}

//
// Control process boot
//
type Boot struct {
	Name     string           // what program calls itself
	InstallD string           // where program is installed
	ConfigF  string           // abs path to config file
	LogF     string           // path to log file, or empty/"stdout"
	StdoutF  string           // path to stdout file, or empty/"stdout" for stdout
	Config   *uconfig.Section // the loaded config

	//
	// set by build system.  examples:
	// go build -ldflags '-X /import/path.Version=#{$stamp}-#{REV}'
	// go build -ldflags '-X main.Version=#{$stamp}-#{REV}'
	//
	Version string
}

//
// bootstrap process to initial state.  order matters.
//
func (this *Boot) Boot() (err error) {

	if 0 == len(this.Name) {
		this.Name = filepath.Base(os.Args[0])
	}
	if 0 == len(this.InstallD) {
		this.InstallD = uconfig.InstallD
	}

	version := false
	show := ""

	flag.StringVar(&this.ConfigF, "config", this.ConfigF,
		"config file (config/[NAME].yml)")

	flag.BoolVar(&ulog.DebugEnabled, "debug", ulog.DebugEnabled,
		"turn on debugging")

	flag.StringVar(&this.LogF, "log", "",
		"set to 'stdout' or to path of log file (default: log/[NAME].log)")

	flag.StringVar(&this.Name, "name", this.Name, "name of program")

	flag.BoolVar(&version, "version", version, "print version and exit")

	flag.StringVar(&show, "show", show,
		"show settings for named component, or 'all'")

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
	if "stdout" == this.LogF || Testing {
		ulog.UseStdout = true
		this.StdoutF = "stdout"
	} else if 0 != len(this.LogF) {
		this.LogF, err = filepath.Abs(this.LogF)
		if err != nil {
			return
		}
		ulog.Dir = filepath.Dir(this.LogF)
	} else {
		ulog.Dir = path.Join(this.InstallD, "log")
		this.LogF = path.Join(ulog.Dir, this.Name+".log")
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

//
// Continue boot process: redirect stdin, stdout, stderr and setup logging
//
// if logF is empty, then use configured setting, which may be "stdout"
//
// invoke after Boot() or program initialized
//
func (this *Boot) Redirect(maxSz int64) (err error) {

	syscall.Close(0) // don't need stdin

	//
	// ulog
	//
	ulog.Init(this.LogF, maxSz)

	//
	// redirect stdout, stderr to file, if necessary
	//

	if 0 == len(this.StdoutF) {
		if ulog.UseStdout {
			this.StdoutF = "stdout"
		} else {
			this.StdoutF = this.Name + ".stdout"
		}
	}

	if "stdout" != this.StdoutF {

		if maxSz <= 0 {
			maxSz = 40 * 1024 * 1024
		}

		this.StdoutF = ulog.GetLogName(this.StdoutF)
		stdoutD := path.Dir(this.StdoutF)
		if !uio.FileExists(stdoutD) {
			if err = os.MkdirAll(stdoutD, 02775); err != nil {
				return uerr.Chainf(err, "problem creating %s", stdoutD)
			}
		}

		fi, err := os.Stat(this.StdoutF)
		if err == nil && fi.Size() > int64(maxSz) {
			dst := this.StdoutF + ".last"
			os.Remove(dst)
			os.Rename(this.StdoutF, dst)
		}
		fd, err := syscall.Open(this.StdoutF,
			syscall.O_WRONLY|syscall.O_APPEND|syscall.O_CREAT, 0664)
		if err != nil {
			return err
		} else if err = syscall.Dup2(fd, 1); err != nil {
			return err
		} else if err = syscall.Dup2(fd, 2); err != nil {
			return err
		}
		// add a marker to the stdout so we can triage panics
		fmt.Printf("\n\n%s: Process started.  Name=%s, Version=%s\n\n",
			time.Now().UTC().Format("2006/01/02 15:04:05Z"), this.Name,
			this.Version)
	}

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

//
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
//
func (this *Boot) Configure(
	cspec string,
	beforeStart func(c *uconfig.Section) (err error),
) (
	config *uconfig.Section,
	err error,
) {

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

				config.AddProp("logDir", ulog.Dir)
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

//
//
//
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

	// generic component load
	//
	config.AddProp("logDir", ulog.Dir)
	config.AddProp("name", this.Name)
	var gconfig *uconfig.Array
	err = config.GetArray(cspec, &gconfig)
	if err != nil {
		return
	}
	components, err := golum.Load(gconfig)
	if err != nil {
		return
	}

	if nil != beforeStart {
		err = beforeStart(config)
		if err != nil {
			return
		}
	}

	if nil != components {
		err = components.Start()
	}
	return
}
