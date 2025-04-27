package main

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strings"
)

func buildGroupAddCommand(isRoot bool, canSudo bool, gid int64, groupName string) (string, []string, error) {
	gidStr := fmt.Sprintf("%d", gid)
	var cmdBase string
	var cmdArgs []string
	if isRoot {
		cmdBase = groupaddCmd
		cmdArgs = []string{groupaddGidFlag, gidStr, groupName}
		return cmdBase, cmdArgs, nil
	} else if canSudo {
		cmdBase = sudoCmd
		cmdArgs = []string{groupaddCmd, groupaddGidFlag, gidStr, groupName}
		return cmdBase, cmdArgs, nil
	} else {
		cmdBase = sudoCmd
		cmdArgs = []string{groupaddCmd, groupaddGidFlag, gidStr, groupName}
		return cmdBase, cmdArgs, fmt.Errorf("current user has no permissions to create groups")
	}
}

// CreateGroup creates a new group with a specified name and GID.
func CreateGroup(groupName string, gid int64) error {
	currentUser, err := getCurrentUserInfo()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	isRoot := currentUser.Uid == rootUidStr || currentUser.Gid == rootGidStr || currentUser.Username == rootUsername
	canSudo := checkUserHasSudo(currentUser)

	cmdBase, cmdArgs, err := buildGroupAddCommand(isRoot, canSudo, gid, groupName)
	if err != nil {
		slog.Error("")
	}
	cmd := exec.Command(cmdBase, cmdArgs...)
	slog.Info("Running group creation command", "cmd", cmdBase, "args", cmdArgs)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to create group %s: %w", groupName, err)
	}

	slog.Info("Group created successfully", "groupName", groupName, "gid", gid)
	return nil
}

// AddUserToGroup adds an existing user to an existing group.
func AddUserToGroup(username, groupName string) error {
	currentUser, err := getCurrentUserInfo()
	if err != nil {
		return fmt.Errorf("failed to get current user: %w", err)
	}
	isRoot := currentUser.Uid == rootUidStr
	canSudo := checkUserHasSudo(currentUser)

	var cmdBase string
	var cmdArgs []string
	if isRoot {
		cmdBase = usermodCmd
		cmdArgs = []string{"-aG", groupName, username}
	} else if canSudo {
		cmdBase = sudoCmd
		cmdArgs = []string{usermodCmd, "-aG", groupName, username}
	} else {
		return fmt.Errorf("current user has no permissions to modify users")
	}

	cmd := exec.Command(cmdBase, cmdArgs...)
	slog.Info("Running usermod command", "cmd", cmdBase, "args", cmdArgs)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to add user %s to group %s: %w", username, groupName, err)
	}

	slog.Info("User added to group successfully", "username", username, "groupName", groupName)
	return nil
}

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
		cmds := strings.Join(allowedCommands, ", ")
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
