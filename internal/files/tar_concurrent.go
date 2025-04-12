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

func CreateTarGzWithExcludes(srcDir, outPath string, exclude []string) error {
	outFile, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer outFile.Close()

	gz := gzip.NewWriter(outFile)
	defer gz.Close()

	tw := tar.NewWriter(gz)
	defer tw.Close()

	var (
		mu      sync.Mutex
		wg      sync.WaitGroup
		errs    = make(chan error, 1)
		bufPool = sync.Pool{
			New: func() any {
				return make([]byte, 32*1024)
			},
		}
	)

	// Normalize exclude patterns to clean relative paths
	cleanExcludes := make(map[string]struct{})
	for _, ex := range exclude {
		ex = filepath.Clean(ex)
		cleanExcludes[ex] = struct{}{}
	}

	isExcluded := func(rel string) bool {
		parts := strings.Split(rel, string(os.PathSeparator))
		for _, part := range parts {
			if _, ok := cleanExcludes[part]; ok {
				return true
			}
		}
		return false
	}

	err = filepath.Walk(srcDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}

		if path == srcDir {
			return nil
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return fmt.Errorf("getting relative path: %w", err)
		}

		// Skip excluded paths
		if isExcluded(rel) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		wg.Add(1)
		go func(path, rel string, info os.FileInfo) {
			defer wg.Done()

			var link string
			if info.Mode()&os.ModeSymlink != 0 {
				link, err = os.Readlink(path)
				if err != nil {
					errs <- fmt.Errorf("reading symlink: %w", err)
					return
				}
			}

			header, err := tar.FileInfoHeader(info, link)
			if err != nil {
				errs <- fmt.Errorf("creating tar header: %w", err)
				return
			}
			header.Name = rel

			mu.Lock()
			if err := tw.WriteHeader(header); err != nil {
				mu.Unlock()
				errs <- fmt.Errorf("writing tar header: %w", err)
				return
			}
			mu.Unlock()

			if !info.Mode().IsRegular() {
				return
			}

			f, err := os.Open(path)
			if err != nil {
				errs <- fmt.Errorf("opening file: %w", err)
				return
			}
			defer f.Close()

			buf := bufPool.Get().([]byte)
			defer bufPool.Put(buf)

			mu.Lock()
			_, err = io.CopyBuffer(tw, f, buf)
			mu.Unlock()
			if err != nil {
				errs <- fmt.Errorf("copying data: %w", err)
				return
			}

			fmt.Println("Added:", rel)
		}(path, rel, info)

		return nil
	})

	if err != nil {
		return fmt.Errorf("walking path: %w", err)
	}

	wg.Wait()
	close(errs)

	if e := <-errs; e != nil {
		return e
	}

	return nil
}
