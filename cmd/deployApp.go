package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy a Go web application as a systemd service",
	RunE:  deployService,
}

// Struct for storing deployment flags
type DeployFlags struct {
	AppName     string   `mapstructure:"app-name"`
	BinaryDir   string   `mapstructure:"binary-dir"`
	EnvVars     []string `mapstructure:"env-vars"`
	ServiceUser string   `mapstructure:"service-user"`
	BinaryName  string   `mapstructure:"binary-name"`
	InstallDir  string   `mapstructure:"install-dir"`
	SystemdDir  string   `mapstructure:"systemd-dir"`
	SourceDir   string   `mapstructure:"source-dir"`
}

var deployFlags DeployFlags

// init function to define the command flags and bind them with viper
func init() {
	rootCmd.AddCommand(deployCmd)

	// Define flags here
	deployCmd.Flags().StringVarP(&deployFlags.AppName, "app-name", "a", "", "The name of the application")
	deployCmd.Flags().StringVarP(&deployFlags.BinaryDir, "binary-dir", "b", "/etc/appname", "Directory where the binary will be placed")
	deployCmd.Flags().StringArrayVarP(&deployFlags.EnvVars, "env-vars", "e", nil, "List of environment variables to set for the systemd service")
	deployCmd.Flags().StringVarP(&deployFlags.ServiceUser, "service-user", "u", "appuser", "User to run the service")
	deployCmd.Flags().StringVarP(&deployFlags.BinaryName, "binary-name", "n", "appname", "Name of the compiled binary")
	deployCmd.Flags().StringVarP(&deployFlags.InstallDir, "install-dir", "i", "/etc/appname", "Directory to install the binary")
	deployCmd.Flags().StringVarP(&deployFlags.SystemdDir, "systemd-dir", "s", "/etc/systemd/system", "Directory where systemd service files will be stored")
	deployCmd.Flags().StringVarP(&deployFlags.SourceDir, "source-dir", "d", ".", "Source directory to build the application")

	// Bind the flags with viper
	viper.BindPFlags(deployCmd.Flags())
}

func deployService(cmd *cobra.Command, args []string) error {
	// 1. Validate input
	if deployFlags.AppName == "" {
		return fmt.Errorf("application name is required")
	}

	// 2. Create user for the service if not exists
	if err := createUser(deployFlags.ServiceUser); err != nil {
		return err
	}

	// 3. Build the binary
	if err := buildBinary(deployFlags.SourceDir, deployFlags.InstallDir, deployFlags.BinaryName); err != nil {
		return err
	}

	// 4. Set up environment variables in systemd unit file
	if err := createSystemdUnit(deployFlags); err != nil {
		return err
	}

	// 5. Install, enable, and start the service
	return manageSystemdService(deployFlags)
}

func createUser(serviceUser string) error {
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

func createSystemdUnit(flags DeployFlags) error {
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

func manageSystemdService(flags DeployFlags) error {
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
