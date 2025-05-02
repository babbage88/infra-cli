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
	deployUtilsPath             string = "remote_utils/bin"
	deployUtilsTar              string = "remote_utils.tar.gz"
	validateUserUtilPath        string = "remote_utils/bin/validate-user"
	remoteValidateUserBaseCmd   string = "validate-user"
	remoteUserUtils             string = "user-utils"
	remoteUserUtilsUsernameFlag string = "-username"
	cpCmd                       string = "cp"
	cpCmdRecursiveFlag          string = "-r"
	rmCmd                       string = "rm"
	rmRecursiveForceFlag        string = "-rf"
	remoteUserUtilsUidFlag      string = "-uid"
	mkdirCmdBase                string = "mkdir"
	chmodCmdBase                string = "chmod"
	chownCmdBase                string = "chown"
	chmodFileExecutableArg      string = "+x"

	validateUsernameCmdFlag string = "-username"
	validateUidCmdFlag      string = "-uid"

	mkdirRecursivePflag string = "-p"
	sudoCmd             string = "sudo"
)

type AppDeployer interface {
	InstallApplication() error
	ConfigureService(hostName string, serviceAccount map[int64]string, envVars map[string]string)
}

type RemoteSystemdDeployerOptions func(r *RemoteSystemdBinDeployer)

type RemoteSystemdBinDeployer struct {
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

func NewRemoteSystemdDeployer(hostname, sshUser, appName, sourceDir string, opts ...RemoteSystemdDeployerOptions) *RemoteSystemdBinDeployer {
	remoteDeployer := &RemoteSystemdBinDeployer{
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
	return func(r *RemoteSystemdBinDeployer) {
		r.EnvVars = envVars
	}
}

func WithServiceAccount(s map[int64]string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.ServiceAccount = s
	}
}

func WithRemoteSshUser(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.RemoteSshUser = s
	}
}

func WithSourceDir(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.SourceDir = s
	}
}

func WithDestinationBin(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.DestinationBin = s
	}
}

func WithSystemdDir(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.SystemdDir = s
	}
}

func WithInstallDir(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.InstallDir = s
	}
}

func WithSourceBin(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.SourceBin = s
	}
}

func (r *RemoteSystemdBinDeployer) StartSshDeploymentAgent(sshKey, sshPassphrase string, EnvVars map[string]string, useSshAgent bool, sshPort uint) error {
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

func (r *RemoteSystemdBinDeployer) InstallApplication() error {
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
		return fmt.Errorf("No AppName specified has been specified.")
	}
	slog.Info("Using for app-name", slog.String("AppName", r.AppName))

	remoteUtilsPath := generateUniqueDestinationPath("/tmp", "utils")
	remoteTempApps := generateUniqueDestinationPath("/tmp", "apps")

	slog.Info("Remote temp path for utils", slog.String("remote-utils-path", remoteUtilsPath))

	// Create remote install dir
	sudo := true
	err = r.MakeInstallDir(sudo, []string{r.InstallDir, remoteUtilsPath, remoteTempApps})
	if err != nil {
		return fmt.Errorf("error creating remote path %w", err)
	}

	// Upload application binary
	err = r.UploadAndMoveFile(sourceBinPath, r.InstallDir)
	if err != nil {
		return fmt.Errorf("error uploading source bin %w", err)
	}

	err = r.chmodRemoteFileExecutable(filepath.Join(r.InstallDir, r.DestinationBin))
	if err != nil {
		return fmt.Errorf("error setting file executable file: %s error: %w", filepath.Join(r.InstallDir, r.DestinationBin), err)
	}

	// Upload utils
	err = r.SshClient.Upload(deployUtilsPath, remoteUtilsPath)
	if err != nil {
		return fmt.Errorf("error uploading utils %w", err)
	}

	err = r.chmodRemoteFileExecutable(filepath.Join(remoteUtilsPath, remoteValidateUserBaseCmd))
	if err != nil {
		return fmt.Errorf("error making remote_utils executable %w", err)
	}

	err = r.chmodRemoteFileExecutable(filepath.Join(remoteUtilsPath, remoteUserUtils))
	if err != nil {
		return fmt.Errorf("error making remote_utils executable %w", err)
	}

	// Validate service user
	for uid, username := range r.ServiceAccount {
		if err != nil {
			slog.Error("error encounter making validate-user executable", "error", err.Error())
			return fmt.Errorf("error making validate-user util executable %w", err)
		}
		remoteValidateUserCmdArgs := []string{
			filepath.Join(remoteUtilsPath, remoteValidateUserBaseCmd),
			validateUidCmdFlag,
			fmt.Sprintf("%d", uid),
			validateUsernameCmdFlag,
			username,
		}
		output, err := r.SshClient.RunCommandAndCaptureOutput(sudoCmd, remoteValidateUserCmdArgs)
		if err != nil {
			return fmt.Errorf("error validating remote service user/uid: %w", err)
		}
		fmt.Println("validate command", string(output))
	}

	// Create service user if needed
	userUtilsPath := filepath.Join(remoteUtilsPath, remoteUserUtils)
	r.CreateUserOnRemote(userUtilsPath)

	return nil
}

func (r *RemoteSystemdBinDeployer) CreateUserOnRemote(userUtilsPath string) error {
	if r.ServiceAccount == nil {
		return fmt.Errorf("No remote ServiceAccount has bee configured for RemoteSystemdDeployer")
	}
	for uid, username := range r.ServiceAccount {
		Uid := fmt.Sprintf("%d", uid)
		err := r.SshClient.RunCommand(sudoCmd, []string{userUtilsPath, remoteUserUtilsUsernameFlag, username, remoteUserUtilsUidFlag, Uid})
		if err != nil {
			slog.Error("Failed to create user:", slog.String("ServiceUser", username), slog.Int64("uid", uid), slog.String("error", err.Error()))
			return fmt.Errorf("failed to create user: %w", err)
		}
	}

	return nil
}

func (r *RemoteSystemdBinDeployer) MakeInstallDir(sudo bool, argsDirs []string) error {
	for _, path := range argsDirs {
		if sudo {
			output, err := r.SshClient.RunCommandAndCaptureOutput(sudoCmd, []string{mkdirCmdBase, mkdirRecursivePflag, path})
			if err != nil {
				slog.Error("Error created Install Dir", "error", err.Error())
				return err
			}
			fmt.Println(string(output))
			return err

		} else {
			output, err := r.SshClient.RunCommandAndCaptureOutput(mkdirCmdBase, []string{mkdirRecursivePflag, path})
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

// UploadAndMoveFile uploads a file to a temporary directory under /tmp and moves it to the final destination using sudo.
func (r *RemoteSystemdBinDeployer) UploadAndMoveFile(sourcePath, destinationPath string) error {
	stat, err := os.Stat(sourcePath)
	if err != nil {
		return fmt.Errorf("failed to stat source path: %w", err)
	}

	// If source is a directory, use MoveAndCopyDirectory
	if stat.IsDir() {
		slog.Info("Detected source as directory", slog.String("sourceDir", sourcePath))
		return r.UploadAndMoveDirectory(sourcePath, destinationPath)
	}

	tmpDir := generateUniqueDestinationPath("/tmp", "files2copy")

	// Create the temp directory on the remote server
	mkdirArgs := []string{mkdirRecursivePflag, tmpDir}
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

	// Clean up the temporary upload directory
	cleanupCmdArgs := []string{rmCmd, rmRecursiveForceFlag, tmpDir}
	slog.Info("cleaning up the tmp direcotry", slog.String("tmpDir", tmpDir))
	err = r.SshClient.RunCommand(sudoCmd, cleanupCmdArgs)
	if err != nil {
		return fmt.Errorf("failed to clean up temporary directory: %w", err)
	}

	return nil
}

// UploadAndMoveDirectory uploads a local directory to a remote temporary path and copies its contents to the destination.
// It ensures idempotency by creating a unique timestamped directory under /tmp.
func (r *RemoteSystemdBinDeployer) UploadAndMoveDirectory(sourceDir, destinationDir string) error {
	// Generate a timestamped temp directory: /tmp/YYYYMMDD_HHmmss
	tmpDir := generateUniqueDestinationPath("/tmp/", "appfiles")

	// Create the temp directory on the remote server
	mkdirArgs := []string{mkdirRecursivePflag, tmpDir}
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
	mkdirDestArgs := []string{mkdirRecursivePflag, destinationDir}
	slog.Info("Ensuring destination directory exists", slog.String("destinationDir", destinationDir))
	err = r.SshClient.RunCommand(sudoCmd, mkdirDestArgs)
	if err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Copy the uploaded directory into the final destination
	copyCmdArgs := []string{cpCmd, cpCmdRecursiveFlag, tmpUploadPath + "/.", destinationDir}
	slog.Info("Copying from tmp to destination", slog.String("tmpUploadPath", tmpUploadPath), slog.String("destinationDir", destinationDir))
	err = r.SshClient.RunCommand(sudoCmd, copyCmdArgs)
	if err != nil {
		return fmt.Errorf("failed to copy directory to destination: %w", err)
	}

	// Clean up the temporary upload directory
	cleanupCmdArgs := []string{rmCmd, rmRecursiveForceFlag, tmpDir}
	slog.Info("Cleaning up the tmp directory", slog.String("tmpDir", tmpDir))
	err = r.SshClient.RunCommand(sudoCmd, cleanupCmdArgs)
	if err != nil {
		return fmt.Errorf("failed to clean up temporary directory: %w", err)
	}

	return nil
}

func (r *RemoteSystemdBinDeployer) chmodRemoteFileExecutable(path string) error {
	chmodCmdArgs := []string{chmodCmdBase, chmodFileExecutableArg, path}
	err := r.SshClient.RunCommand(sudoCmd, chmodCmdArgs)
	if err != nil {
		return fmt.Errorf("failed to chmod destination file: %w", err)
	}
	return nil
}

func generateUniqueDestinationPath(baseDir string, subDir string) string {
	now := time.Now()
	unixTime := now.Unix()
	unixTimeStr := fmt.Sprintf("%d", unixTime)
	remoteTmpBase := fmt.Sprintf("%s/%s/%s", baseDir, unixTimeStr, subDir)
	remoteDynamicPath := filepath.Join(remoteTmpBase)
	return remoteDynamicPath
}
