package ssh

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSSHAuthMethodsUsesExplicitKeyWithoutAgent(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "missing-agent.sock"))

	keyPath := writeTestPrivateKey(t)

	auth, err := buildSSHAuthMethods(keyPath, "", false)
	if err != nil {
		t.Fatalf("expected key auth to succeed without agent, got error: %v", err)
	}

	if len(auth) != 1 {
		t.Fatalf("expected exactly one auth method from explicit key, got %d", len(auth))
	}
}

func TestBuildSSHAuthMethodsFallsBackToKeyWhenAgentRequestedButUnavailable(t *testing.T) {
	t.Setenv("SSH_AUTH_SOCK", filepath.Join(t.TempDir(), "missing-agent.sock"))

	keyPath := writeTestPrivateKey(t)

	auth, err := buildSSHAuthMethods(keyPath, "", true)
	if err != nil {
		t.Fatalf("expected key auth fallback to succeed, got error: %v", err)
	}

	if len(auth) != 1 {
		t.Fatalf("expected key fallback auth method, got %d methods", len(auth))
	}
}

func TestBuildSSHAuthMethodsErrorsWithoutKeyOrAgent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("SSH_AUTH_SOCK", "")

	_, err := buildSSHAuthMethods("", "", false)
	if err == nil {
		t.Fatal("expected an error when no key or agent is configured")
	}
}

func TestBuildSSHAuthMethodsUsesAutoDiscoveredDefaultKey(t *testing.T) {
	homeDir := t.TempDir()
	t.Setenv("HOME", homeDir)
	t.Setenv("SSH_AUTH_SOCK", "")

	sshDir := filepath.Join(homeDir, ".ssh")
	if err := os.MkdirAll(sshDir, 0700); err != nil {
		t.Fatalf("create .ssh dir: %v", err)
	}

	keyPath := filepath.Join(sshDir, "id_rsa")
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})
	if err := os.WriteFile(keyPath, privateKeyPEM, 0600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	auth, err := buildSSHAuthMethods("", "", false)
	if err != nil {
		t.Fatalf("expected auto-discovered key auth to succeed, got error: %v", err)
	}

	if len(auth) != 1 {
		t.Fatalf("expected exactly one auth method from auto-discovered key, got %d", len(auth))
	}
}

func writeTestPrivateKey(t *testing.T) string {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate rsa key: %v", err)
	}

	privateKeyPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(privateKey),
	})

	keyPath := filepath.Join(t.TempDir(), "id_rsa")
	if err := os.WriteFile(keyPath, privateKeyPEM, 0600); err != nil {
		t.Fatalf("write private key: %v", err)
	}

	return keyPath
}
