package bob

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

/*
	func BuildGoBinary(sourceDir, binaryName string, verbose bool) error {
		args := make([]string, 0, 4)
		if verbose {
			args = []string{"build", "-v", "-o", binaryName}
		} else {
			args= []string{"build", "-o", binaryName}
		}
			// Build the binary from source
			cmd := exec.Command("go", args)
			if err := cmd.Run(); err != nil {
				slog.Error("error building go bin", "err", err.Error(), "source", sourceDir)
				return fmt.Errorf("failed to build binary: %v", err)
			}
		return nil
	}
*/
func BuildAndDeployGoBinary(sourceDir, installDir, binaryName, serviceUser string) error {
	// Ensure the install directory exists
	if err := os.MkdirAll(installDir, 0755); err != nil {
		return fmt.Errorf("failed to create install directory: %v", err)
	}

	// Build the binary from source
	cmd := exec.Command("go", "build", "-v", "-o", filepath.Join(installDir, binaryName), sourceDir)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to build binary: %v", err)
	}

	// Ensure the binary is owned by the app user
	cmd = exec.Command("chown", serviceUser, filepath.Join(installDir, binaryName))
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set ownership: %v", err)
	}

	return nil
}
