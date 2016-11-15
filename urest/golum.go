package urest

import (
	"net"
	"net/http"
	"time"

	"github.com/tredeske/u/ucerts"
	"github.com/tredeske/u/uconfig"
)

//
// show params available for building http.Client
//
func ShowHttpClient(name string, help *uconfig.Help) {
	p := help
	if 0 != len(name) {
		p = help.Init(name,
			"HTTP client endpoint info")
	}
	p.NewItem("httpDisableCompression",
		"bool",
		"Turn off compression").Set("default", false)
	p.NewItem("httpMaxIdleConnsPerHost",
		"int",
		"Max conns to keep around just in case").SetOptional()
	p.NewItem("httpResponseTimeout",
		"duration",
		"How long to wait for a response").SetOptional()
	p.NewItem("tlsHandshakeTimeout",
		"duration",
		"How long to wait for TLS init").Set("default", "17s")
	p.NewItem("tcpTimeout",
		"duration",
		"Detect unable to connect after").Set("default", "67s")
	p.NewItem("tcpKeepAlive",
		"duration",
		"Detect broken conn after no keepalives").Set("default", "67s")
	ucerts.ShowTlsConfig("", p)
}

//
// build a http.Client using uconfig
//
// if c is nil, then build default http.Client
//
func BuildHttpClient(c *uconfig.Chain) (rv interface{}, err error) {
	dialer := &net.Dialer{
		Timeout:   67 * time.Second,
		KeepAlive: 67 * time.Second,
	}
	httpTransport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: 17 * time.Second,
	}
	httpClient := &http.Client{
		Transport: httpTransport,
	}

	if nil != c {
		err = c.
			Build(&httpTransport.TLSClientConfig, ucerts.BuildTlsConfig).
			GetBool("httpDisableCompression", &httpTransport.DisableCompression).
			GetInt("httpMaxIdleConnsPerHost", &httpTransport.MaxIdleConnsPerHost).
			GetDuration("httpResponseTimeout", &httpTransport.ResponseHeaderTimeout).
			GetDuration("tlsHandshakeTimeout", &httpTransport.TLSHandshakeTimeout).
			GetDuration("tcpTimeout", &dialer.Timeout).
			GetDuration("tcpKeepAlive", &dialer.KeepAlive).
			Error
		if err != nil {
			return
		}
	} else {
		httpTransport.TLSClientConfig = ucerts.DefaultTlsConfig()
	}
	httpTransport.Dial = (dialer).Dial
	rv = httpClient
	return
}

func DefaultHttpClient() (rv *http.Client) {
	it, err := BuildHttpClient(nil)
	if err != nil {
		panic(err)
	}
	return it.(*http.Client)
}
