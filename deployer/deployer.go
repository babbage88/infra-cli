package deployer

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/babbage88/infra-cli/ssh"
)

const (
	deployUtilsPath           string = "remote_utils/bin"
	deployUtilsTar            string = "remote_utils.tar.gz"
	remoteUtilsPath           string = "/tmp/utils"
	validateUserUtilPath      string = "remote_utils/bin/validate-user"
	remoteValidateUserBaseCmd string = "/tmp/utils/remote_utils/validate-user"
	mkdirCmdBase              string = "mkdir"
	mkdirArgs                 string = "-p"
)

type AppDeployer interface {
	InstallApplication() error
	ConfigureService(hostName string, serviceAccount map[int64]string, envVars map[string]string)
}

type RemoteSystemdDeployerOptions func(r *RemoteSystemdDeployer)

type RemoteSystemdDeployer struct {
	SshClient      *ssh.RemoteAppDeploymentAgent `json:"sshClient"`
	RemoteHostName string                        `json:"remoteHost"`
	RemoteSshUser  string                        `json:"remoteSshUser"`
	AppName        string                        `json:"appName"`
	EnvVars        map[string]string             `json:"envVars"`
	ServiceAccount map[int64]string              `json:"serviceAccount"`
	InstallDir     string                        `json:"installDir"`
	SystemdDir     string                        `json:"systemdDir"`
	SourceDir      string                        `json:"sourceDir"`
	SourceBin      string                        `json:"sourceBin"`
	DestinationBin string                        `json:"destinationBin"`
}

func NewRemoteSystemdDeployer(hostname, sshUser, appName, sourceDir string, opts ...RemoteSystemdDeployerOptions) *RemoteSystemdDeployer {
	remoteDeployer := &RemoteSystemdDeployer{
		RemoteHostName: hostname,
		RemoteSshUser:  sshUser,
		AppName:        appName,
		SourceDir:      sourceDir,
	}

	for _, opt := range opts {
		opt(remoteDeployer)
	}
	return remoteDeployer
}

func WithEnvars(envVars map[string]string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdDeployer) {
		r.EnvVars = envVars
	}

}

func WithServiceAccount(s map[int64]string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdDeployer) {
		r.ServiceAccount = s
	}

}

func WithRemoteSshUser(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdDeployer) {
		r.RemoteSshUser = s
	}
}

func WithSourceDir(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdDeployer) {
		r.SourceDir = s
	}
}

func WithDestinationBin(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdDeployer) {
		r.DestinationBin = s
	}
}

func WithSystemdDir(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdDeployer) {
		r.SystemdDir = s
	}
}

func WithInstallDir(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdDeployer) {
		r.InstallDir = s
	}
}

func WithSourceBin(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdDeployer) {
		r.SourceBin = s
	}
}

func (r *RemoteSystemdDeployer) StartSshDeploymentAgent(sshKey, sshPassphrase string, EnvVars map[string]string, useSshAgent bool, sshPort uint) error {
	client, err := ssh.InitializeRemoteSshAgent(
		r.RemoteHostName,
		r.RemoteSshUser,
		sshKey,
		sshPassphrase,
		EnvVars,
		useSshAgent,
		sshPort,
	)
	if err != nil {
		slog.Error("error initializing ssh client during RemoteSystemd deployment", slog.String("error", err.Error()))
		return fmt.Errorf("error initialize ssh client prior to RemoteSystemdDeployer %w", err)
	}

	r.SshClient = client
	return nil
}

func (r *RemoteSystemdDeployer) InstallApplication() error {
	slog.Info("Starting remote deployment")

	var err error
	var sourceBinPath string

	switch r.SourceBin {
	case ".", "./", "":
		sourceBinPath, err = os.Getwd()
		if err != nil {
			return err
		}
	default:
		slog.Info("Made it here")
		sourceBinPath, err = filepath.Abs(r.SourceBin)
		if err != nil {
			slog.Error("Error retirieving filepath.Abs from SourceBin", "error", err.Error())
			return err
		}
	}

	sourceBaseName := filepath.Base(sourceBinPath)
	slog.Info("Using for app-name", slog.String("SourceBaseDir", sourceBaseName))
	r.AppName = sourceBaseName
	sudo := true
	err = r.MakeInstallDir(sudo)
	if err != nil {
		return fmt.Errorf("error creating remote path %w", err)
	}
	slog.Info("Creating install dir on remote host", slog.String("install-dir", r.InstallDir), slog.String("remote-host", r.RemoteHostName))

	err = r.SshClient.RunCommand(mkdirCmdBase, mkdirArgsWithPath(remoteUtilsPath))
	if err != nil {
		return fmt.Errorf("error creating remote utils path %w", err)
	}

	err = r.SshClient.Upload(sourceBinPath, r.InstallDir)
	if err != nil {
		return fmt.Errorf("error uploading source bin %w", err)
	}

	err = r.SshClient.Upload(deployUtilsPath, remoteUtilsPath)
	if err != nil {
		return fmt.Errorf("error uploading utils %w", err)
	}

	for uid, username := range r.ServiceAccount {
		output, err := r.SshClient.RunCommandAndCaptureOutput(remoteValidateUserBaseCmd, []string{username, fmt.Sprintf("%d", uid)})
		if err != nil {
			return fmt.Errorf("error validating remote service user/uid: %w", err)
		}

		fmt.Println("validate command", string(output))
	}

	r.CreateUserOnRemote()

	return nil
}

func (r *RemoteSystemdDeployer) CreateUserOnRemote() error {
	if r.ServiceAccount == nil {
		return fmt.Errorf("No remote ServiceAccount has bee configured for RemoteSystemdDeployer")
	}
	for uid, username := range r.ServiceAccount {
		err := r.SshClient.RunCommand("useradd", []string{"-m", username})
		if err != nil {
			slog.Error("Failed to create user:", slog.String("ServiceUser", username), slog.Int64("uid", uid), slog.String("error", err.Error()))
			return fmt.Errorf("failed to create user: %w", err)
		}
	}

	return nil
}

func (r *RemoteSystemdDeployer) MakeInstallDir(sudo bool) error {
	argsDirs := []string{r.InstallDir, "/temp/utils"}
	for _, path := range argsDirs {
		if sudo {
			output, err := r.SshClient.RunCommandAndCaptureOutput("sudo", []string{"mkdir", "-p", path})
			if err != nil {
				slog.Error("Error created Install Dir", "error", err.Error())
				return err
			}
			fmt.Println(string(output))
			return err

		} else {
			output, err := r.SshClient.RunCommandAndCaptureOutput("mkdir", []string{"-p", path})
			if err != nil {
				slog.Error("Error created Install Dir", "error", err.Error())
				return err
			}
			fmt.Println(string(output))
			return err
		}
	}
	return nil
}
func mkdirArgsWithPath(path string) []string {
	retVal := make([]string, 0, 2)
	retVal = append(retVal, mkdirArgs)
	retVal = append(retVal, path)
	fmt.Println(retVal)
	return retVal
}
