package uconfig

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tredeske/u/unet"
)

var (
	ThisHost    = ""
	ThisIp      = ""
	ThisProcess = "" // set by boot
	LocalAddrs  = make(map[string]bool)
	ThisD       = ""
	InitD       = initInitD()    // initial dir upon exec
	InstallD    = initInstallD() // where we're installed
	DryRun      = false
)

func initInitD() (dir string) {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return
}

func initInstallD() (dir string) {
	thisProgram, err := filepath.Abs(os.Args[0])
	if err != nil {
		panic(err)
	}
	dir = filepath.Dir(thisProgram)
	ThisD = dir
	if strings.HasSuffix(dir, "/bin") {
		dir = filepath.Dir(dir)
	}
	return
}

// record some info about the environment
func InitEnv() (err error) {
	if 0 != len(ThisHost) {
		return
	}

	ThisHost, err = os.Hostname()
	if err != nil {
		return
	}

	ips, err := unet.FindLocalIps(nil, nil)
	if err != nil {
		return
	}

	for _, ip := range ips {
		names, errLookup := unet.ResolveNames(ip, 100*time.Millisecond)
		if errLookup != nil {
			log.Printf("NOTE: unable to lookup %s."+
				"  Consider adding it to /etc/hosts.  Err: %s", ip, errLookup)
		}
		LocalAddrs[ip.String()] = true
		for _, name := range names {
			LocalAddrs[name] = true
		}
		if 0 == len(ThisIp) && nil == errLookup && 0 != len(names) &&
			!ip.IsLoopback() && !ip.IsMulticast() {
			ThisIp = ip.String()
		}
	}
	if 0 == len(ThisIp) {
		ThisIp = "not-in-etc-hosts"
	}
	return
}
