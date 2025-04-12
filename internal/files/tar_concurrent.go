package files

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type fileToArchive struct {
	Path     string
	RelPath  string
	Info     os.FileInfo
	LinkDest string
}

// matchAnyExclude checks if a relative path matches any exclude rule.
func matchAnyExclude(rel string, excludes []string) bool {
	for _, ex := range excludes {
		if ex == "" {
			continue
		}
		// If it's a glob pattern
		if strings.ContainsAny(ex, "*?[]") {
			matched, err := filepath.Match(ex, filepath.Base(rel))
			if err == nil && matched {
				return true
			}
		} else {
			// Basic directory or file match
			parts := strings.Split(rel, string(os.PathSeparator))
			for _, part := range parts {
				if part == ex {
					return true
				}
			}
		}
	}
	return false
}

// CreateTarGzWithExcludes tars and gzips the srcDir into outPath excluding paths.
func CreateTarGzWithExcludes(srcDir, outPath string, excludes []string) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	gz := gzip.NewWriter(outFile)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	var filesMu sync.Mutex
	var files []fileToArchive
	var wg sync.WaitGroup
	walkErr := make(chan error, 1)

	// Use goroutines to walk directories concurrently.
	walk := func(path string) {
		defer wg.Done()
		err := filepath.Walk(path, func(fpath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if fpath == srcDir {
				return nil
			}
			rel, err := filepath.Rel(srcDir, fpath)
			if err != nil {
				return err
			}
			// Skip if path matches any exclude rule
			if matchAnyExclude(rel, excludes) {
				if info.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}

			var linkDest string
			if info.Mode()&os.ModeSymlink != 0 {
				linkDest, err = os.Readlink(fpath)
				if err != nil {
					return err
				}
			}

			filesMu.Lock()
			files = append(files, fileToArchive{
				Path:     fpath,
				RelPath:  rel,
				Info:     info,
				LinkDest: linkDest,
			})
			filesMu.Unlock()
			return nil
		})
		if err != nil {
			walkErr <- err
		}
	}

	// Start concurrent file walks for directories.
	entries, err := os.ReadDir(srcDir)
	if err != nil {
		return fmt.Errorf("reading src dir: %w", err)
	}

	for _, entry := range entries {
		name := entry.Name()
		fullPath := filepath.Join(srcDir, name)
		if matchAnyExclude(name, excludes) {
			continue
		}
		wg.Add(1)
		go walk(fullPath)
	}

	// Wait for all goroutines to finish walking the directories
	go func() {
		wg.Wait()
		close(walkErr)
	}()

	// Handle errors from the walking phase
	if err := <-walkErr; err != nil {
		return fmt.Errorf("walking files: %w", err)
	}

	// Write files to the tar
	for _, f := range files {
		header, err := tar.FileInfoHeader(f.Info, f.LinkDest)
		if err != nil {
			return fmt.Errorf("creating tar header: %w", err)
		}
		header.Name = f.RelPath

		if err := tw.WriteHeader(header); err != nil {
			return fmt.Errorf("writing tar header: %w", err)
		}

		// Copy the file content to the tar
		if f.Info.Mode().IsRegular() {
			srcFile, err := os.Open(f.Path)
			if err != nil {
				return fmt.Errorf("opening file: %w", err)
			}
			_, err = io.Copy(tw, srcFile)
			srcFile.Close()
			if err != nil {
				return fmt.Errorf("copying file: %w", err)
			}
		}
		fmt.Println("Added:", f.RelPath)
	}

	return nil
}
