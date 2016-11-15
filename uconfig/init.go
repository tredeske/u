package uconfig

import (
	"net"
	"os"
	"path/filepath"
	"strings"
)

var (
	ThisHost    = ""
	ThisProcess = "" // set by boot
	LocalAddrs  = make(map[string]bool)
	ThisD       = ""
	InitD       = initInitD()    // initial dir upon exec
	InstallD    = initInstallD() // where we're installed
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

//
// record some info about the environment
//
func InitEnv() (err error) {
	if 0 != len(ThisHost) {
		return
	}

	ThisHost, err = os.Hostname()
	if err != nil {
		return
	}

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return
	}
	LocalAddrs[ThisHost] = true
	LocalAddrs["localhost"] = true
	for _, a := range addrs {
		addr := strings.Split(a.String(), "/")[0]
		names, err := net.LookupAddr(addr)
		if err == nil && 0 != len(names) {
			LocalAddrs[addr] = true
			for _, name := range names {
				LocalAddrs[name] = true
			}
		}
	}
	return nil
}
