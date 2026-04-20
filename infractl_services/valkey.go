package infractl_services

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/babbage88/infra-cli/deployer"
	coredeploy "github.com/babbage88/infra-core/deployment"
)

func DefaultValkeyInstallRequest() coredeploy.ValkeyInstallRequest {
	return coredeploy.ValkeyInstallRequest{
		Bind: "0.0.0.0",
		Port: 6379,
	}
}

func MergeValkeyInstallDefaults(req coredeploy.ValkeyInstallRequest, defaults coredeploy.ValkeyInstallRequest) coredeploy.ValkeyInstallRequest {
	if strings.TrimSpace(req.Username) == "" {
		req.Username = defaults.Username
	}
	if strings.TrimSpace(req.Password) == "" {
		req.Password = defaults.Password
	}
	if strings.TrimSpace(req.Bind) == "" {
		req.Bind = defaults.Bind
	}
	if req.Port == 0 {
		req.Port = defaults.Port
	}
	if strings.TrimSpace(req.ACLFile) == "" {
		req.ACLFile = defaults.ACLFile
	}
	req.SSH = MergeSSHDefaults(req.SSH, defaults.SSH)
	return req
}

func InstallValkey(req coredeploy.ValkeyInstallRequest) (coredeploy.ValkeyInstallResult, error) {
	if strings.TrimSpace(req.SSH.Host) == "" {
		return coredeploy.ValkeyInstallResult{}, fmt.Errorf("ssh.host is required")
	}
	if strings.TrimSpace(req.SSH.User) == "" {
		return coredeploy.ValkeyInstallResult{}, fmt.Errorf("ssh.user is required")
	}
	if strings.TrimSpace(req.Username) == "" {
		return coredeploy.ValkeyInstallResult{}, fmt.Errorf("username is required")
	}
	if strings.TrimSpace(req.Password) == "" {
		return coredeploy.ValkeyInstallResult{}, fmt.Errorf("password is required")
	}
	if req.Port <= 0 {
		return coredeploy.ValkeyInstallResult{}, fmt.Errorf("port must be greater than zero")
	}

	sshOpts, cleanupSSHKey, err := PrepareSSHOptions(req.SSH)
	if err != nil {
		return coredeploy.ValkeyInstallResult{}, err
	}
	defer cleanupSSHKey()

	installer, err := deployer.NewRemoteValkeyInstallerWithSsh(
		sshOpts.Host,
		sshOpts.User,
		sshOpts.KeyPath,
		sshOpts.Passphrase,
		sshOpts.UseAgent,
		sshOpts.Port,
	)
	if err != nil {
		return coredeploy.ValkeyInstallResult{}, fmt.Errorf("initialize SSH client: %w", err)
	}
	defer installer.SshClient.Close()

	if err := installer.EnsureInstalledAndConfigured(req.Username, req.Password, req.Bind, req.Port, req.ACLFile); err != nil {
		return coredeploy.ValkeyInstallResult{}, err
	}

	return coredeploy.ValkeyInstallResult{
		Host:     sshOpts.Host,
		Port:     req.Port,
		Username: req.Username,
		URI:      BuildValkeyURL(sshOpts.Host, req.Port, req.Username, req.Password),
	}, nil
}

func BuildValkeyURL(host string, port int, username, password string) string {
	return fmt.Sprintf(
		"redis://%s:%s@%s:%d",
		url.QueryEscape(username),
		url.QueryEscape(password),
		host,
		port,
	)
}
