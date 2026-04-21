package infractl_services

import (
	"testing"

	coredeploy "github.com/babbage88/infra-core/deployment"
)

func TestMergeMariaDBInstallDefaults(t *testing.T) {
	req := coredeploy.MariaDBInstallRequest{
		DatabaseName: "appdb",
		Username:     "app",
		Password:     "secret",
	}

	merged := MergeMariaDBInstallDefaults(req, DefaultMariaDBInstallRequest())
	if merged.Bind != "0.0.0.0" {
		t.Fatalf("expected default bind, got %q", merged.Bind)
	}
	if merged.Port != 3306 {
		t.Fatalf("expected default port, got %d", merged.Port)
	}
	if merged.DatabaseName != "appdb" {
		t.Fatalf("expected explicit database name to be preserved, got %q", merged.DatabaseName)
	}
}

func TestBuildMariaDBURLEscapesCredentialsAndDatabase(t *testing.T) {
	got := BuildMariaDBURL("db-01", 3306, "app db", "app user", "pa:ss@word")
	want := "mysql://app+user:pa%3Ass%40word@db-01:3306/app+db"
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}
