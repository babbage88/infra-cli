package deployer

import (
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"time"
)

func (r *RemoteSystemdDeployer) TarMoveCopy(sourceDir, destinationDir string, excludes []string, sudo bool) error {
	// Generate a timestamped tmp dir
	timestamp := time.Now().Format("20060102_150405")
	tmpDir := path.Join("/tmp", timestamp)

	// Local temporary tar.gz file
	tmpTarPath := filepath.Join(os.TempDir(), fmt.Sprintf("%s.tar.gz", timestamp))
	defer os.Remove(tmpTarPath) // Clean up local tar file

	// Step 1: Tar the local directory
	slog.Info("Creating tar.gz archive", slog.String("sourceDir", sourceDir), slog.String("tmpTarPath", tmpTarPath))
	err := r.Archiver.CreateTarGzWithExcludes(sourceDir, tmpTarPath, excludes)
	if err != nil {
		return fmt.Errorf("failed to create tar.gz: %w", err)
	}

	// Step 2: Create the /tmp/yyyymmdd_hhmmss remote directory
	slog.Info("Creating remote tmp directory", slog.String("tmpDir", tmpDir))
	err = r.SshClient.RunCommand(mkdirCmdBase, []string{"-p", tmpDir})
	if err != nil {
		return fmt.Errorf("failed to create remote tmp directory: %w", err)
	}

	// Step 3: Upload the tar.gz archive
	tmpRemoteTarPath := path.Join(tmpDir, "archive.tar.gz")
	slog.Info("Uploading archive to remote", slog.String("localTar", tmpTarPath), slog.String("remoteTar", tmpRemoteTarPath))
	err = r.SshClient.Upload(tmpTarPath, tmpRemoteTarPath)
	if err != nil {
		return fmt.Errorf("failed to upload tar.gz archive: %w", err)
	}

	// Step 4: Ensure destination directory exists
	destMkdirCmd := []string{"-p", destinationDir}
	if sudo {
		err = r.SshClient.RunCommand(sudoCmd, destMkdirCmd)
	} else {
		err = r.SshClient.RunCommand(mkdirCmdBase, destMkdirCmd)
	}
	if err != nil {
		return fmt.Errorf("failed to ensure destination directory: %w", err)
	}

	// Step 5: Extract archive at destination
	extractCmd := []string{"tar", "-xzf", tmpRemoteTarPath, "-C", destinationDir}
	slog.Info("Extracting archive at destination", slog.String("destinationDir", destinationDir))
	if sudo {
		err = r.SshClient.RunCommand(sudoCmd, extractCmd)
	} else {
		err = r.SshClient.RunCommand("", extractCmd)
	}
	if err != nil {
		return fmt.Errorf("failed to extract archive at destination: %w", err)
	}

	// Step 6: Fix ownership if sudo was used
	if sudo {
		chownCmdArgs := make([]string, 3)
		for uid, username := range r.ServiceAccount {
			args := []string{"-R", fmt.Sprintf("%d:%s", uid, username), destinationDir}
			chownCmdArgs = append(chownCmdArgs, args...)
			slog.Info("Fixing ownership", slog.String("destinationDir", destinationDir), slog.String("serviceUser", username))
			break
		}

		err = r.SshClient.RunCommand(sudoCmd, chownCmdArgs)
		if err != nil {
			return fmt.Errorf("failed to chown extracted files: %w", err)
		}
	}

	// Step 7: Clean up remote tmp dir
	cleanupCmd := []string{"-rf", tmpDir}
	slog.Info("Cleaning up remote tmp directory", slog.String("tmpDir", tmpDir))
	err = r.SshClient.RunCommand(sudoCmd, cleanupCmd)
	if err != nil {
		return fmt.Errorf("failed to clean up remote tmp directory: %w", err)
	}

	return nil
}
