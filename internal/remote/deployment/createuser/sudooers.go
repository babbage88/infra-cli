package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"golang.org/x/term"
)

// Represents a parsed sudoers entry
type SudoersEntry struct {
	UserOrGroup string
	IsGroup     bool
}

func checkUserHasSudo(u *user.User) bool {
	groupIDs, err := u.GroupIds()
	if err != nil {
		log.Fatalf("Failed to get user's group IDs: %v", err)
	}

	var groups []string
	for _, gid := range groupIDs {
		group, err := user.LookupGroupId(gid)
		if err == nil {
			groups = append(groups, group.Name)
		}
	}

	// Step 1: Check if user is in known sudo groups
	for _, group := range groups {
		if group == sudoGroupname || group == wheelGroupname || group == adminGroupname {
			fmt.Println("User likely has sudo privileges (belongs to sudo/wheel/admin group).")
			return true
		}
	}

	// Step 2: Parse /etc/sudoers and /etc/sudoers.d/*
	hasSudoersSudo, err := parseSudoersFiles(u.Username, groups)
	if err != nil {
		fmt.Println("Warning: Error parsing sudoers files:", err)
	}
	if hasSudoersSudo {
		fmt.Println("User has sudo privileges according to sudoers file entries.")
		return true
	}

	// Step 3: Final fallback - Try running a safe sudo command
	fmt.Println("Performing a sudo command test to verify...")
	if trySudoLsCommand() {
		fmt.Println("User successfully executed sudo command (likely has sudo privileges).")
		return true
	}

	fmt.Println("User does NOT appear to have sudo privileges.")
	return false
}

func trySudoLsCommand() bool {
	cmd := exec.Command(sudoCmd, trySudoArgs...)
	err := cmd.Run()

	if err == nil {
		// Success without password
		return true
	}

	// If the error is due to a password prompt required
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
		fmt.Println("Sudo requires a password. Prompting user...")

		// Prompt for password
		fmt.Print("Password: ")
		passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
		fmt.Println()
		if err != nil {
			log.Printf("Failed to read password input: %v", err)
			return false
		}
		password := string(passwordBytes)

		return trySudoWithPassword(password)
	}

	if errors.As(err, &exitErr) && exitErr.ExitCode() == 127 {
		fmt.Println("Warning: sudo command not found.")
		return false
	}

	// Some other error
	return false
}

func trySudoWithPassword(password string) bool {
	cmd := exec.Command("sudo", "-S", "ls", "/etc")
	cmd.Stdin = bytes.NewBufferString(password + "\n")
	cmd.Stdout = nil
	cmd.Stderr = nil

	err := cmd.Run()
	return err == nil
}

// Load and parse all sudoers entries
func parseSudoersFiles(username string, groups []string) (bool, error) {
	filesToParse := []string{"/etc/sudoers"}

	sudoersDEntries, err := filepath.Glob("/etc/sudoers.d/*")
	if err != nil {
		return false, fmt.Errorf("failed to read sudoers.d directory: %w", err)
	}
	filesToParse = append(filesToParse, sudoersDEntries...)

	for _, filePath := range filesToParse {
		hasSudo, err := parseSudoersFile(filePath, username, groups)
		if err != nil {
			slog.Warn("Error parsing line is sudoers, continuing", slog.String("file", filePath))
			continue
		}
		if hasSudo {
			slog.Info("Founder sudoers entry.", slog.String("file", filePath), slog.String("username", username))
			return true, nil
		}
	}
	return false, nil
}

// Parse a single sudoers file
func parseSudoersFile(filePath string, username string, groups []string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, fmt.Errorf("unable to open sudoers file %s: %w", filePath, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := cleanSudoersLine(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		entries, err := extractSudoersEntries(line)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsGroup && entry.UserOrGroup == username {
				return true, nil
			}
			if entry.IsGroup && contains(groups, entry.UserOrGroup) {
				return true, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return false, fmt.Errorf("error reading sudoers file %s: %w", filePath, err)
	}
	return false, nil
}

// Remove comments and trim spaces
func cleanSudoersLine(line string) string {
	// Remove everything after a comment
	if idx := strings.Index(line, "#"); idx != -1 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

// Extracts sudoers entries from a cleaned line
func extractSudoersEntries(line string) ([]SudoersEntry, error) {
	if line == "" {
		return nil, errors.New("empty line")
	}

	// Split by whitespace to get the first "tokens" that might be users or groups
	tokens := strings.Fields(line)
	if len(tokens) == 0 {
		return nil, errors.New("no tokens")
	}

	var entries []SudoersEntry
	for _, token := range tokens {
		isGroup := false
		userOrGroup := token

		if strings.HasPrefix(token, "%") {
			isGroup = true
			userOrGroup = strings.TrimPrefix(token, "%")
			if userOrGroup == "" {
				// Skip invalid entry like just "%"
				continue
			}
		}

		if isValidIdentifier(userOrGroup) {
			entries = append(entries, SudoersEntry{
				UserOrGroup: userOrGroup,
				IsGroup:     isGroup,
			})
		}
	}

	if len(entries) == 0 {
		return nil, errors.New("no valid sudoers entries")
	}
	return entries, nil
}

// Check that the username or group name only contains safe characters
func isValidIdentifier(s string) bool {
	for _, r := range s {
		if !(r == '-' || r == '_' || ('a' <= r && r <= 'z') || ('A' <= r && r <= 'Z') || ('0' <= r && r <= '9')) {
			return false
		}
	}
	return true
}

// Safe contains helper
func contains(list []string, item string) bool {
	for _, v := range list {
		if v == item {
			return true
		}
	}
	return false
}
