package infractl_services

import (
	"encoding/base64"
	"os"
	"testing"

	coredeploy "github.com/babbage88/infra-core/deployment"
)

func TestPrepareSSHOptionsWritesBase64PrivateKeyToTemporaryFile(t *testing.T) {
	keyContent := "-----BEGIN OPENSSH PRIVATE KEY-----\ntest\n-----END OPENSSH PRIVATE KEY-----\n"
	opts, cleanup, err := PrepareSSHOptions(coredeploy.SSHOptions{
		PrivateKeyBase64: base64.StdEncoding.EncodeToString([]byte(keyContent)),
	})
	if err != nil {
		t.Fatalf("PrepareSSHOptions returned error: %v", err)
	}
	defer cleanup()

	if opts.KeyPath == "" {
		t.Fatal("expected temporary key path")
	}
	if opts.PrivateKeyBase64 != "" {
		t.Fatal("expected private key base64 to be cleared after materialization")
	}
	got, err := os.ReadFile(opts.KeyPath)
	if err != nil {
		t.Fatalf("read temporary key: %v", err)
	}
	if string(got) != keyContent {
		t.Fatalf("expected key content %q, got %q", keyContent, string(got))
	}

	cleanup()
	if _, err := os.Stat(opts.KeyPath); !os.IsNotExist(err) {
		t.Fatalf("expected cleanup to remove temporary key, stat err: %v", err)
	}
}

func TestPrepareSSHOptionsRejectsMultipleKeySources(t *testing.T) {
	_, _, err := PrepareSSHOptions(coredeploy.SSHOptions{
		KeyPath:       "/tmp/key",
		PrivateKeyPEM: "key",
	})
	if err == nil {
		t.Fatal("expected error for multiple key sources")
	}
}
