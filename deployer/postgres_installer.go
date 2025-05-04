package deployer

import (
	"github.com/babbage88/goph/v2"
	"github.com/babbage88/infra-cli/ssh"
)

type RemotePostgresInstaller struct {
	RemoteHostname string       `json:"hostname"`
	SshClient      *goph.Client `json:"sshClient"`
	RemoteSshUser  string       `json:"remoteSshUser"`
}

func NewRemotePostgresInstallerWithSsh(hostname, sshUser, sshKey, sshPassphrase string, useSshAgent bool, port uint) *RemotePostgresInstaller {
	client, err := ssh.InitializeSshClient(hostname, sshUser, sshKey, sshPassphrase, useSshAgent, port)
	if err != nil {
		return nil
	}
	pgInstaller := &RemotePostgresInstaller{
		SshClient:      client,
		RemoteHostname: hostname,
		RemoteSshUser:  sshUser,
	}
	return pgInstaller
}
