package urest

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
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
	return ShowHttpServerDefault(name, descr, help, nil, nil)
}

// show params available for building http.Server
func ShowHttpServerDefault(
	name, descr string,
	help *uconfig.Help,
	default1 *http.Server,
	default2 *http2.Server,
) *uconfig.Help {
	if nil == default1 {
		default1 = &http.Server{}
	}
	if nil == default2 {
		default2 = &http2.Server{}
	}
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
	p.NewItem("httpIdleTimeout", "duration", `
IdleTimeout is the maximum amount of time to wait for the next request when
keep-alives are enabled. If zero, the value of ReadTimeout is used. If negative,
or if zero and ReadTimeout is zero or negative, there is no timeout.`).
		Default(default1.IdleTimeout)
	p.NewItem("httpReadTimeout", "duration", `
The maximum duration for reading the entire request, including the body. A zero
or negative value means there will be no timeout.

Because ReadTimeout does not let Handlers make per-request decisions on each
request body's acceptable deadline or upload rate, most users will prefer to use
ReadHeaderTimeout. It is valid to use them both.`).
		Default(default1.ReadTimeout)
	p.NewItem("httpReadHeaderTimeout",
		"duration", `
The amount of time allowed to read request headers. The connection's read deadline
is reset after reading the headers and the Handler can decide what is considered
too slow for the body. If zero, the value of ReadTimeout is used. If negative,
or if zero and ReadTimeout is zero or negative, there is no timeout.`).
		Default(default1.ReadHeaderTimeout)
	p.NewItem("httpWriteTimeout",
		"duration", `
The maximum duration before timing out writes of the response.  It is reset
whenever a new request's header is read. Like ReadTimeout, it does not let
Handlers make decisions on a per-request basis.  A zero or negative value means
there will be no timeout.`).
		Default(default1.WriteTimeout)
	p.NewItem("httpKeepAlives",
		"bool",
		"Enable keepalives?").Default("true")

	p.NewItem("http2MaxHandlers", "int", `
Limit the number of http.Handler ServeHTTP goroutines which may run at a time over
all connections.  Negative or zero no limit.`).
		Default(default2.MaxHandlers)
	p.NewItem("http2MaxConcurrentStreams", "int", `
Optionally specify the number of concurrent streams that each client may have
open at a time. This is unrelated to the number of http.Handler goroutines which
may be active globally, which is MaxHandlers.  If zero, defaults to at least 100,
per the HTTP/2 spec's recommendations.`).
		Default(default2.MaxConcurrentStreams)
	p.NewItem("http2MaxDecoderHeaderTableSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2MaxEncoderHeaderTableSize", "int",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2MaxReadFrameSize", "int", `
Optionally specify the largest frame this server is willing to read. A valid
value is between 16k and 16M, inclusive. If zero or otherwise invalid, a default
value is used.`).
		Default(default2.MaxReadFrameSize)
	p.NewItem("http2PermitProhibitedCipherSuites", "bool",
		"see https://pkg.go.dev/golang.org/x/net/http2#Server").Optional()
	p.NewItem("http2IdleTimeout", "duration", `
Specify how long until idle clients should be closed with a GOAWAY frame. PING
frames are not considered activity for the purposes of IdleTimeout.  If zero or
negative, there is no timeout.`).
		Default(default2.IdleTimeout)
	p.NewItem("http2ReadIdleTimeout", "duration", `
The timeout after which a health check using a ping frame will be carried out if
no frame is received on the connection.  If zero, no health check is performed.

If this is not set, then connections may never be cleaned up.`).
		Default(default2.ReadIdleTimeout)
	p.NewItem("http2PingTimeout", "duration", `
The timeout after which the connection will be closed if a response to a ping is
not received.  If zero, a default of 15 seconds is used.`).
		Default(default2.PingTimeout)
	p.NewItem("http2WriteByteTimeout", "duration", `
The timeout after which a connection will be closed if no data can be written to
it. The timeout begins when data is available to write, and is extended whenever
any bytes are written.  If zero or negative, there is no timeout.`).
		Default(default2.WriteByteTimeout)
	p.NewItem("http2MaxUploadBufferPerConnection", "int", `
The size of the initial flow control window for each connection. The HTTP/2 spec
does not allow this to be smaller than 65535 or larger than 2^32-1.  If the
value is outside this range, a default value will be used instead.`).
		Default(default2.MaxUploadBufferPerConnection)
	p.NewItem("http2MaxUploadBufferPerStream", "int", `
The size of the initial flow control window for each stream. The HTTP/2 spec does
not allow this to be larger than 2^32-1. If the value is zero or larger than the
maximum, a default value will be used instead.`).
		Default(default2.MaxUploadBufferPerStream)
	p.NewItem("tlsCert", "string", "Name of TLS cert to use").Optional()
	return p
}

type Option func(*Options) error

type Options struct {
	maxVersion uint8
	default1   *http.Server
	default2   *http2.Server
}

func WithDefaults(server1 *http.Server, server2 *http2.Server) Option {
	return func(opts *Options) error {
		opts.default1 = server1
		opts.default2 = server2
		return nil
	}
}

func MaxHttpVersion(version string) Option {
	return func(opts *Options) error {
		switch version {
		case "1", "1.0":
			opts.maxVersion = 10
		case "1.1":
			opts.maxVersion = 11
		case "2", "2.0":
			opts.maxVersion = 20
		case "3", "3.0":
			opts.maxVersion = 30
		default:
			return fmt.Errorf("HTTP version (%s) not one of 1, 1.1, 2, 3",
				version)
		}
		return nil
	}
}

// obtain a HTTP Server builder for uconfig
//
// Use:
//
//	var c *uconfig.Chain
//	var httpServer *http.Server
//	builder, err = urest.ServerBuilder(MaxHttpVerion("1.1"))
//	if err != nil { ... }
//	err = c.Build(&httpServer, builder). ... .Done()
func ServerBuilder(options ...Option) (b uconfig.Builder, err error) {
	opts := &Options{
		maxVersion: 20,
	}
	for _, option := range options {
		err = option(opts)
		if err != nil {
			return
		}
	}
	return opts.buildHttpServer, nil
}

// implement uconfig.Builder, return built http.Server
func (opts *Options) buildHttpServer(c *uconfig.Chain) (rv any, err error) {
	server1 := &http.Server{}
	if nil != opts.default1 {
		*server1 = *opts.default1
	}
	var s2 *http2.Server
	if opts.maxVersion >= 20 {
		s2 = &http2.Server{}
		if nil != opts.default2 {
			*s2 = *opts.default2
		}
	}
	if nil == c {
		if nil != s2 {
			err = http2.ConfigureServer(server1, s2)
		}
		return server1, err
	}

	keepAlives := true
	var tlsCertN string
	err = c.
		GetString("tlsCert", &tlsCertN).
		ThenCheck(func() (err error) {
			if 0 != len(tlsCertN) {
				server1.TLSConfig, err = ucerts.LookupTlsConfig(tlsCertN)
			}
			return
		}).
		GetString("httpAddress", &server1.Addr).
		GetBool("httpDisableOptionsHandler",
			&server1.DisableGeneralOptionsHandler).
		GetDuration("httpIdleTimeout", &server1.IdleTimeout).
		GetDuration("httpReadTimeout", &server1.ReadTimeout).
		GetDuration("httpReadHeaderTimeout", &server1.ReadHeaderTimeout).
		GetDuration("httpWriteTimeout", &server1.WriteTimeout).
		GetInt("httpMaxHeaderBytes", &server1.MaxHeaderBytes).
		GetBool("httpKeepAlives", &keepAlives).
		IfHasKeysMatching(func(c *uconfig.Chain) (err error) {
			if opts.maxVersion < 20 {
				return errors.New("HTTP/2 not allowed here")
			}
			if nil == s2 {
				s2 = &http2.Server{}
			}
			err = c.
				GetInt("http2MaxHandlers", &s2.MaxHandlers).
				GetUInt("http2MaxConcurrentStreams", &s2.MaxConcurrentStreams).
				GetUInt("http2MaxDecoderHeaderTableSize",
					&s2.MaxDecoderHeaderTableSize).
				GetUInt("http2MaxEncoderHeaderTableSize",
					&s2.MaxEncoderHeaderTableSize).
				GetUInt("http2MaxReadFrameSize", &s2.MaxReadFrameSize,
					uconfig.UIntZeroOr(uconfig.UIntRange(1<<14, 1<<24))).
				GetBool("http2PermitProhibitedCipherSuites",
					&s2.PermitProhibitedCipherSuites).
				GetDuration("http2IdleTimeout", &s2.IdleTimeout).
				GetDuration("http2ReadIdleTimeout", &s2.ReadIdleTimeout).
				GetDuration("http2PingTimeout", &s2.PingTimeout).
				GetDuration("http2WriteByteTimeout", &s2.WriteByteTimeout).
				GetInt("http2MaxUploadBufferPerConnection",
					&s2.MaxUploadBufferPerConnection,
					uconfig.IntZeroOr(uconfig.IntRange(65535, (1<<32)-1))).
				GetInt("http2MaxUploadBufferPerStream",
					&s2.MaxUploadBufferPerStream, uconfig.IntRange(0, (1<<32)-1)).
				Error
			return
		}, regexp.MustCompile("^http2")).
		Error
	if err != nil {
		return
	}
	if !keepAlives {
		server1.SetKeepAlivesEnabled(keepAlives)
	}
	if nil != s2 {
		err = http2.ConfigureServer(server1, s2)
		if err != nil {
			return
		}
	}
	rv = server1
	return
}

// build a http.Server using uconfig and default options
//
// if c is nil, then build default http.Server
func BuildHttpServer(c *uconfig.Chain) (rv any, err error) {
	b, err := ServerBuilder()
	if err != nil {
		return
	}
	return b(c)
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
