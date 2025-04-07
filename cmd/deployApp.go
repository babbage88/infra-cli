package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"

	"github.com/babbage88/goph"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

func startSshClient(host string, user string, sshKeyPath string, sshPassphrase string) (*goph.Client, error) {
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

func getCurrentUserName() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	name := currentUser.Name
	return name, nil
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a Go web application as a systemd service",
	RunE:  deployServiceOnLocal,
}

// Struct for storing deployment flags
type DeployFlags struct {
	RemoteHostName   string   `mapstructure:"remote-host"`
	RemoteSshUser    string   `mapstructure:"remote-ssh-user"`
	AppName          string   `mapstructure:"app-name"`
	BinaryDir        string   `mapstructure:"binary-dir"`
	EnvVars          []string `mapstructure:"env-vars"`
	ServiceUser      string   `mapstructure:"service-user"`
	ServiceUid       int64    `mapstructure:"service-uid"`
	BinaryName       string   `mapstructure:"binary-name"`
	InstallDir       string   `mapstructure:"install-dir"`
	SystemdDir       string   `mapstructure:"systemd-dir"`
	SourceDir        string   `mapstructure:"source-dir"`
	RemoteDeployment bool     `mapstructure:"remote-deployment"`
}

var deployFlags DeployFlags

const validateUserUtilPath string = "remote_utils/validate-user"

func (d *DeployFlags) copyUserValidateToRemote(client *goph.Client) error {
	err := client.Upload(validateUserUtilPath, "/tmp/validate-user")
	if err != nil {
		return err
		// return fmt.Errorf("error deploying remote_utils src: %s dst: %s err: %w", validateUserUtilPath, "/tmp/validate-user", error)
	}
	return nil
}

// init function to define the command flags and bind them with viper
func init() {
	curUser, _ := getCurrentUserName()

	rootCmd.AddCommand(deployCmd)

	// Define flags here
	deployCmd.Flags().StringVarP(&deployFlags.AppName, "app-name", "a", "", "The name of the application")
	deployCmd.Flags().StringVar(&deployFlags.BinaryDir, "binary-dir", "/etc/appname", "Directory where the binary will be placed")
	deployCmd.Flags().StringArrayVar(&deployFlags.EnvVars, "env-vars", nil, "List of environment variables to set for the systemd service")
	deployCmd.Flags().StringVar(&deployFlags.ServiceUser, "service-user", "appuser", "User to run the service")
	deployCmd.Flags().Int64Var(&deployFlags.ServiceUid, "service-uid", 8888, "UID for service account to run the service")
	deployCmd.Flags().StringVar(&deployFlags.BinaryName, "binary-name", "appname", "Name of the compiled binary")
	deployCmd.Flags().StringVar(&deployFlags.InstallDir, "install-dir", "/etc/appname", "Directory to install the binary")
	deployCmd.Flags().StringVar(&deployFlags.SystemdDir, "systemd-dir", "/etc/systemd/system", "Directory where systemd service files will be stored")
	deployCmd.Flags().StringVar(&deployFlags.SourceDir, "source-dir", ".", "Source directory to build the application")
	deployCmd.Flags().StringVar(&deployFlags.RemoteHostName, "remote-host", ".", "Remote Hostname to deploy application to")
	deployCmd.Flags().BoolVar(&deployFlags.RemoteDeployment, "remote-deployment", true, "Select Remote destination Host, done via ssh.")
	deployCmd.Flags().StringVar(&deployFlags.RemoteSshUser, "remote-ssh-user", curUser, "Remote SSH user to connect with")

	// Bind the flags with viper
	viper.BindPFlags(deployCmd.Flags())
}

func deployServiceToRemoteHost(cmd *cobra.Command, args []string) error {
	var err error = nil
	// 1. Validate input
	if deployFlags.AppName == "" {
		var sourceAbsoluteDir string
		log.Printf("No application name specified, attempting to parse from source-dir name\n")

		switch deployFlags.SourceDir {
		case ".", "./", "":
			sourceAbsoluteDir, err = os.Getwd()
			if err != nil {
				return err
			}
		default:
			sourceAbsoluteDir = filepath.Dir(deployFlags.SourceDir)
		}

		sourceBaseName := filepath.Base(sourceAbsoluteDir)
		log.Printf("Using %s for app-name", sourceBaseName)
		deployFlags.AppName = sourceBaseName
	}

	return err
}

func deployServiceOnLocal(cmd *cobra.Command, args []string) error {
	// 1. Validate input
	if deployFlags.AppName == "" {
		return fmt.Errorf("application name is required")
	}

	// 2. Create user for the service if not exists
	if err := createUserOnLocal(deployFlags.ServiceUser); err != nil {
		return err
	}

	// 3. Build the binary
	if err := buildBinary(deployFlags.SourceDir, deployFlags.InstallDir, deployFlags.BinaryName); err != nil {
		return err
	}

	// 4. Set up environment variables in systemd unit file
	if err := createSystemdUnitOnLocal(deployFlags); err != nil {
		return err
	}

	// 5. Install, enable, and start the service
	return manageSystemdServiceOnLocal(deployFlags)
}

func createUserOnRemote(serviceUser string, serviceUid int64) error {
	client, err := startSshClient(deployFlags.RemoteHostName, deployFlags.RemoteSshUser, rootViperCfg.GetString("ssh_key"), rootViperCfg.GetString("ssh_passphrase"))
	if err != nil {
		log.Printf("Error initializing ssh-client err: %s\n", err.Error())
		return err
	}

	createUserCmd, err := client.Command("useradd", "-m", serviceUser)
	if err := createUserCmd.Run(); err != nil {
		log.Printf("Failed to create user: %s err: %s\n", serviceUser, err.Error())
		return fmt.Errorf("failed to create user: %v", err)
	}

	return nil
}

func validateLocalUidUnamePair(serviceUser string, serviceUid int64) error {
	return validateSvcUserInput(serviceUser, serviceUid)
}

/*
func validateRemoteUidUnamePair(serviceUser string, serviceUid int64) error {
	var unameExists bool = false
	var uidExists bool = false

	client, err := startSshClient(deployFlags.RemoteHostName, deployFlags.RemoteSshUser, rootViperCfg.GetString("ssh_key"), rootViperCfg.GetString("ssh_passphrase"))
	if err != nil {
		log.Printf("Error initializing ssh-client err: %s\n", err.Error())
		return err
	}

	checkUnameCmd, err := client.Command("id", "-u", serviceUser)
	if err != nil {
		log.Printf("Error creating remote command to check username exists err: %s\n", err.Error())
		return err
	}

	if err := checkUnameCmd.Run(); err == nil {
		log.Printf("Service user already exists: %s\n", serviceUser)
		unameExists = true // Username exists already, return no error
	}

	checkUidCmd, err := client.Command("getent", "passwd", fmt.Sprintf("%d", serviceUid))
	if err != nil {
		log.Printf("Error creating remote command to check if uid exists err: %s\n", err.Error())
		return err
	}

	if err := checkUidCmd.Run(); err == nil {
		log.Printf("Service user already exists: %s\n", serviceUser)
		uidExists = true // The specified uid already exists
	}
}
*/

func createUserOnLocal(serviceUser string) error {
	// Check if the user already exists
	cmd := exec.Command("id", "-u", serviceUser)
	if err := cmd.Run(); err == nil {
		return nil // User exists
	}

	// Create a new user
	cmd = exec.Command("useradd", "-m", serviceUser)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create user: %v", err)
	}
	return nil
}

func buildBinary(sourceDir, installDir, binaryName string) error {
	// Ensure the install directory exists
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %v", err)
	}

	// Build the binary from source
	cmd := exec.Command("go", "build", "-o", filepath.Join(installDir, binaryName), sourceDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build binary: %v", err)
	}

	// Ensure the binary is owned by the app user
	cmd = exec.Command("chown", deployFlags.ServiceUser, filepath.Join(installDir, binaryName))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set ownership: %v", err)
	}

	return nil
}

func createSystemdUnitOnLocal(flags DeployFlags) error {
	// Create the systemd unit file
	unitFilePath := filepath.Join(flags.SystemdDir, fmt.Sprintf("%s.service", flags.AppName))
	unitFile, err := os.Create(unitFilePath)
	if err != nil {
		return fmt.Errorf("failed to create systemd unit file: %v", err)
	}
	defer unitFile.Close()

	// Write systemd unit file content
	systemdContent := fmt.Sprintf(`[Unit]
Description=%s Service
After=network.target

[Service]
ExecStart=%s/%s
WorkingDirectory=%s
User=%s
Group=%s
Restart=on-failure
Environment=%s

[Install]
WantedBy=multi-user.target
`, flags.AppName, flags.InstallDir, flags.BinaryName, flags.InstallDir, flags.ServiceUser, flags.ServiceUser, formatEnvVars(flags.EnvVars))

	_, err = unitFile.WriteString(systemdContent)
	if err != nil {
		return fmt.Errorf("failed to write systemd unit file: %v", err)
	}

	// Set the correct permissions for the systemd unit
	cmd := exec.Command("chmod", "644", unitFilePath)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set systemd unit file permissions: %v", err)
	}

	return nil
}

func formatEnvVars(envVars []string) string {
	// Format environment variables for systemd unit file
	var formattedVars []string
	for _, envVar := range envVars {
		formattedVars = append(formattedVars, fmt.Sprintf("'%s'", envVar))
	}
	return fmt.Sprintf("%s", formattedVars)
}

func manageSystemdServiceOnLocal(flags DeployFlags) error {
	// Reload systemd to recognize the new service
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %v", err)
	}

	// Enable the service to start on boot
	cmd = exec.Command("systemctl", "enable", fmt.Sprintf("%s.service", flags.AppName))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable service: %v", err)
	}

	// Start the service
	cmd = exec.Command("systemctl", "start", fmt.Sprintf("%s.service", flags.AppName))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start service: %v", err)
	}

	return nil
}
