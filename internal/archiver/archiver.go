package archiver

import "io"

// Archiver defines how to create and extract archives.
type Archiver interface {
	CreateTarGzWithExcludes(srcDir, outPath string, excludes []string) error
	ExtractTarGz(gzipStream io.Reader) error
}
