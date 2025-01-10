package urest

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/tredeske/u/ucerts"
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ulog"
	"golang.org/x/net/http2"
)

var defaultDialer_ = &net.Dialer{
	Timeout:   67 * time.Second,
	KeepAlive: 67 * time.Second,
}

// show params available for building http.Client
func ShowHttpClient(name, descr string, help *uconfig.Help) *uconfig.Help {
	tp := DefaultHttpTransport()
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
		Default(tp.DisableCompression)

	p.NewItem("httpDisableKeepAlives",
		"bool",
		"Turn off reuse of connections").
		Default(tp.DisableKeepAlives)

	p.NewItem("httpExpectContinueTimeout",
		"duration",
		"If not zero, emit Expect: 100-continue and wait to send body").
		Default(tp.ExpectContinueTimeout)

	p.NewItem("httpIdleConnTimeout",
		"duration",
		"How long to keep idle connections around").
		Default(tp.IdleConnTimeout)

	p.NewItem("httpMaxIdleConns",
		"int",
		"Max conns to keep around just in case").
		Default(tp.MaxIdleConns)

	p.NewItem("httpMaxIdleConnsPerHost",
		"int",
		"Max conns to keep around just in case").
		Default(tp.MaxIdleConnsPerHost)

	p.NewItem("httpResponseTimeout",
		"duration",
		"How long to wait for a response (not incl body)").
		Default(tp.ResponseHeaderTimeout)

	p.NewItem("tcpKeepAlive", "duration",
		"Detect broken conn after no keepalives").
		Default(defaultDialer_.KeepAlive)

	p.NewItem("tcpTimeout",
		"duration",
		"Detect unable to connect after").
		Default(defaultDialer_.Timeout)

	p.NewItem("tlsHandshakeTimeout",
		"duration",
		"How long to wait for TLS init").
		Default(tp.TLSHandshakeTimeout)

	p.NewItem("useHttp2", "bool",
		"Force use of HTTP2 even if no HTTP2 parameters are set").
		Default(tp.ForceAttemptHTTP2)
	p.NewItem("http2DisableCompression", "bool",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").Optional()
	p.NewItem("http2AllowHttp", "bool",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").Optional()
	p.NewItem("http2MaxHeaderListSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").Optional()
	p.NewItem("http2MaxReadFrameSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").Optional()
	p.NewItem("http2MaxDecoderHeaderTableSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").Optional()
	p.NewItem("http2MaxEncoderHeaderTableSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").Optional()
	p.NewItem("http2StrictMaxConcurrentStreams", "bool",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").Optional()
	p.NewItem("http2ReadIdleTimeout", "duration",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").Default("37s")
	p.NewItem("http2PingTimeout", "duration",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").Optional()
	p.NewItem("http2WriteByteTimeout", "duration",
		"see https://pkg.go.dev/golang.org/x/net/http2#Transport").Optional()

	//ucerts.ShowTlsConfig("", p)
	p.NewItem("tlsCert", "string", "Name of TLS cert to use").Optional()
	return p
}

// build a http.Client using uconfig
//
// if c is nil, then build default http.Client
func BuildHttpClient(c *uconfig.Chain) (rv any, err error) {

	dialer := &net.Dialer{}
	*dialer = *defaultDialer_
	tp := DefaultHttpTransport()
	tp.DialContext = dialer.DialContext
	if nil != c {
		var t2 *http2.Transport
		var tlsCertN string
		err = c.
			GetString("tlsCert", &tlsCertN).
			ThenCheck(func() (err error) {
				if 0 != len(tlsCertN) {
					tp.TLSClientConfig, err = ucerts.LookupTlsConfig(tlsCertN)
				}
				return
			}).
			GetBool("httpDisableCompression", &tp.DisableCompression).
			GetBool("httpDisableKeepAlives", &tp.DisableKeepAlives).
			GetInt("httpMaxIdleConnsPerHost", &tp.MaxIdleConnsPerHost).
			GetInt("httpMaxIdleConns", &tp.MaxIdleConns).
			GetDuration("httpResponseTimeout", &tp.ResponseHeaderTimeout).
			GetDuration("httpIdleConnTimeout", &tp.IdleConnTimeout).
			GetDuration("httpExpectContinueTimeout", &tp.ExpectContinueTimeout).
			GetDuration("tlsHandshakeTimeout", &tp.TLSHandshakeTimeout).
			GetDuration("tcpTimeout", &dialer.Timeout).
			GetDuration("tcpKeepAlive", &dialer.KeepAlive).
			GetBool("useHttp2", &tp.ForceAttemptHTTP2).
			IfHasKeysMatching( // special http2 config settings
				func(c *uconfig.Chain) (err error) {
					t2, err = http2.ConfigureTransports(tp)
					t2.ReadIdleTimeout = 37 * time.Second
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
		if tp.ForceAttemptHTTP2 && nil == t2 {
			//
			// no special http2 settings are present, but we were told to ensure
			// http2 would be used
			//
			t2, err = http2.ConfigureTransports(tp)
			if err != nil {
				return
			}
			t2.ReadIdleTimeout = 37 * time.Second
		}
	} else {
		tp.TLSClientConfig = ucerts.DefaultTlsConfig()
	}
	rv = &http.Client{
		Transport: tp,
	}
	return
}

func CloneHttpClient(prototype *http.Client) (rv *http.Client) {
	rv = &http.Client{}
	*rv = *prototype
	rv.Transport = (prototype.Transport.(*http.Transport)).Clone()
	return
}

func DefaultHttpTransport() (rv *http.Transport) {
	rv = (http.DefaultTransport.(*http.Transport)).Clone()
	rv.DialContext = defaultDialer_.DialContext
	return
}

func DefaultHttpClient() (rv *http.Client) {
	return &http.Client{
		Transport: DefaultHttpTransport(),
	}
}

func DefaultHttp2Client() (rv *http.Client) {
	tp1 := DefaultHttpTransport()
	rv = &http.Client{
		Transport: tp1,
	}
	tp2, err := http2.ConfigureTransports(tp1)
	if err != nil {
		panic(err)
	}
	tp2.ReadIdleTimeout = 37 * time.Second
	return
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
		Optional()
	p.NewItem("httpDisableOptionsHandler",
		"bool",
		"If true, pass OPTIONS to handler instead of always responding 200").
		Optional()
	p.NewItem("httpMaxHeaderBytes",
		"int",
		"Max number of bytes allowed in request headers").Optional()
	p.NewItem("httpIdleTimeout",
		"duration",
		"How long to wait for next req").Optional()
	p.NewItem("httpReadTimeout",
		"duration",
		"Max time to read entire request").Optional()
	p.NewItem("httpReadHeaderTimeout",
		"duration",
		"Max time to read headers in request").Optional()
	p.NewItem("httpWriteTimeout",
		"duration",
		"How long to wait for client to accept response").Optional()
	p.NewItem("httpKeepAlives",
		"bool",
		"Enable keepalives?").Default("true")
	p.NewItem("http2MaxHandlers", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2MaxConcurrentStreams", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2MaxDecoderHeaderTableSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2MaxEncoderHeaderTableSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2MaxReadFrameSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2PermitProhibitedCipherSuites", "bool",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2IdleTimeout", "duration",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2ReadIdleTimeout", "duration",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Default("37s")
	p.NewItem("http2PingTimeout", "duration",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2WriteByteTimeout", "duration",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Default("37s")
	p.NewItem("http2MaxUploadBufferPerConnection", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2MaxUploadBufferPerStream", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("tlsCert", "string", "Name of TLS cert to use").Optional()
	return p
}

// build a http.Server using uconfig
//
// if c is nil, then build default http.Server
func BuildHttpServer(c *uconfig.Chain) (rv any, err error) {
	httpServer := &http.Server{}
	if nil == c {
		return httpServer, nil
	}

	keepAlives := true
	http2Configured := false
	var tlsCertN string
	err = c.
		GetString("tlsCert", &tlsCertN).
		ThenCheck(func() (err error) {
			if 0 != len(tlsCertN) {
				httpServer.TLSConfig, err = ucerts.LookupTlsConfig(tlsCertN)
			}
			return
		}).
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
				s2 := http2.Server{
					ReadIdleTimeout:  37 * time.Second,
					WriteByteTimeout: 37 * time.Second,
				}
				http2Configured = true
				err = c.
					GetInt("http2MaxHandlers", &s2.MaxHandlers).
					GetUInt("http2MaxConcurrentStreams", &s2.MaxConcurrentStreams).
					GetUInt("http2MaxDecoderHeaderTableSize",
						&s2.MaxDecoderHeaderTableSize).
					GetUInt("http2MaxEncoderHeaderTableSize",
						&s2.MaxEncoderHeaderTableSize).
					GetUInt("http2MaxReadFrameSize", &s2.MaxReadFrameSize).
					GetBool("http2PermitProhibitedCipherSuites",
						&s2.PermitProhibitedCipherSuites).
					GetDuration("http2IdleTimeout", &s2.IdleTimeout).
					GetDuration("http2ReadIdleTimeout", &s2.ReadIdleTimeout).
					GetDuration("http2PingTimeout", &s2.PingTimeout).
					GetDuration("http2WriteByteTimeout", &s2.WriteByteTimeout).
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
	if !http2Configured {
		err = http2.ConfigureServer(httpServer,
			&http2.Server{
				// enable pings to help http/2 conn cleanup
				ReadIdleTimeout:  37 * time.Second,
				WriteByteTimeout: 37 * time.Second,
			})
		if err != nil {
			return
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
			var l net.Listener
			l, err = tls.Listen("tcp", svr.Addr, svr.TLSConfig)
			if nil == err {
				err = svr.Serve(l)
			}
			//err = svr.ListenAndServeTLS("", "")
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
