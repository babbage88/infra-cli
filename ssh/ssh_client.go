package ssh

import (
	"errors"
	"log/slog"
	"net"
	"os"

	"github.com/babbage88/goph/v2"
	"golang.org/x/crypto/ssh"
)

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
			slog.Error(err.Error())
			os.Exit(1)
		}

	} else {
		auth, err = goph.Key(sshKeyPath, sshPassphrase)
	}

	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}

	client, err := goph.NewConn(&goph.Config{
		User:     user,
		Addr:     host,
		Port:     port,
		Auth:     auth,
		Callback: VerifyHost,
	})
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	// Defer closing the network connection.
	return client, err
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
