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
	p.NewItem("httpDisableKeepAlives",
		"bool",
		"Turn off reuse of connections").Set("default", false)
	p.NewItem("httpMaxIdleConns",
		"int",
		"Max conns to keep around just in case").SetOptional()
	p.NewItem("httpMaxIdleConnsPerHost",
		"int",
		"Max conns to keep around just in case").SetOptional()
	p.NewItem("httpResponseTimeout",
		"duration",
		"How long to wait for a response (not incl body)").SetOptional()
	p.NewItem("httpIdleConnTimeout",
		"duration",
		"How long to keep idle connections around").SetOptional()
	p.NewItem("httpExpectContinueTimeout",
		"duration",
		"If not zero, emit Expect: 100-continue and wait to send body").SetOptional()
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
	httpTransport := &http.Transport{}
	*httpTransport = *(http.DefaultTransport.(*http.Transport))
	httpTransport.TLSHandshakeTimeout = 17 * time.Second
	/*
		httpTransport := &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			TLSHandshakeTimeout: 17 * time.Second,
		}
	*/
	httpClient := &http.Client{
		Transport: httpTransport,
	}

	if nil != c {
		err = c.
			Build(&httpTransport.TLSClientConfig, ucerts.BuildTlsConfig).
			GetBool("httpDisableCompression", &httpTransport.DisableCompression).
			GetBool("httpDisableKeepAlives", &httpTransport.DisableKeepAlives).
			GetInt("httpMaxIdleConnsPerHost", &httpTransport.MaxIdleConnsPerHost).
			GetInt("httpMaxIdleConns", &httpTransport.MaxIdleConns).
			GetDuration("httpResponseTimeout", &httpTransport.ResponseHeaderTimeout).
			GetDuration("httpIdleConnTimeout", &httpTransport.IdleConnTimeout).
			GetDuration("httpExpectContinueTimeout", &httpTransport.ExpectContinueTimeout).
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

//
// show params available for building http.Server
//
func ShowHttpServer(name string, help *uconfig.Help) {
	p := help
	if 0 != len(name) {
		p = help.Init(name,
			"HTTP server endpoint info")
	}
	p.NewItem("httpAddress",
		"string",
		"host:port to listen on.  if not set, endpoint is disabled.").SetOptional()
	p.NewItem("httpMaxHeaderBytes",
		"int",
		"Max number of bytes allowed in request headers").SetOptional()
	p.NewItem("httpReadTimeout",
		"duration",
		"Max time to read entire request").SetOptional()
	p.NewItem("httpReadHeaderTimeout",
		"duration",
		"Max time to read headers in request").SetOptional()
	p.NewItem("httpWriteTimeout",
		"duration",
		"How long to wait for client to accept response").SetOptional()
	p.NewItem("httpIdleTimeout",
		"duration",
		"How long to wait for next req").SetOptional()
	p.NewItem("httpKeepAlives",
		"bool",
		"Enable keepalives?").Set("default", "true")
	ucerts.ShowTlsConfig("", p)
}

//
// build a http.Server using uconfig
//
// if c is nil, then build default http.Server
//
func BuildHttpServer(c *uconfig.Chain) (rv interface{}, err error) {
	httpServer := &http.Server{}
	keepAlives := true

	if nil != c {
		err = c.
			Build(&httpServer.TLSConfig, ucerts.BuildTlsConfig).
			GetValidString("httpAddress", "", &httpServer.Addr).
			GetDuration("httpReadTimeout", &httpServer.ReadTimeout).
			GetDuration("httpWriteTimeout", &httpServer.WriteTimeout).

			// TODO
			//
			//GetDuration("httpReadHeaderTimeout", &httpServer.ReadHeaderTimeout).
			//GetDuration("httpIdleTimeout", &httpServer.IdleTimeout).
			//

			GetInt("httpMaxHeaderBytes", &httpServer.MaxHeaderBytes).
			GetBool("httpKeepAlives", &keepAlives).
			Error
		if err != nil {
			return
		}
		if !keepAlives {
			httpServer.SetKeepAlivesEnabled(keepAlives)
		}
	}
	rv = httpServer
	return
}

//
// is the httpServer configured for TLS?
//
func IsTlsServer(s *http.Server) bool {
	return nil != s && ucerts.HasTlsCerts(s.TLSConfig)
}
