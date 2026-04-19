package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	infraSSH "github.com/babbage88/infra-cli/ssh"
)

func buildPostgresURL(host string, port int, dbname, username, password string) string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=disable",
		urlQueryEscape(username),
		urlQueryEscape(password),
		host,
		port,
		urlQueryEscape(dbname),
	)
}

func urlQueryEscape(value string) string {
	replacer := strings.NewReplacer(
		"%", "%25",
		":", "%3A",
		"/", "%2F",
		"?", "%3F",
		"#", "%23",
		"[", "%5B",
		"]", "%5D",
		"@", "%40",
	)
	return replacer.Replace(value)
}

func runGooseyBinary(gooseyPath, dbURL string) error {
	gooseyPath = expandPath(gooseyPath)
	if gooseyPath == "" {
		return nil
	}

	if info, err := os.Stat(gooseyPath); err != nil {
		return fmt.Errorf("stat goosey binary %q: %w", gooseyPath, err)
	} else if info.IsDir() {
		return fmt.Errorf("goosey path %q is a directory", gooseyPath)
	}

	cmd := exec.Command(gooseyPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Env = append(
		os.Environ(),
		"DATABASE_URL="+dbURL,
		"GOOSE_DBSTRING="+dbURL,
	)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run goosey binary %q: %w", gooseyPath, err)
	}

	return nil
}

func maybeBuildRemoteGooseyBinary(gooseyPath string, buildRemote bool, goos, goarch string) (string, func(), error) {
	gooseyPath = expandPath(gooseyPath)
	if gooseyPath == "" || !buildRemote {
		return gooseyPath, func() {}, nil
	}

	sourceDir := filepath.Dir(gooseyPath)
	info, err := os.Stat(sourceDir)
	if err != nil {
		return "", nil, fmt.Errorf("stat goosey source directory %q: %w", sourceDir, err)
	}
	if !info.IsDir() {
		return "", nil, fmt.Errorf("goosey source directory %q is not a directory", sourceDir)
	}

	tmpFile, err := os.CreateTemp("", fmt.Sprintf("goosey-%s-%s-*", goos, goarch))
	if err != nil {
		return "", nil, fmt.Errorf("create temp goosey binary: %w", err)
	}
	tmpPath := tmpFile.Name()
	if err := tmpFile.Close(); err != nil {
		os.Remove(tmpPath)
		return "", nil, fmt.Errorf("close temp goosey binary: %w", err)
	}

	buildCmd := exec.Command("go", "build", "-o", tmpPath, ".")
	buildCmd.Dir = sourceDir
	buildCmd.Stdout = os.Stdout
	buildCmd.Stderr = os.Stderr
	buildCmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")

	if err := buildCmd.Run(); err != nil {
		os.Remove(tmpPath)
		return "", nil, fmt.Errorf("build remote goosey binary for %s/%s from %q: %w", goos, goarch, sourceDir, err)
	}

	cleanup := func() {
		_ = os.Remove(tmpPath)
	}

	return tmpPath, cleanup, nil
}

func runGooseyBinaryRemote(sshClient infraSSH.Client, gooseyPath, dbURL string) error {
	gooseyPath = expandPath(gooseyPath)
	if gooseyPath == "" {
		return nil
	}

	info, err := os.Stat(gooseyPath)
	if err != nil {
		return fmt.Errorf("stat goosey binary %q: %w", gooseyPath, err)
	}
	if info.IsDir() {
		return fmt.Errorf("goosey path %q is a directory", gooseyPath)
	}

	remotePath := filepath.ToSlash(filepath.Join("/tmp", fmt.Sprintf("goosey-%d", os.Getpid())))
	if err := sshClient.Upload(gooseyPath, remotePath); err != nil {
		return fmt.Errorf("upload goosey binary to remote host: %w", err)
	}

	cleanupCmd := fmt.Sprintf("rm -f %s", shellQuote(remotePath))
	defer func() {
		if _, cleanupErr := sshClient.Run(cleanupCmd); cleanupErr != nil {
			// Keep cleanup best-effort; primary caller will already handle main errors.
		}
	}()

	cmdStr := fmt.Sprintf(
		"chmod 755 %s && env DATABASE_URL=%s GOOSE_DBSTRING=%s %s",
		shellQuote(remotePath),
		shellQuote(dbURL),
		shellQuote(dbURL),
		shellQuote(remotePath),
	)

	out, err := sshClient.Run(cmdStr)
	if err != nil {
		output := strings.TrimSpace(string(out))
		if strings.Contains(output, "cannot execute binary file") {
			return fmt.Errorf(
				"remote goosey binary is not executable on the target host; this usually means it was built for the wrong OS/architecture. Build a Linux binary for the remote host and try again: %s",
				output,
			)
		}
		return formatSSHExecError(fmt.Errorf("run remote goosey binary: %w", err), out)
	}

	return nil
}
