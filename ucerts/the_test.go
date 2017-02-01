package ucerts

import "testing"

func TestCerts(t *testing.T) {

	tlsc := DefaultTlsConfig()
	if nil == tlsc {
		t.Fatalf("did not create")
	}

	if HasTlsCerts(tlsc) {
		t.Fatalf("should not")
	}
}
