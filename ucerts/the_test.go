package ucerts

import (
	"crypto/tls"
	"testing"

	"github.com/tredeske/u/uconfig"
)

func TestCerts(t *testing.T) {

	tlsc := DefaultTlsConfig()
	if nil == tlsc {
		t.Fatalf("did not create")
	}

	if HasTlsCerts(tlsc) {
		t.Fatalf("should not")
	}
}

func TestCreate(t *testing.T) {
	configS := `
tlsCiphers:
- TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384
`
	var tlsConfig *tls.Config
	s, err := uconfig.NewSection(configS)
	if err != nil {
		t.Fatalf("Unable to parse: %s", err)
	}
	err = s.Chain().
		Build(&tlsConfig, BuildTlsConfig).
		Done()
	if err != nil {
		t.Fatalf("Unable to parse: %s", err)
	} else if nil == tlsConfig {
		t.Fatalf("No TLS config produced")
	} else if tlsConfig.InsecureSkipVerify {
		t.Fatalf("insecure set!")
	} else if tlsConfig.MinVersion != tls.VersionTLS12 {
		t.Fatalf("tls min is not 1.2, is %d", tlsConfig.MinVersion)
	} else if 1 != len(tlsConfig.CipherSuites) {
		t.Fatalf("tls CipherSuites is len %d", len(tlsConfig.CipherSuites))
	} else if tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384 != tlsConfig.CipherSuites[0] {
		t.Fatalf("Invalid cipher suite set: %#v", tlsConfig.CipherSuites)
	}
}
