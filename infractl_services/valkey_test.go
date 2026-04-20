package infractl_services

import (
	"testing"

	coredeploy "github.com/babbage88/infra-core/deployment"
)

func TestMergeValkeyInstallDefaults(t *testing.T) {
	req := coredeploy.ValkeyInstallRequest{
		Username: "app",
		Password: "secret",
	}

	merged := MergeValkeyInstallDefaults(req, DefaultValkeyInstallRequest())
	if merged.Bind != "0.0.0.0" {
		t.Fatalf("expected default bind, got %q", merged.Bind)
	}
	if merged.Port != 6379 {
		t.Fatalf("expected default port, got %d", merged.Port)
	}
	if merged.Username != "app" {
		t.Fatalf("expected explicit username to be preserved, got %q", merged.Username)
	}
}

func TestBuildValkeyURLEscapesCredentials(t *testing.T) {
	got := BuildValkeyURL("valkey-01", 6379, "app user", "pa:ss@word")
	want := "redis://app+user:pa%3Ass%40word@valkey-01:6379"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
