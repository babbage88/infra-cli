package ssh

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"

	"github.com/babbage88/goph/v2"
	cryptossh "golang.org/x/crypto/ssh"
)

type RemoteAppDeploymentAgent struct {
	SshClient           *goph.Client      `json:"-"`
	SourceUtilsDir      string            `json:"srcUtilsDir"`
	DestinationUtilsDir string            `json:"dstUtilsDir"`
	EnvVars             map[string]string `json:"envVars"`
	RemoteCommand       *goph.Cmd         `json:"remoteCommands"`
}

func VerifyHost(host string, remote net.Addr, key cryptossh.PublicKey) error {
	//
	// If you want to connect to new hosts.
	// here your should check new connections public keys
	// if the key not trusted you shuld return an error
	//

	// hostFound: is host in known hosts file.
	// err: error if key not in known hosts file OR host in known hosts file but key changed!
	hostFound, err := goph.CheckKnownHost(host, remote, key, "")

	// Host in known hosts but key mismatch!
	// Maybe because of MAN IN THE MIDDLE ATTACK!
	if hostFound && err != nil {
		return err
	}

	// handshake because public key already exists.
	if hostFound && err == nil {
		return nil
	}

	// Ask user to check if he trust the host public key.
	if askIsHostTrusted(host, key) == false {
		// Make sure to return error on non trusted keys.
		return errors.New("you typed no, aborted!")
	}

	// Add the new host to known hosts file.
	return goph.AddKnownHost(host, remote, key, "")
}

func initializeSshClient(host string, user string, port uint, sshKeyPath string, sshPassphrase string, agent bool) (*goph.Client, error) {
	auth, err := buildSSHAuthMethods(sshKeyPath, sshPassphrase, agent)
	if err != nil {
		return nil, err
	}

	client, err := goph.NewConn(&goph.Config{
		User:     user,
		Addr:     host,
		Port:     port,
		Auth:     auth,
		Timeout:  goph.DefaultTimeout,
		Callback: VerifyHost,
	})
	if err != nil {
		return nil, err
	}
	// Defer closing the network connection.
	return client, err
}

func buildSSHAuthMethods(sshKeyPath string, sshPassphrase string, useAgent bool) (goph.Auth, error) {
	keyPaths := resolveSSHKeyPaths(sshKeyPath)
	authMethods := make(goph.Auth, 0, len(keyPaths)+1)

	for _, keyPath := range keyPaths {
		keyAuth, err := goph.Key(keyPath, sshPassphrase)
		if err != nil {
			return nil, fmt.Errorf("load SSH key %q: %w", keyPath, err)
		}
		authMethods = append(authMethods, keyAuth...)
	}

	shouldUseAgent := useAgent || goph.HasAgent()
	if shouldUseAgent {
		agentAuth, err := goph.UseAgent()
		if err != nil {
			if useAgent && len(keyPaths) == 0 {
				return nil, fmt.Errorf("use ssh agent: %w", err)
			}
			if useAgent {
				slog.Warn("SSH agent unavailable, continuing with SSH key authentication", "error", err.Error())
			}
		} else {
			authMethods = append(authMethods, agentAuth...)
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no SSH authentication method configured; provide --ssh-key or enable ssh-agent")
	}

	return authMethods, nil
}

func resolveSSHKeyPaths(explicitPath string) []string {
	if explicitPath != "" {
		return []string{explicitPath}
	}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		return nil
	}

	var keyPaths []string
	for _, name := range []string{"id_ed25519", "id_rsa"} {
		candidate := filepath.Join(homeDir, ".ssh", name)
		if info, statErr := os.Stat(candidate); statErr == nil && !info.IsDir() {
			keyPaths = append(keyPaths, candidate)
		}
	}

	return keyPaths
}

func InitializeSshClient(hostname, username, sshKey, sshPassphrase string, useAgent bool, port uint) (*goph.Client, error) {
	client, err := initializeSshClient(hostname, username, port, sshKey, sshPassphrase, useAgent)
	if err != nil {
		return client, SshErrorWrapper(500, err, "Error initializing ssh client")
	}
	slog.Info("ssh client initalized successfully")
	return client, nil
}

func RunCommandAndCaptureOutput(c *goph.Client, remoteCmd string, args []string) ([]byte, error) {
	cmd, err := c.Command(remoteCmd, args...)
	if err != nil {
		return nil, err
	}

	combinedOutput, err := cmd.CombinedOutput()
	if err != nil {
		slog.Error("error attempting command", "error", err.Error())
		return nil, err
	}
	return combinedOutput, err
}
