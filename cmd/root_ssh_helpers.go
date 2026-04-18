package cmd

import (
	"fmt"
	"strings"

	"github.com/babbage88/goph/v2"
	infraSSH "github.com/babbage88/infra-cli/ssh"
)

type rootSSHOptions struct {
	Host       string
	User       string
	KeyPath    string
	Passphrase string
	UseAgent   bool
	Port       uint
}

func resolveRootSSHOptions(defaultHost, defaultUser string) (rootSSHOptions, error) {
	opts := rootSSHOptions{
		Host:       strings.TrimSpace(rootViperCfg.GetString("ssh_remote_host")),
		User:       strings.TrimSpace(rootViperCfg.GetString("ssh_remote_user")),
		KeyPath:    infraSSH.ExpandPath(rootViperCfg.GetString("ssh_key")),
		Passphrase: rootViperCfg.GetString("ssh_passphrase"),
		UseAgent:   rootViperCfg.GetBool("ssh_use_agent"),
		Port:       rootViperCfg.GetUint("ssh_port"),
	}

	if opts.Host == "" {
		opts.Host = strings.TrimSpace(defaultHost)
	}
	if opts.Host == "" {
		return opts, fmt.Errorf("SSH host is required; set the global --ssh-remote-host flag")
	}

	if opts.User == "" {
		opts.User = strings.TrimSpace(defaultUser)
	}
	if opts.User == "" {
		opts.User = infraSSH.CurrentUserName()
	}
	if opts.Port == 0 {
		opts.Port = 22
	}

	return opts, nil
}

func initializeRootSSHClient(defaultHost, defaultUser string) (*goph.Client, rootSSHOptions, error) {
	opts, err := resolveRootSSHOptions(defaultHost, defaultUser)
	if err != nil {
		return nil, opts, err
	}

	client, err := infraSSH.InitializeSshClient(opts.Host, opts.User, opts.KeyPath, opts.Passphrase, opts.UseAgent, opts.Port)
	if err != nil {
		return nil, opts, err
	}

	return client, opts, nil
}
