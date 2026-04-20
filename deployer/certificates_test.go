package deployer

import (
	"crypto/x509"
	"net"
	"reflect"
	"testing"
)

func TestAddSANsNormalizesHostPorts(t *testing.T) {
	tmpl := &x509.Certificate{}

	addSANs(tmpl, []string{
		"localhost",
		"localhost:3000",
		"https://localhost:5173",
		"127.0.0.1:8443",
		"127.0.0.1",
		"[::1]:3000",
		"::1",
		"localhost",
	})

	wantDNSNames := []string{"localhost"}
	if !reflect.DeepEqual(tmpl.DNSNames, wantDNSNames) {
		t.Fatalf("DNSNames = %v, want %v", tmpl.DNSNames, wantDNSNames)
	}

	wantIPAddresses := []net.IP{
		net.ParseIP("127.0.0.1"),
		net.ParseIP("::1"),
	}
	if !reflect.DeepEqual(tmpl.IPAddresses, wantIPAddresses) {
		t.Fatalf("IPAddresses = %v, want %v", tmpl.IPAddresses, wantIPAddresses)
	}
}
