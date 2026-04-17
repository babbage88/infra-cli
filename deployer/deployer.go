// Package deployer provides functionality for deploying applications to remote systems
// via SSH, including systemd service creation and environment configuration.
package deployer

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/babbage88/infra-cli/internal/archiver"
	remoteutils "github.com/babbage88/infra-cli/remote_utils"
	"github.com/babbage88/infra-cli/ssh"
)

const (
	deployUtilsPath             string = "remote_utils/bin"
	deployUtilsTar              string = "remote_utils.tar.gz"
	validateUserUtilPath        string = "remote_utils/bin/deploy-utils"
	remoteValidateUserBaseCmd   string = "deploy-utils"
	remoteSystemdBaseCmd        string = "deploy-utils"
	remoteSystemdNoVu           string = `--validate-user=false`
	remoteSystemdExecBinFlag    string = "--exec-bin"
	remoteSystemdEnableSvcFlag  string = "--enable-systemd=true"
	remoteSystemdAppNameFlag    string = "--app-name"
	remoteSystemdEnvVarsFlag    string = "--env-vars"
	remoteSystemdInstalldirFlag string = "--install-dir"
	remoteSystemdSvcUserFlag    string = "--svcuser"
	remoteSystemdDir            string = "--systemd-dir"
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

// AppDeployer defines the interface for application deployment operations.
// Implementations should handle the installation and configuration of applications
// on remote systems.
type AppDeployer interface {
	// InstallApplication performs the main deployment operation, including
	// uploading files and binaries to the correct locations.
	InstallApplication() error

	// ConfigureService sets up the service configuration for the deployed application,
	// including environment variables and service account settings.
	ConfigureService(hostName string, serviceAccount map[int64]string, envVars map[string]string)
}

// RemoteSystemdDeployerOptions is a function type used for configuring
// RemoteSystemdBinDeployer instances with optional parameters.
type RemoteSystemdDeployerOptions func(r *RemoteSystemdBinDeployer)

// RemoteSystemdBinDeployer handles the deployment of binary applications
// to remote systems with systemd service configuration.
type RemoteSystemdBinDeployer struct {
	SshClient      *ssh.RemoteAppDeploymentAgent `json:"sshClient"`      // SSH client for remote operations
	Archiver       archiver.Archiver             `json:"archiver"`       // Archiver for file compression
	RemoteHostName string                        `json:"remoteHost"`     // Target remote hostname
	RemoteSshUser  string                        `json:"remoteSshUser"`  // SSH username for remote host
	AppName        string                        `json:"appName"`        // Name of the application to deploy
	EnvVars        map[string]string             `json:"envVars"`        // Environment variables for the service
	ServiceAccount map[int64]string              `json:"serviceAccount"` // Service account mapping (UID -> username)
	InstallDir     string                        `json:"installDir"`     // Installation directory on remote host
	SystemdDir     string                        `json:"systemdDir"`     // Systemd unit file directory
	SourceDir      string                        `json:"sourceDir"`      // Local source directory
	SourceBin      string                        `json:"sourceBin"`      // Local source binary path
	DestinationBin string                        `json:"destinationBin"` // Destination binary name on remote
}

// NewRemoteSystemdDeployer creates a new RemoteSystemdBinDeployer instance
// with the specified basic configuration and optional settings.
//
// Parameters:
//   - hostname: The remote hostname to deploy to
//   - sshUser: The SSH username for the remote host
//   - appName: The name of the application to deploy
//   - sourceDir: The local source directory
//   - opts: Optional configuration functions
//
// Returns a configured RemoteSystemdBinDeployer instance.
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

// WithEnvars returns a configuration option that sets environment variables
// for the deployed service.
//
// Parameters:
//   - envVars: Map of environment variable names to values
//
// Returns a RemoteSystemdDeployerOptions function.
func WithEnvars(envVars map[string]string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.EnvVars = envVars
	}
}

// WithServiceAccount returns a configuration option that sets the service account
// mapping for the deployed service.
//
// Parameters:
//   - s: Map of UIDs to usernames for service accounts
//
// Returns a RemoteSystemdDeployerOptions function.
func WithServiceAccount(s map[int64]string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.ServiceAccount = s
	}
}

// WithRemoteSshUser returns a configuration option that sets the SSH user
// for remote operations.
//
// Parameters:
//   - s: The SSH username
//
// Returns a RemoteSystemdDeployerOptions function.
func WithRemoteSshUser(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.RemoteSshUser = s
	}
}

// WithSourceDir returns a configuration option that sets the source directory.
//
// Parameters:
//   - s: The local source directory path
//
// Returns a RemoteSystemdDeployerOptions function.
func WithSourceDir(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.SourceDir = s
	}
}

// WithDestinationBin returns a configuration option that sets the destination
// binary name on the remote host.
//
// Parameters:
//   - s: The destination binary name
//
// Returns a RemoteSystemdDeployerOptions function.
func WithDestinationBin(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.DestinationBin = s
	}
}

// WithSystemdDir returns a configuration option that sets the systemd unit file
// directory on the remote host.
//
// Parameters:
//   - s: The systemd directory path
//
// Returns a RemoteSystemdDeployerOptions function.
func WithSystemdDir(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.SystemdDir = s
	}
}

// WithInstallDir returns a configuration option that sets the installation
// directory on the remote host.
//
// Parameters:
//   - s: The installation directory path
//
// Returns a RemoteSystemdDeployerOptions function.
func WithInstallDir(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.InstallDir = s
	}
}

// WithSourceBin returns a configuration option that sets the local source
// binary path.
//
// Parameters:
//   - s: The local source binary path
//
// Returns a RemoteSystemdDeployerOptions function.
func WithSourceBin(s string) RemoteSystemdDeployerOptions {
	return func(r *RemoteSystemdBinDeployer) {
		r.SourceBin = s
	}
}

// StartSshDeploymentAgent initializes the SSH client for remote deployment operations.
// This method must be called before any deployment operations can be performed.
//
// Parameters:
//   - sshKey: Path to the SSH private key file
//   - sshPassphrase: Passphrase for the SSH key (if encrypted)
//   - useSshAgent: Whether to use SSH agent for key management
//   - sshPort: SSH port number for the remote host
//
// Returns an error if the SSH client initialization fails.
func (r *RemoteSystemdBinDeployer) StartSshDeploymentAgent(sshKey, sshPassphrase string, useSshAgent bool, sshPort uint) error {
	client, err := ssh.InitializeRemoteSshAgent(
		r.RemoteHostName,
		r.RemoteSshUser,
		sshKey,
		sshPassphrase,
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

// InstallApplication performs the complete deployment process for the application.
// This includes uploading the binary, setting up the service user, creating the
// systemd unit file, and configuring environment variables.
//
// The deployment process follows these steps:
// 1. Validates required parameters (SourceBin, AppName)
// 2. Creates remote directories
// 3. Uploads the application binary
// 4. Uploads deployment utilities
// 5. Validates and creates service users
// 6. Creates the systemd unit file on remote host
//
// Returns an error if any step in the deployment process fails.
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

	slog.Info("Remote temp path for utils", slog.String("remote-utils-path", remoteUtilsPath))

	// Create remote install dir
	sudo := true
	err = r.MakeInstallDir(sudo, []string{r.InstallDir, r.SystemdDir, remoteUtilsPath})
	if err != nil {
		return fmt.Errorf("error creating remote path %w", err)
	}

	// Upload application binary
	err = r.UploadAndMoveFile(sourceBinPath, filepath.Join(r.InstallDir, r.DestinationBin))
	if err != nil {
		return fmt.Errorf("error uploading source bin %w", err)
	}

	err = r.chmodRemoteFileExecutable(filepath.Join(r.InstallDir, r.DestinationBin))
	if err != nil {
		return fmt.Errorf("error setting file executable file: %s error: %w", filepath.Join(r.InstallDir, r.DestinationBin), err)
	}

	// Upload utils
	remoteGOOS, remoteGOARCH, err := r.detectRemotePlatform()
	if err != nil {
		return err
	}

	localDeployUtilsPath, cleanupDeployUtils, err := remoteutils.ExtractToTempDir(remoteGOOS, remoteGOARCH)
	if err != nil {
		return err
	}
	defer cleanupDeployUtils()

	err = r.SshClient.Upload(localDeployUtilsPath, remoteUtilsPath)
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
			slog.Error("error encounter making deploy-utils executable", "error", err.Error())
			return fmt.Errorf("error making deploy-utils util executable %w", err)
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
	if err := r.CreateUserOnRemote(userUtilsPath); err != nil {
		return err
	}
	if err := r.CreateUnitFileOnRemote(remoteUtilsPath); err != nil {
		return err
	}

	return nil
}

// CreateUserOnRemote creates the service user account on the remote system
// using the user-utils binary. This ensures the service has a dedicated user
// account with the specified UID and username.
//
// Parameters:
//   - userUtilsPath: Path to the user-utils binary on the remote system
//
// Returns an error if user creation fails.
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

// CreateUnitFileOnRemote creates the systemd unit file on the remote system.
// This method handles both environment file creation and systemd unit file generation.
//
// The process includes:
// 1. Creating an environment file with the specified variables
// 2. Uploading the env file to a temporary location
// 3. Moving it to the final location with sudo
// 4. Setting appropriate permissions
// 5. Running the deploy-utils binary to create the systemd unit file
//
// Parameters:
//   - utilsDir: Directory containing the deployment utilities on the remote system
//
// Returns an error if any step in the process fails.
func (r *RemoteSystemdBinDeployer) CreateUnitFileOnRemote(utilsDir string) error {
	// Write env file to remote first, using temp upload and sudo cp pattern
	if len(r.EnvVars) > 0 {
		envFileName := fmt.Sprintf("%s.env", r.AppName)
		tmpDir := generateUniqueDestinationPath("/tmp", "envfiles")
		tmpEnvFileRemote := path.Join(tmpDir, envFileName)
		finalEnvFileRemote := fmt.Sprintf("/etc/%s.env", r.AppName)

		var envFileContent strings.Builder
		keys := sortedEnvKeys(r.EnvVars)
		for _, key := range keys {
			value := r.EnvVars[key]
			envFileContent.WriteString(fmt.Sprintf("%s=%s\n", key, value))
		}
		tmpEnvFileLocal, err := writeTempEnvFile(envFileName, envFileContent.String())
		if err != nil {
			return err
		}
		defer os.Remove(tmpEnvFileLocal)
		// Create remote tmp dir
		err = r.SshClient.RunCommand(mkdirCmdBase, []string{mkdirRecursivePflag, tmpDir})
		if err != nil {
			return fmt.Errorf("failed to create remote tmp dir for env file: %w", err)
		}
		// Upload to remote tmp location
		err = r.SshClient.Upload(tmpEnvFileLocal, tmpEnvFileRemote)
		if err != nil {
			return fmt.Errorf("failed to upload env file to remote tmp: %w", err)
		}
		// Move to /etc/<app-name>.env with sudo
		err = r.SshClient.RunCommand(sudoCmd, []string{"cp", tmpEnvFileRemote, finalEnvFileRemote})
		if err != nil {
			return fmt.Errorf("failed to sudo cp env file to /etc: %w", err)
		}
		// Set permissions
		err = r.SshClient.RunCommand(sudoCmd, []string{"chmod", "600", finalEnvFileRemote})
		if err != nil {
			return fmt.Errorf("failed to chmod env file on remote: %w", err)
		}
		// Clean up remote tmp file
		err = r.SshClient.RunCommand(sudoCmd, []string{rmCmd, rmRecursiveForceFlag, tmpDir})
		if err != nil {
			return fmt.Errorf("failed to clean up remote tmp env dir: %w", err)
		}
	}
	cmd, args := r.systemdCmd(utilsDir)
	fmt.Printf("[DEBUG] About to run remote command: %s %v\n", cmd, args)
	output, err := r.SshClient.RunCommandAndCaptureOutput(cmd, args)
	if err != nil {
		slog.Error("Failed to create unit file on remote:", slog.String("error", err.Error()))
		return fmt.Errorf("failed to create unit file: %w", err)
	}
	fmt.Println(string(output))
	return nil
}

// MakeInstallDir creates directories on the remote system. It can create
// multiple directories in a single call and supports both sudo and non-sudo
// operations.
//
// Parameters:
//   - sudo: Whether to use sudo for directory creation
//   - argsDirs: Slice of directory paths to create
//
// Returns an error if directory creation fails.
func (r *RemoteSystemdBinDeployer) MakeInstallDir(sudo bool, argsDirs []string) error {
	for _, path := range argsDirs {
		if sudo {
			output, err := r.SshClient.RunCommandAndCaptureOutput(sudoCmd, []string{mkdirCmdBase, mkdirRecursivePflag, path})
			if err != nil {
				slog.Error("Error created Install Dir", "error", err.Error())
				return err
			}
			fmt.Println(string(output))
			continue

		} else {
			output, err := r.SshClient.RunCommandAndCaptureOutput(mkdirCmdBase, []string{mkdirRecursivePflag, path})
			if err != nil {
				slog.Error("Error created Install Dir", "error", err.Error())
				return err
			}
			fmt.Println(string(output))
			continue
		}
	}
	return nil
}

// UploadAndMoveFile uploads a file to a temporary directory under /tmp and moves it to the final destination using sudo.
// This method ensures secure file transfer by using temporary locations and sudo for final placement.
//
// The process includes:
// 1. Creating a temporary directory on the remote system
// 2. Uploading the file to the temporary location
// 3. Moving the file to the final destination with sudo
// 4. Cleaning up the temporary directory
//
// Parameters:
//   - sourcePath: Local path to the file to upload
//   - destinationPath: Final destination path on the remote system
//
// Returns an error if any step in the process fails.
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
//
// The process includes:
// 1. Creating a timestamped temporary directory on the remote system
// 2. Uploading the local directory to the temporary location
// 3. Ensuring the destination directory exists
// 4. Copying the contents to the final destination with sudo
// 5. Cleaning up the temporary directory
//
// Parameters:
//   - sourceDir: Local directory to upload
//   - destinationDir: Final destination directory on the remote system
//
// Returns an error if any step in the process fails.
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

// chmodRemoteFileExecutable sets the executable permission on a remote file
// using the chmod command with sudo privileges.
//
// Parameters:
//   - path: Path to the file on the remote system
//
// Returns an error if the chmod operation fails.
func (r *RemoteSystemdBinDeployer) chmodRemoteFileExecutable(path string) error {
	chmodCmdArgs := []string{chmodCmdBase, chmodFileExecutableArg, path}
	err := r.SshClient.RunCommand(sudoCmd, chmodCmdArgs)
	if err != nil {
		return fmt.Errorf("failed to chmod destination file: %w", err)
	}
	return nil
}

// generateUniqueDestinationPath creates a unique path for temporary file operations
// by combining a base directory, current Unix timestamp, and a subdirectory name.
// This ensures idempotent operations and prevents conflicts between concurrent deployments.
//
// Parameters:
//   - baseDir: Base directory (e.g., "/tmp")
//   - subDir: Subdirectory name for the specific operation
//
// Returns a unique path string in the format: baseDir/timestamp/subDir
func generateUniqueDestinationPath(baseDir string, subDir string) string {
	now := time.Now()
	unixTime := now.Unix()
	unixTimeStr := fmt.Sprintf("%d", unixTime)
	remoteTmpBase := fmt.Sprintf("%s/%s/%s", baseDir, unixTimeStr, subDir)
	remoteDynamicPath := filepath.Join(remoteTmpBase)
	return remoteDynamicPath
}

// formatEnvVarsForStringFlag returns the path to the environment file that will be
// created on the remote system. This path is used by the systemd unit file to
// load environment variables.
//
// Returns the path to the environment file in the format: /etc/<app-name>.env
func (r *RemoteSystemdBinDeployer) formatEnvVarsForStringFlag() string {
	// Instead of returning Environment= lines, return the path to the env file
	return fmt.Sprintf("/etc/%s.env", r.AppName)
}

// systemdCmd constructs the command and arguments for creating the systemd unit file
// on the remote system. It uses the deploy-utils binary with appropriate flags
// for the application configuration.
//
// Parameters:
//   - utilsDir: Directory containing the deployment utilities on the remote system
//
// Returns the command string and slice of arguments for the systemd unit file creation.
func (r *RemoteSystemdBinDeployer) systemdCmd(utilsDir string) (string, []string) {
	var cmd string = sudoCmd
	var args []string
	for _, value := range r.ServiceAccount {

		fullUtilsPath := filepath.Join(utilsDir, remoteSystemdBaseCmd)
		envFilePath := fmt.Sprintf("/etc/%s.env", r.AppName)
		execPath := filepath.Join(r.InstallDir, r.DestinationBin)
		args = []string{
			fullUtilsPath,
			remoteSystemdNoVu,
			remoteSystemdEnableSvcFlag,
			remoteSystemdAppNameFlag, r.AppName,
			remoteSystemdInstalldirFlag, r.InstallDir, remoteSystemdExecBinFlag, execPath,
			remoteSystemdEnvVarsFlag, envFilePath,
			remoteSystemdDir, r.SystemdDir,
			remoteSystemdSvcUserFlag, value,
		}
		break
	}

	return cmd, args
}

func sortedEnvKeys(envVars map[string]string) []string {
	keys := make([]string, 0, len(envVars))
	for key := range envVars {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func writeTempEnvFile(pattern, contents string) (string, error) {
	tmpFile, err := os.CreateTemp("", pattern)
	if err != nil {
		return "", fmt.Errorf("failed to create temp env file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.WriteString(contents); err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to write temp env file: %w", err)
	}

	return tmpFile.Name(), nil
}

func (r *RemoteSystemdBinDeployer) detectRemotePlatform() (string, string, error) {
	rawOS, err := r.SshClient.RunCommandAndCaptureOutput("uname", []string{"-s"})
	if err != nil {
		return "", "", fmt.Errorf("detect remote operating system: %w", err)
	}

	rawArch, err := r.SshClient.RunCommandAndCaptureOutput("uname", []string{"-m"})
	if err != nil {
		return "", "", fmt.Errorf("detect remote architecture: %w", err)
	}

	goos, err := normalizeRemoteGOOS(strings.TrimSpace(string(rawOS)))
	if err != nil {
		return "", "", err
	}

	goarch, err := normalizeRemoteGOARCH(strings.TrimSpace(string(rawArch)))
	if err != nil {
		return "", "", err
	}

	slog.Info("Detected remote platform", "goos", goos, "goarch", goarch, "host", r.RemoteHostName)
	return goos, goarch, nil
}

func (r *RemoteSystemdBinDeployer) DetectRemotePlatform() (string, string, error) {
	return r.detectRemotePlatform()
}

func normalizeRemoteGOOS(raw string) (string, error) {
	switch strings.ToLower(raw) {
	case "linux":
		return "linux", nil
	case "darwin":
		return "darwin", nil
	default:
		return "", fmt.Errorf("unsupported remote operating system %q", raw)
	}
}

func normalizeRemoteGOARCH(raw string) (string, error) {
	switch strings.ToLower(raw) {
	case "x86_64", "amd64":
		return "amd64", nil
	case "aarch64", "arm64":
		return "arm64", nil
	default:
		return "", fmt.Errorf("unsupported remote architecture %q", raw)
	}
}
