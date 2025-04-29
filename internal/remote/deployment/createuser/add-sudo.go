package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

// CreateSudoersFile creates a sudoers file for a user or group, with optional command restrictions and NOPASSWD.
func CreateSudoersFile(name string, isGroup bool, allowedCommands []string, nopasswd bool) error {
	currentUser, err := getCurrentUserInfo()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	isRoot := currentUser.Uid == rootUidStr
	canSudo := checkUserHasSudo(currentUser)

	if !isRoot && !canSudo {
		return fmt.Errorf("current user has no permissions to create sudoers files")
	}

	targetName := name
	if isGroup {
		targetName = "%" + name
	}

	var sudoRule string
	if len(allowedCommands) > 0 {
		absoluteCommands := make([]string, 0, len(allowedCommands))
		for _, cmd := range allowedCommands {
			absPath, err := exec.LookPath(cmd)
			if err != nil {
				return fmt.Errorf("failed to find absolute path for command '%s': %w", cmd, err)
			}
			absoluteCommands = append(absoluteCommands, absPath)
		}

		cmds := strings.Join(absoluteCommands, ", ")
		if nopasswd {
			sudoRule = fmt.Sprintf("%s ALL=(ALL) NOPASSWD: %s", targetName, cmds)
		} else {
			sudoRule = fmt.Sprintf("%s ALL=(ALL) %s", targetName, cmds)
		}
	} else {
		if nopasswd {
			sudoRule = fmt.Sprintf("%s ALL=(ALL) NOPASSWD:ALL", targetName)
		} else {
			sudoRule = fmt.Sprintf("%s ALL=(ALL) ALL", targetName)
		}
	}

	sudoersFilePath := fmt.Sprintf("%s/%s%s", sudoersDirectory, sudoersFilePrefix, name)

	f, err := os.OpenFile(sudoersFilePath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0440)
	if err != nil {
		return fmt.Errorf("failed to open sudoers file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(sudoRule + "\n")
	if err != nil {
		return fmt.Errorf("failed to write sudoers file: %w", err)
	}

	slog.Info("Sudoers file created successfully", "file", sudoersFilePath, "rule", sudoRule)
	return nil
}
