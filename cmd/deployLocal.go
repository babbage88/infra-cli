package cmd

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/babbage88/infra-cli/bob"
	"github.com/babbage88/infra-cli/internal/deployment/validate"
	"github.com/spf13/cobra"
)

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
	if err := bob.BuildAndDeployGoBinary(deployFlags.SourceDir, deployFlags.InstallDir, deployFlags.DestinationBinary, deployFlags.ServiceUser); err != nil {
		return err
	}

	// 4. Set up environment variables in systemd unit file
	if err := createSystemdUnitOnLocal(deployFlags); err != nil {
		return err
	}

	// 5. Install, enable, and start the service
	return manageSystemdServiceOnLocal(deployFlags)
}

func validateLocalUidUnamePair(serviceUser string, serviceUid int64) error {
	err := validate.ValidateRemoteUidUnamePair(deployFlags.ServiceUser, deployFlags.ServiceUid)
	switch err.(type) {
	case *validate.KnownRemoteUserAndIdError:
		log.Println("Username and UID both exist and match.")
		return nil
	case *validate.RemoteUsernameExistsError:
		log.Println("Username or UID exists, but they do not match.")
		return err
	case nil:
		log.Println("Username and UID are a valid pair; neither currently exist.")
		return nil
	default:
		log.Printf("Unexpected error validating user: %s\n", err)
		return err
	}
}

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
`, flags.AppName, flags.InstallDir, flags.DestinationBinary, flags.InstallDir, flags.ServiceUser, flags.ServiceUser, formatEnvVars(flags.EnvVars))

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
