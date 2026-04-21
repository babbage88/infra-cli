package infractl_services

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/babbage88/infra-cli/deployer"
	coredeploy "github.com/babbage88/infra-core/deployment"
)

func DefaultMariaDBInstallRequest() coredeploy.MariaDBInstallRequest {
	return coredeploy.MariaDBInstallRequest{
		Bind: "0.0.0.0",
		Port: 3306,
	}
}

func MergeMariaDBInstallDefaults(req coredeploy.MariaDBInstallRequest, defaults coredeploy.MariaDBInstallRequest) coredeploy.MariaDBInstallRequest {
	if strings.TrimSpace(req.DatabaseName) == "" {
		req.DatabaseName = defaults.DatabaseName
	}
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
	req.SSH = MergeSSHDefaults(req.SSH, defaults.SSH)
	return req
}

func InstallMariaDB(req coredeploy.MariaDBInstallRequest) (coredeploy.MariaDBInstallResult, error) {
	if strings.TrimSpace(req.SSH.Host) == "" {
		return coredeploy.MariaDBInstallResult{}, fmt.Errorf("ssh.host is required")
	}
	if strings.TrimSpace(req.SSH.User) == "" {
		return coredeploy.MariaDBInstallResult{}, fmt.Errorf("ssh.user is required")
	}
	if strings.TrimSpace(req.DatabaseName) == "" {
		return coredeploy.MariaDBInstallResult{}, fmt.Errorf("db_name is required")
	}
	if strings.TrimSpace(req.Username) == "" {
		return coredeploy.MariaDBInstallResult{}, fmt.Errorf("username is required")
	}
	if strings.TrimSpace(req.Password) == "" {
		return coredeploy.MariaDBInstallResult{}, fmt.Errorf("password is required")
	}
	if req.Port <= 0 {
		return coredeploy.MariaDBInstallResult{}, fmt.Errorf("port must be greater than zero")
	}

	sshOpts, cleanupSSHKey, err := PrepareSSHOptions(req.SSH)
	if err != nil {
		return coredeploy.MariaDBInstallResult{}, err
	}
	defer cleanupSSHKey()

	installer, err := deployer.NewRemoteMariaDBInstallerWithSsh(
		sshOpts.Host,
		sshOpts.User,
		sshOpts.KeyPath,
		sshOpts.Passphrase,
		sshOpts.UseAgent,
		sshOpts.Port,
	)
	if err != nil {
		return coredeploy.MariaDBInstallResult{}, fmt.Errorf("initialize SSH client: %w", err)
	}
	defer installer.SshClient.Close()

	if err := installer.EnsureInstalledAndConfigured(req.DatabaseName, req.Username, req.Password, req.Bind, req.Port); err != nil {
		return coredeploy.MariaDBInstallResult{}, err
	}

	return coredeploy.MariaDBInstallResult{
		Host:         sshOpts.Host,
		Port:         req.Port,
		DatabaseName: req.DatabaseName,
		Username:     req.Username,
		URI:          BuildMariaDBURL(sshOpts.Host, req.Port, req.DatabaseName, req.Username, req.Password),
	}, nil
}

func BuildMariaDBURL(host string, port int, dbname, username, password string) string {
	return fmt.Sprintf(
		"mysql://%s:%s@%s:%d/%s",
		url.QueryEscape(username),
		url.QueryEscape(password),
		host,
		port,
		url.QueryEscape(dbname),
	)
}
