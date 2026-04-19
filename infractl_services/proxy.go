package infractl_services

import (
	"fmt"
	"strings"

	"github.com/babbage88/infra-cli/deployer"
	coredeploy "github.com/babbage88/infra-core/deployment"
)

func DefaultProxyInstallRequest(name string) (coredeploy.ProxyInstallRequest, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "nginx":
		return coredeploy.ProxyInstallRequest{
			Name:        "nginx",
			PackageName: "nginx",
			BinaryName:  "nginx",
			ServiceName: "nginx",
			ConfigPath:  "/etc/nginx/nginx.conf",
		}, nil
	case "haproxy":
		return coredeploy.ProxyInstallRequest{
			Name:        "haproxy",
			PackageName: "haproxy",
			BinaryName:  "haproxy",
			ServiceName: "haproxy",
			ConfigPath:  "/etc/haproxy/haproxy.cfg",
		}, nil
	case "angie":
		return coredeploy.ProxyInstallRequest{
			Name:        "angie",
			PackageName: "angie",
			BinaryName:  "angie",
			ServiceName: "angie",
			ConfigPath:  "/etc/angie/angie.conf",
		}, nil
	default:
		return coredeploy.ProxyInstallRequest{}, fmt.Errorf("unsupported proxy %q", name)
	}
}

func MergeProxyInstallDefaults(req coredeploy.ProxyInstallRequest, defaults coredeploy.ProxyInstallRequest) coredeploy.ProxyInstallRequest {
	if strings.TrimSpace(req.Name) == "" {
		req.Name = defaults.Name
	}
	if strings.TrimSpace(req.PackageName) == "" {
		req.PackageName = defaults.PackageName
	}
	if strings.TrimSpace(req.BinaryName) == "" {
		req.BinaryName = defaults.BinaryName
	}
	if strings.TrimSpace(req.ServiceName) == "" {
		req.ServiceName = defaults.ServiceName
	}
	if strings.TrimSpace(req.ConfigPath) == "" {
		req.ConfigPath = defaults.ConfigPath
	}
	if strings.TrimSpace(req.LocalConfigPath) == "" {
		req.LocalConfigPath = defaults.LocalConfigPath
	}
	req.SSH = MergeSSHDefaults(req.SSH, defaults.SSH)
	return req
}

func MergeSSHDefaults(req, defaults coredeploy.SSHOptions) coredeploy.SSHOptions {
	if strings.TrimSpace(req.Host) == "" {
		req.Host = defaults.Host
	}
	if strings.TrimSpace(req.User) == "" {
		req.User = defaults.User
	}
	if strings.TrimSpace(req.KeyPath) == "" {
		req.KeyPath = defaults.KeyPath
	}
	if strings.TrimSpace(req.Passphrase) == "" {
		req.Passphrase = defaults.Passphrase
	}
	if req.Port == 0 {
		req.Port = defaults.Port
	}
	if !req.UseAgent {
		req.UseAgent = defaults.UseAgent
	}
	if req.Port == 0 {
		req.Port = 22
	}
	return req
}

func InstallProxy(req coredeploy.ProxyInstallRequest) (coredeploy.ProxyInstallResult, error) {
	req.Name = strings.ToLower(strings.TrimSpace(req.Name))
	if req.Name == "" {
		return coredeploy.ProxyInstallResult{}, fmt.Errorf("proxy name is required")
	}
	if strings.TrimSpace(req.SSH.Host) == "" {
		return coredeploy.ProxyInstallResult{}, fmt.Errorf("ssh.host is required")
	}
	if strings.TrimSpace(req.SSH.User) == "" {
		return coredeploy.ProxyInstallResult{}, fmt.Errorf("ssh.user is required")
	}

	installer, err := deployer.NewRemoteWebProxyInstallerWithSsh(
		req.SSH.Host,
		req.SSH.User,
		req.SSH.KeyPath,
		req.SSH.Passphrase,
		req.SSH.UseAgent,
		req.SSH.Port,
	)
	if err != nil {
		return coredeploy.ProxyInstallResult{}, fmt.Errorf("initialize SSH client: %w", err)
	}
	defer installer.SshClient.Close()

	cfg := deployer.WebProxyInstallConfig{
		Name:            req.Name,
		PackageName:     req.PackageName,
		BinaryName:      req.BinaryName,
		ServiceName:     req.ServiceName,
		ConfigPath:      req.ConfigPath,
		LocalConfigPath: req.LocalConfigPath,
	}
	if err := installer.EnsureInstalledAndConfigured(cfg); err != nil {
		return coredeploy.ProxyInstallResult{}, err
	}

	return coredeploy.ProxyInstallResult{
		Host:            req.SSH.Host,
		Name:            req.Name,
		PackageName:     req.PackageName,
		BinaryName:      req.BinaryName,
		ServiceName:     req.ServiceName,
		ConfigPath:      req.ConfigPath,
		LocalConfigPath: req.LocalConfigPath,
	}, nil
}
