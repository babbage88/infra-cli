package main

import (
	"fmt"
	"log/slog"
	"os/exec"
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
