package ssh

import (
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net"

	"github.com/babbage88/goph/v2"
	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const validateUserUidBase string = "validate-user"

type RemoteAppDeploymentAgent struct {
	SshClient           *goph.Client      `json:"-"`
	SourceUtilsDir      string            `json:"srcUtilsDir"`
	DestinationUtilsDir string            `json:"dstUtilsDir"`
	EnvVars             map[string]string `json:"envVars"`
	RemoteCommand       *goph.Cmd         `json:"remoteCommands"`
}

func VerifyHost(host string, remote net.Addr, key ssh.PublicKey) error {
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
	var auth goph.Auth
	var err error
	if agent || goph.HasAgent() {
		auth, err = goph.UseAgent()
		if err != nil {
			log.Fatal(err)
		}

	} else {
		auth, err = goph.Key(sshKeyPath, sshPassphrase)
	}

	if err != nil {
		log.Fatal(err)
	}

	client, err := goph.NewConn(&goph.Config{
		User:     user,
		Addr:     host,
		Port:     port,
		Auth:     auth,
		Callback: VerifyHost,
	})
	if err != nil {
		log.Fatal(err)
	}
	// Defer closing the network connection.
	return client, err
}

func NewRemoteAppDeploymentAgentWithPassword(hostname, sshUser, srcUtilsPath, dstUtilsPath, sshPassword string, envVars map[string]string, port uint) (*RemoteAppDeploymentAgent, error) {
	sshClient, err := goph.NewConn(&goph.Config{
		User:     sshUser,
		Addr:     hostname,
		Port:     port,
		Auth:     goph.Password(sshPassword),
		Callback: VerifyHost,
	})
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

func NewRemoteAppDeploymentAgentWithSshKey(hostname, sshUser, srcUtilsPath, dstUtilsPath, sshKey, sshPassphrase string, envVars map[string]string, agent bool, port uint) (*RemoteAppDeploymentAgent, error) {
	sshClient, err := initializeSshClient(hostname, sshUser, port, sshKey, sshPassphrase, agent)
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

func InitializeRemoteSshAgent(hostname, sshUser, sshKey, sshPassphrase string, agent bool, port uint) (*RemoteAppDeploymentAgent, error) {
	sshClient, err := initializeSshClient(hostname, sshUser, port, sshKey, sshPassphrase, agent)
	if err != nil {
		log.Printf("Error initializing ssh client %s\n", err.Error())
		return nil, SshErrorWrapper(500, err, "failed to initialize ssh client")
	}

	remoteDeployAgent := RemoteAppDeploymentAgent{
		SshClient: sshClient,
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

func (r *RemoteAppDeploymentAgent) UploadBin(src, dst string) error {
	err := r.SshClient.Upload(src, dst)
	if err != nil {
		log.Printf("Error uploading files to remote  src: %s dst: %s err: %s\n", src, dst, err.Error())
		return SftpErrorWrapper(501, err, "error preforming upload over sftp")
	}
	r.RunCommand("chmod", []string{"+x", dst})
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
	// You can set env vars, but the server must be configured to `AcceptEnv line`.
	cmd.Env = r.GetEnvarSlice()

	log.Printf("Executing remote command cmd: %s args: %v\n", remoteCmd, args)
	if err != nil {
		slog.Error("error initializing goph Command", "error", err.Error())
		return err
	}

	// Only run ONCE
	err = cmd.Run()
	if err != nil {
		slog.Error("error Running goph Command", "error", err.Error())
	}
	return err
}

func (r *RemoteAppDeploymentAgent) RunCommandAndCaptureOutput(remoteCmd string, args []string) ([]byte, error) {
	cmd, err := r.SshClient.Command(remoteCmd, args...)
	if err != nil {
		return nil, err
	}

	// You can set env vars, but the server must be configured to `AcceptEnv line`.
	cmd.Env = r.GetEnvarSlice()

	combinedOutput, err := cmd.CombinedOutput()
	if err != nil {
		fmt.Println("Error:", err)
		return nil, err
	}
	return combinedOutput, err
}

func (r *RemoteAppDeploymentAgent) GetEnvarSlice() []string {
	argEnvars := make([]string, len(r.EnvVars))
	for k, v := range r.EnvVars {
		argEnvars = append(argEnvars, fmt.Sprintf("%s=%s", k, v))
	}
	return argEnvars
}
