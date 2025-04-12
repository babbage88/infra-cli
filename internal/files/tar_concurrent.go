package files

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type FileToArchive struct {
	Path     string
	RelPath  string
	FileInfo os.FileInfo
}

// TarAndGzipFiles creates a tar.gz archive from the given source directory and writes it to the writer.
// This version is concurrent: it scans files and sends them through a channel to be processed by worker goroutines.
func TarAndGzipFiles(src string, buf io.Writer) error {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("could not resolve absolute path: %w", err)
	}

	gzw := gzip.NewWriter(buf)
	defer gzw.Close()

	tw := tar.NewWriter(gzw)
	defer tw.Close()

	filesChan := make(chan FileToArchive, 64)
	var wg sync.WaitGroup
	var copyErr error
	var mu sync.Mutex

	// Start worker
	wg.Add(1)
	go func() {
		defer wg.Done()
		for file := range filesChan {
			if err := addFileToTar(tw, absSrc, file); err != nil {
				mu.Lock()
				copyErr = err
				mu.Unlock()
				return
			}
		}
	}()

	// Walk the directory and enqueue files
	err = filepath.Walk(absSrc, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, err := filepath.Rel(absSrc, path)
		if err != nil {
			return err
		}
		// Skip the root
		if relPath == "." {
			return nil
		}
		filesChan <- FileToArchive{
			Path:     path,
			RelPath:  filepath.ToSlash(relPath),
			FileInfo: info,
		}
		return nil
	})
	close(filesChan)
	wg.Wait()

	if err != nil {
		return err
	}
	if copyErr != nil {
		return copyErr
	}
	return nil
}

func addFileToTar(tw *tar.Writer, base string, file FileToArchive) error {
	var link string
	if file.FileInfo.Mode()&os.ModeSymlink != 0 {
		var err error
		link, err = os.Readlink(file.Path)
		if err != nil {
			return fmt.Errorf("failed to read symlink %s: %w", file.Path, err)
		}
	}

	// Create header
	hdr, err := tar.FileInfoHeader(file.FileInfo, link)
	if err != nil {
		return fmt.Errorf("failed to create tar header for %s: %w", file.Path, err)
	}

	hdr.Name = file.RelPath
	hdr.Format = tar.FormatPAX // ensure long names work

	if err := tw.WriteHeader(hdr); err != nil {
		return fmt.Errorf("error writing tar header for %s: %w", file.Path, err)
	}

	// Don't copy content for non-regular files
	if !file.FileInfo.Mode().IsRegular() {
		return nil
	}

	f, err := os.Open(file.Path)
	if err != nil {
		return fmt.Errorf("failed to open file %s: %w", file.Path, err)
	}
	defer f.Close()

	if _, err := io.Copy(tw, f); err != nil {
		return fmt.Errorf("error copying file contents for %s: %w", file.Path, err)
	}

	return nil
}
