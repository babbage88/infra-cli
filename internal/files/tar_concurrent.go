package files

import (
	"archive/tar"
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"sync"
)

func CompressWithWorkers(src string, buf io.Writer) error {
	zr := gzip.NewWriter(buf)
	defer zr.Close()

	tw := tar.NewWriter(zr)
	defer tw.Close()

	type FileTask struct {
		Header *tar.Header
		Path   string
	}

	fileChan := make(chan FileTask)
	errChan := make(chan error, 1)
	done := make(chan struct{})

	const workerCount = 4

	// 1. Walk the directory and send files to fileChan
	go func() {
		defer close(fileChan)
		err := filepath.WalkDir(src, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			fi, err := d.Info()
			if err != nil {
				return err
			}
			relPath, err := filepath.Rel(src, path)
			if err != nil {
				return err
			}
			header, err := tar.FileInfoHeader(fi, path)
			if err != nil {
				return err
			}
			header.Name = filepath.ToSlash(relPath)

			fileChan <- FileTask{Header: header, Path: path}
			return nil
		})
		if err != nil {
			errChan <- err
		}
	}()

	// 2. Worker goroutines to read files and send content
	contentChan := make(chan struct {
		Header *tar.Header
		Data   []byte
	})

	var wg sync.WaitGroup
	for i := 0; i < workerCount; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for task := range fileChan {
				var data []byte
				if task.Header.Typeflag != tar.TypeDir {
					f, err := os.Open(task.Path)
					if err != nil {
						errChan <- err
						return
					}
					data, err = io.ReadAll(f)
					f.Close()
					if err != nil {
						errChan <- err
						return
					}
				}
				contentChan <- struct {
					Header *tar.Header
					Data   []byte
				}{Header: task.Header, Data: data}
			}
		}()
	}

	// 3. Close contentChan when all workers are done
	go func() {
		wg.Wait()
		close(contentChan)
	}()

	// 4. Write to tar file (single writer goroutine)
	go func() {
		for item := range contentChan {
			if err := tw.WriteHeader(item.Header); err != nil {
				errChan <- err
				return
			}
			if len(item.Data) > 0 {
				if _, err := tw.Write(item.Data); err != nil {
					errChan <- err
					return
				}
			}
		}
		close(done)
	}()

	// 5. Wait for completion or error
	select {
	case <-done:
		return nil
	case err := <-errChan:
		return err
	}
}
