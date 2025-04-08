package ssh

import (
	"fmt"
	"log"

	"github.com/babbage88/goph"
	"github.com/pkg/sftp"
)

const validateUserUidBase string = "validate-user"

type RemoteAppDeploymentAgent struct {
	SshClient           *goph.Client      `json:"-"`
	SourceUtilsDir      string            `json:"srcUtilsDir"`
	DestinationUtilsDir string            `json:"dstUtilsDir"`
	EnvVars             map[string]string `json:"envVars"`
	RemoteCommand       *goph.Cmd         `json:"remoteCommands"`
}

func initializeSshClient(host string, user string, sshKeyPath string, sshPassphrase string) (*goph.Client, error) {
	auth, err := goph.Key(sshKeyPath, sshPassphrase)
	if err != nil {
		log.Fatal(err)
	}

	client, err := goph.New(user, host, auth)
	if err != nil {
		log.Fatal(err)
	}
	// Defer closing the network connection.
	defer client.Close()
	return client, err
}

func NewRemoteAppDeploymentAgentWithPassword(hostname, sshUser, srcUtilsPath, dstUtilsPath, sshPassword string, envVars map[string]string) (*RemoteAppDeploymentAgent, error) {
	sshClient, err := goph.New(sshUser, hostname, goph.Password(sshPassword))
	if err != nil {
		log.Printf("Error initializing ssh client %s\n", err.Error())
		return nil, SshErrorWrapper(500, err, "failed to initialize ssh client")
	}
	remoteDeployAgent := RemoteAppDeploymentAgent{
		SshClient:           sshClient,
		SourceUtilsDir:      srcUtilsPath,
		DestinationUtilsDir: dstUtilsPath,
		EnvVars:             envVars,
	}

	return &remoteDeployAgent, nil
}

func NewRemoteAppDeploymentAgentWithSshKey(hostname, sshUser, srcUtilsPath, dstUtilsPath, sshKey, sshPassphrase string, envVars map[string]string) (*RemoteAppDeploymentAgent, error) {
	sshClient, err := initializeSshClient(hostname, sshUser, sshKey, sshPassphrase)
	if err != nil {
		log.Printf("Error initializing ssh client %s\n", err.Error())
		return nil, SshErrorWrapper(500, err, "failed to initialize ssh client")
	}

	remoteDeployAgent := RemoteAppDeploymentAgent{
		SshClient:           sshClient,
		SourceUtilsDir:      srcUtilsPath,
		DestinationUtilsDir: dstUtilsPath,
		EnvVars:             envVars,
	}

	return &remoteDeployAgent, nil
}

func (r *RemoteAppDeploymentAgent) CopyUtilsToRemoteHost() error {
	err := r.SshClient.Upload(r.SourceUtilsDir, r.DestinationUtilsDir)
	if err != nil {
		log.Printf("Error uploading RemoteUtils src: %s dst: %s err: %s\n", r.SourceUtilsDir, r.DestinationUtilsDir, err.Error())
		return SftpErrorWrapper(501, err, "error preforming upload over sftp")
	}
	return nil
}

func (r *RemoteAppDeploymentAgent) Upload(src, dst string) error {
	err := r.SshClient.Upload(src, dst)
	if err != nil {
		log.Printf("Error uploading files to remote  src: %s dst: %s err: %s\n", src, dst, err.Error())
		return SftpErrorWrapper(501, err, "error preforming upload over sftp")
	}
	return nil
}

func (r *RemoteAppDeploymentAgent) Download(src, dst string) error {
	err := r.SshClient.Download(src, dst)
	if err != nil {
		log.Printf("Error download files from remote  src: %s dst: %s err: %s\n", src, dst, err.Error())
		return SftpErrorWrapper(501, err, "error preforming upload over sftp")
	}
	return nil
}

func (r *RemoteAppDeploymentAgent) GetSftpClient() (*sftp.Client, error) {
	sftpClient, err := r.SshClient.NewSftp()
	if err != nil {
		log.Printf("Error initializing sftp client err: %s\n", err.Error())
		return nil, SftpInitErrorWrapper(503, err, "error preforming upload over sftp")
	}
	return sftpClient, nil
}

func (r *RemoteAppDeploymentAgent) WriteBytesSftp(destinationPath string, data []byte) (int, error) {
	sftpClient, err := r.GetSftpClient()
	if err != nil {
		log.Printf("Error initializing sftp client err: %s\n", err.Error())
		return 0, SftpInitErrorWrapper(503, err, "error preforming upload over sftp")
	}

	log.Printf("Creating sftp client file: %s on remote host \n", destinationPath)
	f, err := sftpClient.Create(destinationPath)
	if err != nil {
		return 0, SftpFileCreationErrorWrapper(504, err, "error creating file via sftp client")
	}

	bytesWritten, err := f.Write(data)
	defer f.Close()
	if err != nil {
		return 0, SftpFileCreationErrorWrapper(504, err, "error creating file via sftp client")
	}

	log.Printf("Finished writing file: %s bytes: %d remote host\n", destinationPath, bytesWritten)
	return bytesWritten, nil
}

func (r *RemoteAppDeploymentAgent) RunCommand(remoteCmd string, args []string) error {
	cmd, err := r.SshClient.Command(remoteCmd, args...)
	if err != nil {
		return err
	}

	// You can set env vars, but the server must be configured to `AcceptEnv line`.
	cmd.Env = r.GetEnvarSlice()

	log.Printf("Executing remote command cmd: %s args: %v\n", remoteCmd, args)
	// Run you command.
	err = cmd.Run()
	return err
}

func (r *RemoteAppDeploymentAgent) GetEnvarSlice() []string {
	argEnvars := make([]string, len(r.EnvVars))
	for k, v := range r.EnvVars {
		argEnvars = append(argEnvars, fmt.Sprintf("%s=%s", k, v))
	}
	return argEnvars
}
