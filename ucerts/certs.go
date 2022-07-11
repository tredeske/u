package ucerts

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io/ioutil"

	"github.com/tredeske/u/uconfig"
	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/uio"
)

// Does not support password protected PEMs.
func Load(
	privKeyPem, pubCertPem, caCertsPem string,
	tlsc *tls.Config,
) (err error) {

	if 0 != len(privKeyPem) { // if client, may not have a key
		if !uio.FileExists(pubCertPem) {
			err = errors.New("Missing publicCert file " + pubCertPem)
			return
		} else if !uio.FileExists(privKeyPem) {
			err = errors.New("Missing privateKey file " + privKeyPem)
			return
		}
		tlsc.Certificates = make([]tls.Certificate, 1)
		tlsc.Certificates[0], err = tls.LoadX509KeyPair(pubCertPem, privKeyPem)
		if err != nil {
			return uerr.Chainf(err, "Unable to load pubCert (%s) or privKey (%s)",
				pubCertPem, privKeyPem)
		}
	}

	//
	// note: without caCertsPem specified, uses system ca certs
	//
	tlsc.RootCAs, err = LoadRoots(caCertsPem, nil)
	if err != nil {
		return
	}
	if nil == tlsc.ClientCAs {
		tlsc.ClientCAs = tlsc.RootCAs
	}

	/*
		tlsc.CipherSuites = []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
			tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_RSA_WITH_AES_128_GCM_SHA256,

			// CBC not so good, but need for pre tls 1.2
			//
			tls.TLS_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_RSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
			tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
		}
	*/

	//if 0 == tlsc.MinVersion {
	tlsc.MinVersion = tls.VersionTLS12
	tlsc.PreferServerCipherSuites = true
	tlsc.SessionTicketsDisabled = true
	//}
	return
}

func LoadRoots(pem string, roots *x509.CertPool) (rv *x509.CertPool, err error) {

	if 0 != len(pem) {
		if !uio.FileExists(pem) {
			err = errors.New("Missing CA Certs PEM file " + pem)
			return
		}
		var pemBytes []byte
		pemBytes, err = ioutil.ReadFile(pem)
		if err != nil {
			err = uerr.Chainf(err, "Unable to read CA Certs PEM file %s", pem)
			return
		}
		if nil == roots {
			roots = x509.NewCertPool()
		}
		if !roots.AppendCertsFromPEM(pemBytes) {
			err = errors.New("Unable to add CA certs to tls config")
			return
		}
	}
	rv = roots
	return
}

//
// Does the config have TLS certs loaded?
//
func HasTlsCerts(tlsConfig *tls.Config) (rv bool) {
	return nil != tlsConfig && nil != tlsConfig.Certificates
}

//
// for uboot/golum -show
//
func ShowTlsConfig(name string, help *uconfig.Help) {
	p := help
	if 0 != len(name) {
		p = help.Init(name,
			"TLS info")
	}
	p.NewItem("tlsInsecure",
		"bool",
		"(client) If true, do not verify server credentials").
		SetDefault(false)
	p.NewItem("tlsDisableSessionTickets", "bool", "(server) Look it up").
		SetDefault(true)
	p.NewItem("tlsPreferServerCipherSuites", "bool", "(server) Look it up").
		SetDefault(true)
	p.NewItem("tlsClientAuth",
		"string",
		"(server) One of: noClientCert, requestClientCert, requireAnyClientCert, verifyClientCertIfGiven, requireAndVerifyClientCert").
		SetDefault("noClientCert")
	p.NewItem("privateKey", "string", "Path to PEM").SetOptional()
	p.NewItem("publicCert", "string", "Path to PEM").SetOptional()
	p.NewItem("caCerts", "string", "Path to PEM").SetOptional()
	p.NewItem("clientCaCerts", "string", "Path to PEM for validating client certs").
		SetOptional()
	p.NewItem("tlsMin", "string", "One of: 1.0, 1.1, 1.2, 1.3").SetDefault("1.2")
	p.NewItem("tlsMax", "string", "One of: 1.0, 1.1, 1.2, 1.3").SetOptional()
	p.NewItem("tlsServerName", "string", "(client)Name of server").SetOptional()
}

//
// Build a tls.Config
//
func BuildTlsConfig(c *uconfig.Chain) (rv interface{}, err error) {
	var clientCacerts,
		cacerts,
		privateKey,
		publicCert,
		clientAuth,
		serverName,
		tlsMax string
	tlsMin := "1.2"
	insecureSkipVerify := false
	preferServerCipherSuites := true
	sessionTicketsDisabled := true
	if nil != c {
		err = c.
			GetBool("tlsInsecure", &insecureSkipVerify).
			GetBool("tlsDisableSessionTickets", &sessionTicketsDisabled).
			GetBool("tlsPreferServerCiphers", &preferServerCipherSuites).
			GetString("tlsClientAuth", &clientAuth).
			GetString("tlsServerName", &serverName).
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
	tlsConfig := &tls.Config{
		ServerName: serverName,
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
	tlsConfig.InsecureSkipVerify = insecureSkipVerify
	tlsConfig.PreferServerCipherSuites = preferServerCipherSuites
	tlsConfig.SessionTicketsDisabled = sessionTicketsDisabled

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
	rv = tlsConfig
	return
}

func DefaultTlsConfig() (rv *tls.Config) {
	it, err := BuildTlsConfig(nil)
	if err != nil {
		panic(err)
	}
	return it.(*tls.Config)
}
