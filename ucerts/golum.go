package ucerts

import (
	"crypto/tls"
	"errors"
	"fmt"
	"strings"
	"sync"

	"github.com/tredeske/u/golum"
	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/ustrings"
)

var (
	added_    bool
	theCerts_ = certs_{certs: make(map[string]*Cert)}
)

func AddManagers() {
	if !added_ {
		added_ = true
		golum.AddReloadable("certs", &theCerts_)
	}
}

type Cert struct {
	Config      *tls.Config
	Name        string
	PrivateKeyF string
	PublicCertF string
}

// lookup the named cert
func Lookup(name string) *Cert { return theCerts_.Get(name) }

// lookup the name *tls.Config, returning a clone if found, error if not found
func LookupTlsConfig(name string) (*tls.Config, error) {
	cert := theCerts_.Get(name)
	if nil == cert {
		return nil, fmt.Errorf("TLS cert %s not found", name)
	}
	return cert.Config.Clone(), nil
}

// manage loaded certs
type certs_ struct {
	lock  sync.Mutex
	certs map[string]*Cert
}

func (this *certs_) Get(name string) (rv *Cert) {
	this.lock.Lock()
	rv = this.certs[name]
	this.lock.Unlock()
	return
}

func (this *certs_) Start() error { return nil }
func (this *certs_) Stop()        {}

func (this *certs_) Help(name string, help *uconfig.Help) {
	p := help.Init(name, "Manages TLS certs by name")
	certs := p.NewItem("certs", "[]tls.Config", "List of TLS certs.")

	certs.NewItem("name", "string", "Name certs is regestered as")
	ShowTlsConfig("", certs)
}

func (this *certs_) Reload(
	name string,
	c *uconfig.Chain,
) (
	rv golum.Reloadable,
	err error,
) {

	this.lock.Lock()
	defer this.lock.Unlock()

	this.certs = make(map[string]*Cert)

	err = c.
		Each("certs", func(c *uconfig.Chain) (err error) {
			cert := &Cert{}
			return c.
				GetString("name", &cert.Name, uconfig.StringNotBlank()).
				GetString("privateKey", &cert.PrivateKeyF).
				GetString("publicCert", &cert.PublicCertF).
				Build(&cert.Config, BuildTlsConfig).
				ThenCheck(func() (err error) {
					if _, exists := this.certs[cert.Name]; exists {
						return fmt.Errorf("duplicate cert name: %s", cert.Name)
					}
					this.certs[cert.Name] = cert
					return
				}).
				Done()
		}).
		Done()
	if err != nil {
		return
	}
	rv = this
	return
}

// for uboot/golum -show
func ShowTlsConfig(name string, help *uconfig.Help) {
	p := help
	if 0 != len(name) {
		p = help.Init(name,
			"TLS info")
	}
	names12, names13 := cipherNames()
	p.NewItem("tlsInsecure", "bool", `
(client) If true, do not verify server credentials`).Default(false)
	p.NewItem("tlsDisableSessionTickets", "bool", "(server) Look it up").
		Default(false)
	p.NewItem("tlsCiphers", "[]string", fmt.Sprintf(`
Limit ciphers to use for TLS 1.2 and lower, but no effect for TLS 1.3.

RFC 7540 (HTTP/2), section 9.2.2 states that if TLS 1.2 is used, then
TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256 must be on the list.  The go stdlib
enforces this restriction.

RFC 8446 (TLS 1.3), section 9.1 states that TLS_AES_128_GCM_SHA256 must be provided
by all compliant implementations.  The go stdlib enforces this restriction.

TLS 1.3 ciphers (FYI): %s

TLS 1.2 ciphers to choose from: %s`,
		strings.Join(names13, ", "), strings.Join(names12, ", "))).Optional()
	p.NewItem("tlsClientAuth", "string", `
(server) What server should do when client connects:
  - noClientCert
  - requestClientCert
  - requireAnyClientCert
  - verifyClientCertIfGiven
  - requireAndVerifyClientCert`).Default("noClientCert")
	p.NewItem("privateKey", "string", "PEM file containing private key.").
		Optional()
	p.NewItem("publicCert", "string", "PEM file containing public cert.").
		Optional()
	p.NewItem("caCerts", "string", `
(client) PEM file with CA certs client uses to verify server certs.
(server) PEM file with CA certs server uses to verify client certs.`).
		Optional()
	p.NewItem("clientCaCerts", "string", `
(server) PEM file containing CA certs that server should use to verify
client certs.  If 'caCerts' is set, then this defaults to the same PEM.`).
		Optional()
	p.NewItem("tlsMin", "string", `
One of: 1.0, 1.1, 1.2, 1.3
1.3 is recommended for clients.  1.2 is recommended for servers.`).Default("1.2")
	p.NewItem("tlsMax", "string", "One of: 1.0, 1.1, 1.2, 1.3").Default("1.3")
	p.NewItem("tlsServerName", "string", `
(client) Name of server (for TLS SNI, RFC 6066)`).Optional()
}

// Build a tls.Config
//
// The go TLS stdlib no longer supports PreferServerCipherSuites, and also does not
// limit the cipher suites if those are explicitly set on the server side.
func BuildTlsConfig(c *uconfig.Chain) (rv any, err error) {
	var clientCacerts,
		cacerts,
		privateKey,
		publicCert,
		clientAuth string
	var ciphers []string
	names12, _ := cipherNames()
	tlsMin := "1.2"
	tlsMax := "1.3"
	tlsConfig := &tls.Config{}
	if nil != c {
		err = c.
			GetBool("tlsInsecure", &tlsConfig.InsecureSkipVerify).
			GetBool("tlsDisableSessionTickets", &tlsConfig.SessionTicketsDisabled).
			GetStrings("tlsCiphers", &ciphers,
				uconfig.StringOneOf(names12...)).
			GetString("tlsClientAuth", &clientAuth).
			GetString("tlsServerName", &tlsConfig.ServerName).
			GetString("privateKey", &privateKey).
			GetString("publicCert", &publicCert).
			GetString("caCerts", &cacerts).
			GetString("clientCaCerts", &clientCacerts).
			GetString("tlsMin", &tlsMin).
			GetString("tlsMax", &tlsMax).
			Error
		if err != nil {
			return
		}
	}
	if 0 != len(cacerts) || 0 != len(privateKey) {
		err = Load(privateKey, publicCert, cacerts, tlsConfig)
		if err != nil {
			return
		}
	}
	if 0 != len(clientCacerts) {
		tlsConfig.ClientCAs, err = LoadRoots(clientCacerts, nil)
		if err != nil {
			return
		}
	}

	switch clientAuth {
	case "", "noClientCert":
		tlsConfig.ClientAuth = tls.NoClientCert
	case "requestClientCert":
		tlsConfig.ClientAuth = tls.RequestClientCert
	case "requireAnyClientCert":
		tlsConfig.ClientAuth = tls.RequireAnyClientCert
	case "verifyClientCertIfGiven":
		tlsConfig.ClientAuth = tls.VerifyClientCertIfGiven
	case "requireAndVerifyClientCert":
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
	default:
		err = fmt.Errorf("Unknown clientAuth: %s", clientAuth)
		return
	}
	setTlsVersion := func(version string, dst *uint16) (err error) {
		switch version {
		case "": // leave unset, or unchanged from default set above
		case "1.0", "tls1.0":
			*dst = tls.VersionTLS10
		case "1.1", "tls1.1":
			*dst = tls.VersionTLS11
		case "1.2", "tls1.2":
			*dst = tls.VersionTLS12
		case "1.3", "tls1.3":
			*dst = tls.VersionTLS13
		default:
			err = fmt.Errorf("Unknown TLS version: %s", version)
		}
		return
	}
	err = setTlsVersion(tlsMin, &tlsConfig.MinVersion)
	if err != nil {
		return
	}
	err = setTlsVersion(tlsMax, &tlsConfig.MaxVersion)
	if err != nil {
		return
	}
	if 0 != tlsConfig.MaxVersion && tlsConfig.MinVersion > tlsConfig.MaxVersion {
		err = fmt.Errorf("TLS min version, %s, less than max version %s",
			tlsMin, tlsMax)
		return
	}
	if 0 != len(ciphers) {
		if tlsConfig.MinVersion > tls.VersionTLS12 {
			err = errors.New("Cannot set cipher list for TLS 1.3")
			return
		}
		ids := make([]uint16, 0, len(ciphers))
		suites := tls.CipherSuites()
		for _, suite := range suites {
			if ustrings.Contains(ciphers, suite.Name) {
				ids = append(ids, suite.ID)
			}
		}
		if len(ids) != len(ciphers) {
			err = fmt.Errorf("Only found %d of %d ciphers from %#v",
				len(ids), len(ciphers), ciphers)
			return
		}
		tlsConfig.CipherSuites = ids
	}
	rv = tlsConfig
	return
}
