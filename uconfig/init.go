package uconfig

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
)

var (
	ThisHost    = ""
	ThisIp      = ""
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

		//
		// this may take a while...
		//
		// to avoid delays, ensure /etc/hosts can resolve all interfaces
		//
		names, errLookup := net.LookupAddr(addr)
		if errLookup == nil && 0 != len(names) {
			LocalAddrs[addr] = true
			for _, name := range names {
				LocalAddrs[name] = true
			}
		}
	}

	err = findLocalIp()
	return
}

func findLocalIp() (err error) {
	interfaces, err := net.Interfaces()
	if err != nil {
		return
	}
	for _, in := range interfaces {
		if "lo" == in.Name {
			continue
		}
		var addrs []net.Addr
		addrs, err = in.Addrs()
		if err != nil {
			return
		} else if 0 == len(addrs) {
			continue
		}
		addr, ok := addrs[0].(*net.IPNet)
		if !ok {
			err = fmt.Errorf("Did not get back expected addr type: %T", addrs[0])
			return
		}
		ThisIp = addr.IP.String()
		return ///////////////////////////// success
	}
	err = errors.New("Unable to determine src addr")
	return
}
