package ucerts

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"os"

	"github.com/tredeske/u/uerr"
	"github.com/tredeske/u/uio"
)

func LoadKeyAndCert(privKeyPem, pubCertPem string, tlsc *tls.Config) (err error) {
	if !uio.FileExists(pubCertPem) {
		err = errors.New("Missing publicCert file " + pubCertPem)
	} else if !uio.FileExists(privKeyPem) {
		err = errors.New("Missing privateKey file " + privKeyPem)
	} else {
		tlsc.Certificates = make([]tls.Certificate, 1)
		tlsc.Certificates[0], err = tls.LoadX509KeyPair(pubCertPem, privKeyPem)
		if err != nil {
			err = uerr.Chainf(err, "Unable to load pubCert (%s) or privKey (%s)",
				pubCertPem, privKeyPem)
		}
	}
	return
}

func LoadCaCerts(caCertsPem string, tlsc *tls.Config) (err error) {
	if 0 == len(caCertsPem) {
		err = errors.New("No CA Certs PEM file specified")
		return
	}
	tlsc.RootCAs, err = LoadRoots(caCertsPem, nil)
	if err != nil {
		return
	}
	if nil == tlsc.ClientCAs {
		tlsc.ClientCAs = tlsc.RootCAs
	}
	return
}

// Does not support password protected PEMs.
func Load(
	privKeyPem, pubCertPem, caCertsPem string,
	tlsc *tls.Config,
) (err error) {

	if 0 != len(privKeyPem) { // if client, may not have a key
		err = LoadKeyAndCert(privKeyPem, pubCertPem, tlsc)
		if err != nil {
			return
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

	tlsc.MinVersion = tls.VersionTLS12
	tlsc.SessionTicketsDisabled = true
	return
}

func LoadRoots(pem string, roots *x509.CertPool) (rv *x509.CertPool, err error) {

	if 0 != len(pem) {
		if !uio.FileExists(pem) {
			err = errors.New("Missing CA Certs PEM file " + pem)
			return
		}
		var pemBytes []byte
		pemBytes, err = os.ReadFile(pem)
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

// Does the config have TLS certs loaded?
func HasTlsCerts(tlsConfig *tls.Config) (rv bool) {
	return nil != tlsConfig && nil != tlsConfig.Certificates
}

func cipherNames() (names12, names13 []string) {
	suites := tls.CipherSuites()
	for _, suite := range suites {
		for _, v := range suite.SupportedVersions {
			if tls.VersionTLS13 == v {
				names13 = append(names13, suite.Name)
			}
			if tls.VersionTLS12 == v {
				names12 = append(names12, suite.Name)
			}
		}
	}
	return
}

func DefaultTlsConfig() (rv *tls.Config) {
	it, err := BuildTlsConfig(nil)
	if err != nil {
		panic(err)
	}
	return it.(*tls.Config)
}
