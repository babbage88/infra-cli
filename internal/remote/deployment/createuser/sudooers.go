package main

import (
	"bufio"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
)

// Represents a parsed sudoers entry
type SudoersEntry struct {
	UserOrGroup string
	IsGroup     bool
}

// Load and parse all sudoers entries
func parseSudoersFiles(username string, groups []string) (bool, error) {
	filesToParse := []string{"/etc/sudoers"}

	// Also parse /etc/sudoers.d/* files
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

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		entries, err := extractSudoersEntries(line)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			// Check if direct user match
			if !entry.IsGroup && entry.UserOrGroup == username {
				return true, nil
			}
			// Check if group match
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
