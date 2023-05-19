package urest

import (
	"context"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/tredeske/u/ucerts"
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
	"golang.org/x/net/http2"
)

// show params available for building http.Client
func ShowHttpClient(name, descr string, help *uconfig.Help) *uconfig.Help {
	p := help
	if 0 != len(name) {
		if 0 == len(descr) {
			descr = "HTTP client endpoint"
		}
		p = help.Init(name, descr)
	}
	p.NewItem("httpDisableCompression",
		"bool",
		"Turn off compression").
		SetDefault(false)

	p.NewItem("httpDisableKeepAlives",
		"bool",
		"Turn off reuse of connections").
		SetDefault(false)

	p.NewItem("httpExpectContinueTimeout",
		"duration",
		"If not zero, emit Expect: 100-continue and wait to send body").
		SetDefault("1s")

	p.NewItem("httpIdleConnTimeout",
		"duration",
		"How long to keep idle connections around").
		SetDefault("90s")

	p.NewItem("httpMaxIdleConns",
		"int",
		"Max conns to keep around just in case").
		SetDefault("128")

	p.NewItem("httpMaxIdleConnsPerHost",
		"int",
		"Max conns to keep around just in case").
		SetDefault("64")

	p.NewItem("httpResponseTimeout",
		"duration",
		"How long to wait for a response (not incl body)").
		SetDefault("forever")

	p.NewItem("tcpKeepAlive", "duration",
		"Detect broken conn after no keepalives").
		SetDefault("67s")

	p.NewItem("tcpTimeout",
		"duration",
		"Detect unable to connect after").
		SetDefault("67s")

	p.NewItem("tlsHandshakeTimeout",
		"duration",
		"How long to wait for TLS init").
		SetDefault("17s")

	p.NewItem("useHttp2", "bool",
		"Force use of HTTP2 even if no HTTP2 parameters are set").SetOptional()
	p.NewItem("http2DisableCompression", "bool",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").SetOptional()
	p.NewItem("http2AllowHttp", "bool",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").SetOptional()
	p.NewItem("http2MaxHeaderListSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").SetOptional()
	p.NewItem("http2MaxReadFrameSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").SetOptional()
	p.NewItem("http2MaxDecoderHeaderTableSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").SetOptional()
	p.NewItem("http2MaxEncoderHeaderTableSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").SetOptional()
	p.NewItem("http2StrictMaxConcurrentStreams", "bool",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").SetOptional()
	p.NewItem("http2ReadIdleTimeout", "duration",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").SetOptional()
	p.NewItem("http2PingTimeout", "duration",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").SetOptional()
	p.NewItem("http2WriteByteTimeout", "duration",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").SetOptional()

	ucerts.ShowTlsConfig("", p)
	return p
}

// build a http.Client using uconfig
//
// if c is nil, then build default http.Client
func BuildHttpClient(c *uconfig.Chain) (rv any, err error) {

	//
	// wish we could do this, but there is a mutex that gets copied
	//
	//    *httpTransport = *(http.DefaultTransport.(*http.Transport))
	//
	// as it is, as this structures change, we'll need to revisit this
	// code once in a while
	//
	// refer to http.DefaultTransport
	//
	dialer := &net.Dialer{
		Timeout:   67 * time.Second,
		KeepAlive: 67 * time.Second,
		DualStack: true,
	}

	httpTransport := &http.Transport{
		Dial:                  dialer.Dial, // unclear if needed for TLS
		DialContext:           dialer.DialContext,
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          128,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   17 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	if nil != c {
		var t2 *http2.Transport
		var useHttp2 bool
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
			GetBool("useHttp2", &useHttp2).
			IfHasKeysMatching( // special http2 config settings
				func(c *uconfig.Chain) (err error) {
					t2, err = http2.ConfigureTransports(httpTransport)
					if err != nil {
						return
					}
					return c.
						GetBool("http2DisableCompression", &t2.DisableCompression).
						GetBool("http2AllowHttp", &t2.AllowHTTP).
						GetUInt("http2MaxHeaderListSize", &t2.MaxHeaderListSize).
						GetUInt("http2MaxReadFrameSize", &t2.MaxReadFrameSize).
						GetUInt("http2MaxDecoderHeaderTableSize",
							&t2.MaxDecoderHeaderTableSize).
						GetUInt("http2MaxEncoderHeaderTableSize",
							&t2.MaxEncoderHeaderTableSize).
						GetBool("http2StrictMaxConcurrentStreams",
							&t2.StrictMaxConcurrentStreams).
						GetDuration("http2ReadIdleTimeout", &t2.ReadIdleTimeout).
						GetDuration("http2PingTimeout", &t2.PingTimeout).
						GetDuration("http2WriteByteTimeout", &t2.WriteByteTimeout).
						Error
				}, regexp.MustCompile("^http2")).
			Error
		if err != nil {
			return
		}
		if useHttp2 && nil == t2 {
			//
			// no special http2 settings are present, but we were told to ensure
			// http2 would be used
			//
			err = http2.ConfigureTransport(httpTransport)
			if err != nil {
				return
			}
		}
	} else {
		httpTransport.TLSClientConfig = ucerts.DefaultTlsConfig()
	}
	rv = &http.Client{
		Transport: httpTransport,
	}
	return
}

func DefaultHttpClient() (rv *http.Client) {
	it, err := BuildHttpClient(nil)
	if err != nil {
		panic(err)
	}
	return it.(*http.Client)
}

// show params available for building http.Server
func ShowHttpServer(name, descr string, help *uconfig.Help) *uconfig.Help {
	p := help
	if 0 != len(name) {
		if 0 == len(descr) {
			descr = "HTTP server endpoint info"
		}
		p = help.Init(name, descr)
	}
	p.NewItem("httpAddress",
		"string",
		"host:port to listen on.  if not set, endpoint is disabled.").
		SetOptional()
	p.NewItem("httpDisableOptionsHandler",
		"bool",
		"If true, pass OPTIONS to handler instead of always responding 200").
		SetOptional()
	p.NewItem("httpMaxHeaderBytes",
		"int",
		"Max number of bytes allowed in request headers").SetOptional()
	p.NewItem("httpIdleTimeout",
		"duration",
		"How long to wait for next req").SetOptional()
	p.NewItem("httpReadTimeout",
		"duration",
		"Max time to read entire request").SetOptional()
	p.NewItem("httpReadHeaderTimeout",
		"duration",
		"Max time to read headers in request").SetOptional()
	p.NewItem("httpWriteTimeout",
		"duration",
		"How long to wait for client to accept response").SetOptional()
	p.NewItem("httpKeepAlives",
		"bool",
		"Enable keepalives?").Default("true")
	p.NewItem("http2MaxHandlers", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").SetOptional()
	p.NewItem("http2MaxConcurrentStreams", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").SetOptional()
	p.NewItem("http2MaxDecoderHeaderTableSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").SetOptional()
	p.NewItem("http2MaxEncoderHeaderTableSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").SetOptional()
	p.NewItem("http2MaxReadFrameSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").SetOptional()
	p.NewItem("http2PermitProhibitedCipherSuites", "bool",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").SetOptional()
	p.NewItem("http2IdleTimeout", "duration",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").SetOptional()
	p.NewItem("http2MaxUploadBufferPerConnection", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").SetOptional()
	p.NewItem("http2MaxUploadBufferPerStream", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").SetOptional()
	ucerts.ShowTlsConfig("", p)
	return p
}

// build a http.Server using uconfig
//
// if c is nil, then build default http.Server
func BuildHttpServer(c *uconfig.Chain) (rv any, err error) {
	httpServer := &http.Server{}
	keepAlives := true

	if nil != c {
		err = c.
			Build(&httpServer.TLSConfig, ucerts.BuildTlsConfig).
			GetString("httpAddress", &httpServer.Addr).
			GetBool("httpDisableOptionsHandler",
				&httpServer.DisableGeneralOptionsHandler).
			GetDuration("httpIdleTimeout", &httpServer.IdleTimeout).
			GetDuration("httpReadTimeout", &httpServer.ReadTimeout).
			GetDuration("httpReadHeaderTimeout", &httpServer.ReadHeaderTimeout).
			GetDuration("httpWriteTimeout", &httpServer.WriteTimeout).
			GetInt("httpMaxHeaderBytes", &httpServer.MaxHeaderBytes).
			GetBool("httpKeepAlives", &keepAlives).
			IfHasKeysMatching(
				func(c *uconfig.Chain) (err error) {
					s2 := http2.Server{}
					err = c.
						GetInt("http2MaxHandlers", &s2.MaxHandlers).
						GetUInt("http2MaxConcurrentStreams",
							&s2.MaxConcurrentStreams).
						GetUInt("http2MaxConcurrentStreams",
							&s2.MaxConcurrentStreams).
						GetUInt("http2MaxDecoderHeaderTableSize",
							&s2.MaxDecoderHeaderTableSize).
						GetUInt("http2MaxEncoderHeaderTableSize",
							&s2.MaxEncoderHeaderTableSize).
						GetUInt("http2MaxReadFrameSize", &s2.MaxReadFrameSize).
						GetBool("http2PermitProhibitedCipherSuites",
							&s2.PermitProhibitedCipherSuites).
						GetDuration("http2IdleTimeout", &s2.IdleTimeout).
						GetInt("http2MaxUploadBufferPerConnection",
							&s2.MaxUploadBufferPerConnection).
						GetInt("http2MaxUploadBufferPerStream",
							&s2.MaxUploadBufferPerStream).
						Error
					if err != nil {
						return
					}
					return http2.ConfigureServer(httpServer, &s2)
				}, regexp.MustCompile("^http2")).
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

// is the httpServer configured for TLS?
func IsTlsServer(s *http.Server) bool {
	return nil != s && ucerts.HasTlsCerts(s.TLSConfig)
}

// start listening on a server
func StartServer(svr *http.Server, onDone func(err error)) {
	go func() {

		if nil == onDone {
			onDone = func(err error) {
				if err != nil {
					ulog.Errorf("Serve addr=%s failed: %s", svr.Addr, err)
				} else {
					ulog.Printf("No longer serving %s", svr.Addr)
				}
			}
		}

		//
		// start serving reqs
		//

		var err error
		if IsTlsServer(svr) {
			err = svr.ListenAndServeTLS("", "")
		} else {
			err = svr.ListenAndServe()
		}

		//
		// when we're told to stop, we may get a spurious error
		//
		if http.ErrServerClosed == err {
			err = nil
		}
		onDone(err)
	}()
}

func StopServer(svr *http.Server, grace time.Duration) {
	if nil != svr {
		if 0 == grace {
			svr.Close()
		} else {
			ctx, cancel := context.WithTimeout(context.Background(), grace)
			svr.Shutdown(ctx)
			<-ctx.Done() // wait here
			cancel()
		}
	}
}
