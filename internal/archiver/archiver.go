package archiver

import "io"

// Archiver defines how to create and extract archives.
type Archiver interface {
	Compress(srcDir, outPath string, excludes []string) error
	Extract(gzipStream io.Reader) error
}
