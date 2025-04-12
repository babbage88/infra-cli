package files

import (
	"archive/tar"
	"compress/gzip"
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
