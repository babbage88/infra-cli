package deployer

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"time"

	"github.com/babbage88/infra-cli/internal/archiver"
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
	sudoCmd                   string = "sudo"
)

type AppDeployer interface {
	InstallApplication() error
	ConfigureService(hostName string, serviceAccount map[int64]string, envVars map[string]string)
}

type RemoteSystemdDeployerOptions func(r *RemoteSystemdDeployer)

type RemoteSystemdDeployer struct {
	SshClient      *ssh.RemoteAppDeploymentAgent `json:"sshClient"`
	Archiver       archiver.Archiver             `json:"archiver"`
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

	// Require SourceBin
	if r.SourceBin == "" {
		return fmt.Errorf("source-bin must be provided")
	}

	sourceBinPath, err = filepath.Abs(r.SourceBin)
	if err != nil {
		slog.Error("Error retrieving filepath.Abs from SourceBin", "error", err.Error())
		return err
	}

	// Validate that it is a file
	stat, err := os.Stat(sourceBinPath)
	if err != nil {
		return fmt.Errorf("could not stat SourceBin: %w", err)
	}
	if stat.IsDir() {
		return fmt.Errorf("SourceBin must be a file, not a directory: %s", sourceBinPath)
	}

	if r.AppName == "" {
		sourceBaseName := filepath.Base(sourceBinPath)
		r.AppName = sourceBaseName
	}
	slog.Info("Using for app-name", slog.String("AppName", r.AppName))

	// 👉 new logic to create a dynamic /tmp path
	now := time.Now()
	timestamp := now.Format("020106_150405") // DDMMYY_HHmmss
	remoteTmpBase := fmt.Sprintf("/tmp/%s", timestamp)
	remoteUtilsPath := filepath.Join(remoteTmpBase, "utils")
	remoteValidateUserCmd := filepath.Join(remoteUtilsPath, "remote_utils", "validate-user")

	slog.Info("Remote temp path for utils", slog.String("remote-utils-path", remoteUtilsPath))

	// Create remote install dir
	sudo := true
	err = r.MakeInstallDir(sudo)
	if err != nil {
		return fmt.Errorf("error creating remote path %w", err)
	}

	// Create /tmp/{timestamp}/utils on remote
	err = r.SshClient.RunCommand(mkdirCmdBase, mkdirArgsWithPath(remoteUtilsPath))
	if err != nil {
		return fmt.Errorf("error creating remote utils path %w", err)
	}

	// Upload application binary
	err = r.UploadAndMove(sourceBinPath, r.InstallDir, true)
	if err != nil {
		return fmt.Errorf("error uploading source bin %w", err)
	}

	// Upload utils
	err = r.SshClient.Upload(deployUtilsPath, remoteUtilsPath)
	if err != nil {
		return fmt.Errorf("error uploading utils %w", err)
	}

	// Validate service user
	for uid, username := range r.ServiceAccount {
		output, err := r.SshClient.RunCommandAndCaptureOutput(remoteValidateUserCmd, []string{username, fmt.Sprintf("%d", uid)})
		if err != nil {
			return fmt.Errorf("error validating remote service user/uid: %w", err)
		}
		fmt.Println("validate command", string(output))
	}

	// Create service user if needed
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

// UploadAndMove uploads a file to a temporary directory under /tmp and moves it to the final destination using sudo.
// It ensures idempotency by creating a unique timestamped subdirectory for each upload, and cleans up the temp directory afterward.
func (r *RemoteSystemdDeployer) UploadAndMove(sourcePath, destinationPath string, modExecutable bool) error {

	stat, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to stat source path: %w", err)
	}
	if stat.IsDir() {
		return fmt.Errorf("UploadAndMove expects a file, got a directory: %s", sourcePath)
	}
	// Generate timestamped temp directory: /tmp/YYYYMMDD_HHmmss
	timestamp := time.Now().Format("20060102_150405")
	tmpDir := path.Join("/tmp", timestamp)

	// Create the temp directory on the remote server
	mkdirArgs := []string{"-p", tmpDir}
	err = r.SshClient.RunCommand(mkdirCmdBase, mkdirArgs)
	if err != nil {
		return fmt.Errorf("failed to create remote temp directory: %w", err)
	}

	// Extract the file name from sourcePath
	fileName := path.Base(sourcePath)
	tmpFilePath := path.Join(tmpDir, fileName)

	// Upload local file to the remote temp directory
	err = r.SshClient.Upload(sourcePath, tmpFilePath)
	if err != nil {
		return fmt.Errorf("failed to upload file to temp directory: %w", err)
	}

	// Move the file into the final destination using sudo
	moveCmdArgs := []string{"mv", tmpFilePath, destinationPath}
	slog.Info("moving from tmp to destination", slog.String("tmpFilePath", tmpFilePath), slog.String("dst", destinationPath))
	err = r.SshClient.RunCommand(sudoCmd, moveCmdArgs)
	if err != nil {
		return fmt.Errorf("failed to move file to destination with sudo: %w", err)
	}

	if modExecutable {
		// Optional: Set executable bit if needed
		chmodCmdArgs := []string{"chmod", "+x", destinationPath}
		slog.Info("setting destination executable", slog.String("dst", destinationPath))
		err = r.SshClient.RunCommand(sudoCmd, chmodCmdArgs)
		if err != nil {
			return fmt.Errorf("failed to chmod destination file: %w", err)
		}
	}

	// Clean up the temporary upload directory
	cleanupCmdArgs := []string{"rm", "-rf", tmpDir}
	slog.Info("cleaning up the tmp direcotry", slog.String("tmpDir", tmpDir))
	err = r.SshClient.RunCommand(sudoCmd, cleanupCmdArgs)
	if err != nil {
		return fmt.Errorf("failed to clean up temporary directory: %w", err)
	}

	return nil
}

// MoveAndCopyDirectory uploads a local directory to a remote temporary path and copies its contents to the destination.
// It ensures idempotency by creating a unique timestamped directory under /tmp.
func (r *RemoteSystemdDeployer) MoveAndCopyDirectory(sourceDir, destinationDir string) error {
	// Generate a timestamped temp directory: /tmp/YYYYMMDD_HHmmss
	timestamp := time.Now().Format("20060102_150405")
	tmpDir := path.Join("/tmp", timestamp)

	// Create the temp directory on the remote server
	mkdirArgs := []string{"-p", tmpDir}
	err := r.SshClient.RunCommand(mkdirCmdBase, mkdirArgs)
	if err != nil {
		return fmt.Errorf("failed to create remote temp directory: %w", err)
	}

	// Upload the local directory to the remote temp directory
	tmpUploadPath := path.Join(tmpDir, filepath.Base(sourceDir))

	slog.Info("Uploading local directory", slog.String("sourceDir", sourceDir), slog.String("tmpUploadPath", tmpUploadPath))
	err = r.SshClient.Upload(sourceDir, tmpUploadPath)
	if err != nil {
		return fmt.Errorf("failed to upload directory to temp directory: %w", err)
	}

	// Ensure destination directory exists
	mkdirDestArgs := []string{"-p", destinationDir}
	slog.Info("Ensuring destination directory exists", slog.String("destinationDir", destinationDir))
	err = r.SshClient.RunCommand(sudoCmd, mkdirDestArgs)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy the uploaded directory into the final destination
	copyCmdArgs := []string{"cp", "-r", tmpUploadPath + "/.", destinationDir}
	slog.Info("Copying from tmp to destination", slog.String("tmpUploadPath", tmpUploadPath), slog.String("destinationDir", destinationDir))
	err = r.SshClient.RunCommand(sudoCmd, copyCmdArgs)
	if err != nil {
		return fmt.Errorf("failed to copy directory to destination: %w", err)
	}

	// Clean up the temporary upload directory
	cleanupCmdArgs := []string{"rm", "-rf", tmpDir}
	slog.Info("Cleaning up the tmp directory", slog.String("tmpDir", tmpDir))
	err = r.SshClient.RunCommand(sudoCmd, cleanupCmdArgs)
	if err != nil {
		return fmt.Errorf("failed to clean up temporary directory: %w", err)
	}

	return nil
}
