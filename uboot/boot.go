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
	pre_     = pre()          // cause this to occur first
	Name     = ""             // what program calls itself
	InstallD = ""             // where program is installed
	ConfigF  = ""             // abs path to config file
	GlobalF  = ""             // abs path to globals (if any)
	Globals  *uconfig.Section //

	Testing = strings.HasSuffix(os.Args[0], ".test") // detect 'go test'

	//
	// set by build system.  example:
	// go build -ldflags '-X #{GO_PKG}/u/uboot.Version=#{$stamp}-#{REV}'
	//
	Version string // set by build system

	theReloader_ *reloader_ //
)

func pre() bool {
	syscall.Umask(002) // cause umask to be set prior to any init() methods
	return true
}

//
// bootstrap process to initial state.  order matters.
//
func Boot(name string) (err error) {

	Name = name
	InstallD = uconfig.InstallD

	version := false
	show := ""
	flag.StringVar(&ConfigF, "config", "", "config file (config/[NAME].yml)")
	flag.StringVar(&GlobalF, "globals", "", "global subst params file to load")
	flag.BoolVar(&ulog.DebugEnabled, "debug", ulog.DebugEnabled, "turn on debugging")
	flag.StringVar(&ulog.File, "log", ulog.File,
		"set to 'stdout' or to path of log file (default: log/[NAME].log)")
	flag.StringVar(&Name, "name", Name, "name of program")
	flag.BoolVar(&version, "version", version, "print version and exit")
	flag.StringVar(&show, "show", show, "show settings for named component, or 'all'")
	flag.Parse()

	if version {
		fmt.Printf("Version %s\n", Version)
		os.Exit(0)
	}
	if 0 != len(show) {
		golum.Show(show, os.Stdout)
		os.Exit(0)
	}

	if 0 == len(Name) {
		err = errors.New("Program name (-name param) not specified")
		return
	}
	uconfig.ThisProcess = Name

	//
	// verify we have a config file
	//
	if 0 == len(ConfigF) {
		ConfigF = path.Join(InstallD, "config", Name+".yml")
	}
	ConfigF, err = filepath.Abs(ConfigF)
	if err != nil {
		return
	}
	_, err = os.Stat(ConfigF)
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
	ulog.Dir = path.Join(InstallD, "log")
	if 0 == len(ulog.File) && !Testing {
		ulog.File = path.Join(ulog.Dir, Name+".log")
	} else if "stdout" == ulog.File || Testing {
		ulog.File = "stdout"
		ulog.Dir = ""
	}
	if "stdout" != ulog.File {
		ulog.File, err = filepath.Abs(ulog.File)
		if err != nil {
			return err
		}
		ulog.Dir = filepath.Dir(ulog.File)
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

	err = os.Chdir(InstallD)
	if err != nil {
		return err
	}

	return uconfig.InitEnv()
}

//
// Continue boot process: redirect stdin, stdout, stderr and setup logging
//
// if logF is empty, then use configured setting, which may be "stdout"
//
// invoke after Boot()
//
func Redirect(stdoutF, logF string, maxSz int64) (err error) {

	syscall.Close(0) // don't need stdin

	//
	// ulog
	//
	ulog.Init(logF, maxSz)

	if 0 >= maxSz {
		maxSz = 40000000
	}

	//
	// redirect stdout, stderr to file, if necessary
	//
	if 0 != len(stdoutF) && "stdout" != stdoutF && "stdout" != logF {

		stdoutD := ""
		if strings.Contains(stdoutF, "/") || 0 == len(ulog.Dir) {
			stdoutD = path.Dir(stdoutF)
		} else {
			stdoutD = ulog.Dir
			stdoutF = path.Join(stdoutD, stdoutF)
		}
		if !uio.FileExists(stdoutD) {
			if err = os.MkdirAll(stdoutD, 02775); err != nil {
				return uerr.Chainf(err, "problem creating %s", stdoutD)
			}
		}

		fi, err := os.Stat(stdoutF)
		if err == nil && fi.Size() > int64(maxSz) {
			dst := stdoutF + ".last"
			os.Remove(dst)
			os.Rename(stdoutF, dst)
		}
		fd, err := syscall.Open(stdoutF,
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
			time.Now().UTC().Format("2006/01/02 15:04:05Z"), Name, Version)
	}

	log.Printf(`

=================================
Starting
    Name:       %s
    Version:    %s
    InstallDir: %s
    Config:     %s
=================================

`, Name, Version, InstallD, ConfigF)

	return nil
}

//
// Continue boot process: configure from GlobalF (if avail) and ConfigF.
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
// invoke after Redirect()
//
func Configure(
	cspec string,
	beforeStart func(c *uconfig.Section) (err error),
) (
	config *uconfig.Section,
	err error,
) {

	log.Printf("configuring from %s", ConfigF)
	Globals, config, err = uinit.InitConfig(GlobalF, ConfigF)
	if err != nil {
		return
	}

	concurrency := 2
	err = config.GetInt("concurrency", &concurrency)
	if err != nil {
		return
	} else if runtime.GOMAXPROCS(-1) < concurrency+2 {
		runtime.GOMAXPROCS(int(concurrency + 2))
	}
	log.Printf("GOMAXPROCS=%d", runtime.GOMAXPROCS(-1))
	log.Printf("Env: %#v", os.Environ())

	err = loadComponents(config, cspec, beforeStart)
	if err != nil {
		return
	}

	autoreload := false
	err = config.GetBool("autoreload", &autoreload)
	if err != nil {
		return
	} else if autoreload {
		theReloader_ = &reloader_{
			components: cspec,
			interval:   7 * time.Second,
		}
		log.Printf("Starting config reloader")
		theReloader_.Start()
	}
	return
}

//
//
//
func loadComponents(
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
	config.AddSub("logDir", ulog.Dir)
	config.AddSub("name", Name)
	var gconfig *uconfig.Array
	err = config.GetValidArray(cspec, &gconfig)
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
