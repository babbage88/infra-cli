package bumper

import (
	"fmt"
	"strconv"
	"strings"
)

// BumpVersion takes a semantic version string (e.g., "v1.0.13") and an increment
// type ("major", "minor", or "patch"). It returns the new version string after bumping
// the specified part. If the increment type is unrecognized or omitted, it defaults to "patch".
func BumpVersion(currentVersion, increment string) (string, error) {
	// Remove a leading "v" if present.
	version := currentVersion
	version = strings.TrimPrefix(version, "v")

	// Split the version string by "."
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return "", fmt.Errorf("version string does not match expected format (e.g., v1.0.13)")
	}

	// Convert parts to integers.
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return "", fmt.Errorf("invalid major version: %v", err)
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return "", fmt.Errorf("invalid minor version: %v", err)
	}

	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return "", fmt.Errorf("invalid patch version: %v", err)
	}

	// Increment the chosen field.
	switch increment {
	case "major":
		major++
		minor = 0
		patch = 0
	case "minor":
		minor++
		patch = 0
	case "patch":
		fallthrough
	default:
		patch++
	}

	// Return the new version with a "v" prefix.
	newTag := fmt.Sprintf("v%d.%d.%d", major, minor, patch)
	fmt.Println(newTag)
	return newTag, err
}
