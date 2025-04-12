package files

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func CompressWithWorkers(src string, buf io.Writer) error {
	zr := gzip.NewWriter(buf)
	defer zr.Close()

	tw := tar.NewWriter(zr)
	defer tw.Close()

	err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			slog.Error("walk error", slog.String("path", path), slog.String("err", err.Error()))
			return err
		}

		fi, err := d.Info()
		if err != nil {
			slog.Error("info error", slog.String("path", path), slog.String("err", err.Error()))
			return err
		}

		// Compute relative path for tar header
		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		relPath = filepath.ToSlash(relPath)

		// Skip root
		if relPath == "." {
			return nil
		}

		header, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}

		// Set full relative path explicitly
		header.Name = relPath

		// Use PAX format which handles long paths
		header.Format = tar.FormatPAX

		if err := tw.WriteHeader(header); err != nil {
			return err
		}

		// If it's a file, write contents
		if !fi.IsDir() {
			f, err := os.Open(path)
			if err != nil {
				return err
			}
			defer f.Close()

			if _, err := io.Copy(tw, f); err != nil {
				return err
			}
		}
		return nil
	})

	return err
}

func TarAndGzipFiles(src string, buf io.Writer) error {
	absSrc, err := filepath.Abs(src)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}
	zr := gzip.NewWriter(buf)
	defer zr.Close()
	tw := tar.NewWriter(zr)
	defer tw.Close()

	type fileEntry struct {
		header *tar.Header
		path   string
		isDir  bool
	}

	files := make(chan fileEntry)
	errs := make(chan error, 1)

	go func() {
		defer close(files)
		filepath.WalkDir(absSrc, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				errs <- err
				return err
			}

			fi, err := d.Info()
			if err != nil {
				errs <- fmt.Errorf("failed to get file info: %w", err)
				return err
			}

			relPath, err := filepath.Rel(absSrc, path)
			if err != nil {
				errs <- fmt.Errorf("failed to get relative path: %w", err)
				return err
			}

			// skip . (root dir)
			if relPath == "." {
				return nil
			}

			header, err := tar.FileInfoHeader(fi, "")
			if err != nil {
				errs <- fmt.Errorf("failed to create tar header: %w", err)
				return err
			}
			header.Name = filepath.ToSlash(relPath)

			files <- fileEntry{
				header: header,
				path:   path,
				isDir:  fi.IsDir(),
			}
			return nil
		})
		errs <- nil // signal walk success
	}()

	for entry := range files {
		if err := tw.WriteHeader(entry.header); err != nil {
			return fmt.Errorf("error writing tar header: %w", err)
		}

		if !entry.isDir {
			f, err := os.Open(entry.path)
			if err != nil {
				return fmt.Errorf("error opening file: %w", err)
			}
			_, err = io.Copy(tw, f)
			f.Close()
			if err != nil {
				return fmt.Errorf("error copying file contents: %w", err)
			}
		}
	}

	if err := <-errs; err != nil {
		return err
	}

	return nil
}
