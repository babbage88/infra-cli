package ssh

import (
	"github.com/babbage88/goph/v2"
	coressh "github.com/babbage88/infra-core/ssh"
)

type RemoteAppDeploymentAgent = coressh.RemoteAppDeploymentAgent
type Runner = coressh.Runner
type Uploader = coressh.Uploader
type Client = coressh.Client
type PublicKeyOption = coressh.PublicKeyOption
type SshInitializationError = coressh.SshInitializationError
type SftpInitializationError = coressh.SftpInitializationError
type SftpFileCreationializationError = coressh.SftpFileCreationializationError
type SftpTransferError = coressh.SftpTransferError

var (
	SetIgnoreHostKeyVerification = coressh.SetIgnoreHostKeyVerification
	ExpandPath                   = coressh.ExpandPath
	CurrentUserName              = coressh.CurrentUserName
	DefaultPrivateKeyPath        = coressh.DefaultPrivateKeyPath
	ShellQuote                   = coressh.ShellQuote
	FormatExecError              = coressh.FormatExecError
	DiscoverPublicKeyContents    = coressh.DiscoverPublicKeyContents
	DiscoverPublicKeyOptions     = coressh.DiscoverPublicKeyOptions
	VerifyHost                   = coressh.VerifyHost
)

func SshErrorWrapper(code int, err error, message string) error {
	return coressh.SshErrorWrapper(code, err, message)
}

func SftpInitErrorWrapper(code int, err error, message string) error {
	return coressh.SftpInitErrorWrapper(code, err, message)
}

func SftpFileCreationErrorWrapper(code int, err error, message string) error {
	return coressh.SftpFileCreationErrorWrapper(code, err, message)
}

func SftpErrorWrapper(code int, err error, message string) error {
	return coressh.SftpErrorWrapper(code, err, message)
}

func NewRemoteAppDeploymentAgentWithPassword(hostname, sshUser, srcUtilsPath, dstUtilsPath, sshPassword string, envVars map[string]string, port uint) (*RemoteAppDeploymentAgent, error) {
	return coressh.NewRemoteAppDeploymentAgentWithPassword(hostname, sshUser, srcUtilsPath, dstUtilsPath, sshPassword, envVars, port)
}

func NewRemoteAppDeploymentAgentWithSshKey(hostname, sshUser, srcUtilsPath, dstUtilsPath, sshKey, sshPassphrase string, envVars map[string]string, agent bool, port uint) (*RemoteAppDeploymentAgent, error) {
	return coressh.NewRemoteAppDeploymentAgentWithSshKey(hostname, sshUser, srcUtilsPath, dstUtilsPath, sshKey, sshPassphrase, envVars, agent, port)
}

func InitializeRemoteSshAgent(hostname, sshUser, sshKey, sshPassphrase string, agent bool, port uint) (*RemoteAppDeploymentAgent, error) {
	return coressh.InitializeRemoteSshAgent(hostname, sshUser, sshKey, sshPassphrase, agent, port)
}

func InitializeSshClient(hostname, username, sshKey, sshPassphrase string, useAgent bool, port uint) (*goph.Client, error) {
	return coressh.InitializeSshClient(hostname, username, sshKey, sshPassphrase, useAgent, port)
}

func RunCommandAndCaptureOutput(c *goph.Client, remoteCmd string, args []string) ([]byte, error) {
	return coressh.RunCommandAndCaptureOutput(c, remoteCmd, args)
}
