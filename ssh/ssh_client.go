package ssh

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/babbage88/goph/v2"
	sshconfig "github.com/kevinburke/ssh_config"
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
	return verifyKnownHost(host, remote, key)
}

func verifyKnownHost(host string, remote net.Addr, key cryptossh.PublicKey) error {
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
	originalHost := host
	explicitKeyProvided := strings.TrimSpace(sshKeyPath) != ""
	host, user, port, sshKeyPath = resolveSSHConfig(host, user, port, sshKeyPath)
	authSource := determineSSHAuthSource(explicitKeyProvided, sshKeyPath, agent)
	slog.Info("SSH auth selection", "alias", originalHost, "hostname", host, "user", user, "port", port, "auth_source", authSource, "ssh_key", sshKeyPath, "ssh_agent_requested", agent, "ssh_agent_available", goph.HasAgent())

	auth, err := buildSSHAuthMethods(sshKeyPath, sshPassphrase, agent)
	if err != nil {
		return nil, err
	}

	callback := makeHostKeyCallback(originalHost, host)
	client, err := goph.NewConn(&goph.Config{
		User:     user,
		Addr:     host,
		Port:     port,
		Auth:     auth,
		Timeout:  goph.DefaultTimeout,
		Callback: callback,
	})
	if err != nil {
		return nil, err
	}
	// Defer closing the network connection.
	return client, err
}

func makeHostKeyCallback(originalHost, resolvedHost string) func(string, net.Addr, cryptossh.PublicKey) error {
	return func(_ string, remote net.Addr, key cryptossh.PublicKey) error {
		hostForVerification := originalHost
		if strings.TrimSpace(hostForVerification) == "" {
			hostForVerification = resolvedHost
		}

		return verifyKnownHost(hostForVerification, remote, key)
	}
}

func resolveSSHConfig(host, user string, port uint, sshKeyPath string) (string, string, uint, string) {
	if strings.TrimSpace(host) == "" {
		return host, user, port, sshKeyPath
	}

	allowConfigIdentityFile := strings.TrimSpace(sshKeyPath) == "" || isAutoDetectedDefaultSSHKeyPath(sshKeyPath)
	resolvedHost := strings.TrimSpace(sshconfig.Get(host, "HostName"))
	if resolvedHost == "" {
		resolvedHost = host
	}

	if strings.TrimSpace(user) == "" {
		if configUser := strings.TrimSpace(sshconfig.Get(host, "User")); configUser != "" {
			user = configUser
		}
	}

	if configPort := strings.TrimSpace(sshconfig.Get(host, "Port")); configPort != "" {
		if port == 0 || port == 22 {
			if parsedPort, err := strconv.ParseUint(configPort, 10, 16); err == nil {
				port = uint(parsedPort)
			}
		}
	}

	if allowConfigIdentityFile {
		for _, identityFile := range sshconfig.GetAll(host, "IdentityFile") {
			expanded := expandSSHConfigPath(identityFile)
			if expanded == "" {
				continue
			}
			if info, err := os.Stat(expanded); err == nil && !info.IsDir() {
				sshKeyPath = expanded
				break
			}
		}
	}

	if resolvedHost != host {
		slog.Info("Resolved SSH host via ~/.ssh/config", "alias", host, "hostname", resolvedHost, "user", user, "port", port, "ssh_key", sshKeyPath)
	}

	return resolvedHost, user, port, sshKeyPath
}

func isAutoDetectedDefaultSSHKeyPath(path string) bool {
	path = strings.TrimSpace(path)
	if path == "" {
		return false
	}

	for _, candidate := range resolveSSHKeyPaths("") {
		if candidate == path {
			return true
		}
	}

	return false
}

func determineSSHAuthSource(explicitKeyProvided bool, sshKeyPath string, agentRequested bool) string {
	switch {
	case explicitKeyProvided && strings.TrimSpace(sshKeyPath) != "":
		return "explicit-key"
	case strings.TrimSpace(sshKeyPath) != "" && !isAutoDetectedDefaultSSHKeyPath(sshKeyPath):
		return "ssh-config-identityfile"
	case strings.TrimSpace(sshKeyPath) != "":
		if agentRequested || goph.HasAgent() {
			return "default-key+agent"
		}
		return "default-key"
	case agentRequested || goph.HasAgent():
		return "agent-only"
	default:
		return "none"
	}
}

func expandSSHConfigPath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}

	if strings.HasPrefix(path, "~/") || path == "~" {
		homeDir, err := os.UserHomeDir()
		if err == nil && homeDir != "" {
			return filepath.Join(homeDir, strings.TrimPrefix(path, "~/"))
		}
	}

	return os.ExpandEnv(path)
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
