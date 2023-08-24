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
- TLS_AES_256_GCM_SHA384                    # TLS 1.3
- TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384   # TLS 1.2
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
	} else if 2 != len(tlsConfig.CipherSuites) {
		t.Fatalf("tls CipherSuites is len %d", len(tlsConfig.CipherSuites))
	} else if tls.TLS_AES_256_GCM_SHA384 != tlsConfig.CipherSuites[0] {
		t.Fatalf("Invalid cipher suite set: %#v", tlsConfig.CipherSuites)
	}

	//
	// test tls.ClientHello
	//
	if nil == tlsConfig.GetConfigForClient {
		t.Fatalf("cipher list set but no sifter func added!")
	}

	// say hello with only a TLS 1.2 cipher that will match
	hello := &tls.ClientHelloInfo{
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_CHACHA20_POLY1305_SHA256,
		},
		SupportedVersions: []uint16{tls.VersionTLS12, tls.VersionTLS13},
	}
	_, err = tlsConfig.GetConfigForClient(hello)
	if err != nil {
		t.Fatalf("Problem: %s", err)
	} else if 1 != len(hello.CipherSuites) {
		t.Fatalf("hello cipher suits not sifted.  Is %#v", hello.CipherSuites)
	} else if tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384 != hello.CipherSuites[0] {
		t.Fatalf("hello cipher suits not ok.  Is %#v", hello.CipherSuites)
	}

	// add a TLS1.3 cipher and it should take priority
	hello.CipherSuites = append(hello.CipherSuites, tls.TLS_AES_256_GCM_SHA384)
	_, err = tlsConfig.GetConfigForClient(hello)
	if err != nil {
		t.Fatalf("Problem: %s", err)
	} else if 1 != len(hello.CipherSuites) {
		t.Fatalf("hello cipher suits not sifted.  Is %#v", hello.CipherSuites)
	} else if tls.TLS_AES_256_GCM_SHA384 != hello.CipherSuites[0] {
		t.Fatalf("hello cipher suits not ok.  Is %#v", hello.CipherSuites)
	}

}
