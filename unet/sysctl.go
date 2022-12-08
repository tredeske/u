package unet

import (
	"bytes"
	"os"
	"strconv"
	"strings"

	"github.com/tredeske/u/uerr"
)

func SysctlGetNetCoreRmemMax() (rv int64, err error) {
	return SysctlGet("net.core.rmem_max")
}

func SysctlGetNetCoreWmemMax() (rv int64, err error) {
	return SysctlGet("net.core.wmem_max")
}

// equivalent to command line sysctl
func SysctlGet(item string) (rv int64, err error) {
	path := "/proc/sys/" + strings.ReplaceAll(item, ".", "/")
	buf, err := os.ReadFile(path)
	if err != nil {
		err = uerr.Chainf(err, "Unable to get %s", item)
		return
	}
	rv, err = strconv.ParseInt(string(bytes.TrimSpace(buf)), 10, 64)
	if err != nil {
		err = uerr.Chainf(err, "Parsing %s value", item)
		return
	}
	return
}
