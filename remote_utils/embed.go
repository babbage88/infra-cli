package remoteutils

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

//go:embed bin
var embeddedRemoteUtils embed.FS

func ExtractToTempDir(goos, goarch string) (string, func(), error) {
	tempDir, err := os.MkdirTemp("", "infractl-remote-utils-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temp dir for embedded remote utils: %w", err)
	}

	cleanup := func() {
		_ = os.RemoveAll(tempDir)
	}

	sourceFS, err := selectEmbeddedUtilsFS(goos, goarch)
	if err != nil {
		cleanup()
		return "", nil, err
	}

	err = fs.WalkDir(sourceFS, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		data, err := fs.ReadFile(sourceFS, path)
		if err != nil {
			return fmt.Errorf("read embedded remote util %q: %w", path, err)
		}

		targetPath := filepath.Join(tempDir, path)
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return fmt.Errorf("create temp directory for %q: %w", path, err)
		}

		if err := os.WriteFile(targetPath, data, 0o755); err != nil {
			return fmt.Errorf("write embedded remote util %q: %w", path, err)
		}

		return nil
	})
	if err != nil {
		cleanup()
		return "", nil, err
	}

	return tempDir, cleanup, nil
}

func selectEmbeddedUtilsFS(goos, goarch string) (fs.FS, error) {
	platformDir := fmt.Sprintf("bin/%s-%s", goos, goarch)
	platformFS, err := fs.Sub(embeddedRemoteUtils, platformDir)
	if err == nil {
		return platformFS, nil
	}

	legacyFS, legacyErr := fs.Sub(embeddedRemoteUtils, "bin")
	if legacyErr == nil {
		return legacyFS, nil
	}

	return nil, fmt.Errorf("embedded remote utils for %s/%s not found", goos, goarch)
}
