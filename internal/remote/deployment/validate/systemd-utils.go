package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

func formatEnvVars(envVars map[string]string) string {
	// Format environment variables for systemd unit file
	var formattedVars []string
	for key, value := range envVars {
		envLine := fmt.Sprintf(`Environment="%s=%s"`, key, value)
		formattedVars = append(formattedVars, envLine)
	}
	return fmt.Sprintf("%s", formattedVars)
}

func createSystemdUnitOnLocal(appName, systemdDir, installDir, execBin, svcUser string, envVars map[string]string) error {
	// Create the systemd unit file
	unitFilePath := filepath.Join(systemdDir, fmt.Sprintf("%s.service", appName))
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
ExecStart=%s
WorkingDirectory=%s
User=%s
Group=%s
Restart=on-failure
Environment=%s

[Install]
WantedBy=multi-user.target
`, appName, execBin, installDir, svcUser, svcUser, formatEnvVars(envVars))

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

func manageSystemdServiceOnLocal(appName string) error {
	// Reload systemd to recognize the new service
	cmd := exec.Command("systemctl", "daemon-reload")
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to reload systemd daemon: %v", err)
	}

	// Enable the service to start on boot
	cmd = exec.Command("systemctl", "enable", fmt.Sprintf("%s.service", appName))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to enable service: %v", err)
	}

	// Start the service
	cmd = exec.Command("systemctl", "start", fmt.Sprintf("%s.service", appName))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start service: %v", err)
	}

	return nil
}
