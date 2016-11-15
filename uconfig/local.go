package uconfig

import (
	nurl "net/url"
	"strings"
)

func IsThisHost(host string) bool {
	if _, ok := LocalAddrs[host]; ok || strings.Contains(host, "localhost") {
		return true
	}
	return false
}

func IsLocalUrl(url *nurl.URL) bool {
	if url.Scheme == "file" && url.Host == "" {
		return true
	}
	host := strings.Split(url.Host, ":")[0]
	return IsThisHost(host)
}
