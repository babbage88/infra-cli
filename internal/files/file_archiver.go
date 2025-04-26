package files

import (
	"io"

	"github.com/babbage88/infra-cli/internal/archiver"
)

// Ensure files package satisfies Archiver interface
var _ archiver.Archiver = (*FilesArchiver)(nil)

type FilesArchiver struct{}

func (f *FilesArchiver) CreateTarGzWithExcludes(srcDir, outPath string, excludes []string) error {
	return CreateTarGzWithExcludes(srcDir, outPath, excludes)
}

func (f *FilesArchiver) ExtractTarGz(gzipStream io.Reader) error {
	return ExtractTarGz(gzipStream)
}
