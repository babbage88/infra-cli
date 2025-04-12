package files

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"log/slog"
	"os"
)

/*
	func TarAndGzipFiles(src string, buf io.Writer) error {
		zr := gzip.NewWriter(buf)
		defer zr.Close()
		tw := tar.NewWriter(zr)
		defer tw.Close()
		// WalkDir to traverse the directory tree
		err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				slog.Error(err.Error())
				return err
			}
			fi, err := d.Info()
			if err != nil {
				slog.Error("error retrieving FileInfo", slog.String("file", fi.Name()))
			}

			header, err := tar.FileInfoHeader(fi, path)
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(relPath)
			if err := tw.WriteHeader(header); err != nil {
				return err
			}
			if !fi.IsDir() {
				data, err := os.Open(path)
				if err != nil {
					return err
				}
				defer data.Close()

				_, err = io.Copy(tw, data)
				if err != nil {
					return err
				}
			}
			return nil

		})

		return err
	}
*/
func ExtractTarGz(gzipStream io.Reader) error {
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		slog.Error("ExtractTarGz: NewReader failed")
		return err
	}
	tarReader := tar.NewReader(uncompressedStream)
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			slog.Error("ExtractTarGz: Next() failed", "error", err.Error())
			return err
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(header.Name, 0755); err != nil {
				slog.Error("ExtractTarGz: Mkdir() failed", "error", err.Error())
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(header.Name)
			if err != nil {
				slog.Error("ExtractTarGz: Create() failed", "error", err.Error())
				return err
			}
			defer outFile.Close()
			if _, err := io.Copy(outFile, tarReader); err != nil {
				slog.Error("ExtractTarGz: Copy() failed", "error", err.Error())
				return err
			}
		default:
			slog.Error("ExtractTarGz: unknown type", slog.Any("headerTypeflag", header.Typeflag), slog.String("Name", header.Name))
			return err
		}
	}

	return err
}
