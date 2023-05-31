package zon

import (
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/hexops/wrench/internal/errors"
)

func ComputePackageHash(dir string) (string, error) {
	var (
		mu    sync.Mutex
		files []hashedFile
		wg    sync.WaitGroup
	)
	filepath.Walk(dir, func(path string, fi fs.FileInfo, err error) error {
		if fi.IsDir() {
			return nil
		}
		wg.Add(1)
		go func() {
			defer wg.Done()

			normalizedPath := strings.TrimPrefix(path, dir)
			normalizedPath = strings.ReplaceAll(normalizedPath, string(os.PathSeparator), "/")
			normalizedPath = strings.TrimPrefix(normalizedPath, "/")
			hash, err := hashFile(path, normalizedPath, fi)

			mu.Lock()
			defer mu.Unlock()
			files = append(files, hashedFile{
				path:           path,
				normalizedPath: normalizedPath,
				hash:           hash,
				err:            err,
			})
		}()

		return nil
	})
	wg.Wait()

	sort.Slice(files, func(i, j int) bool {
		return files[i].Less(&files[j])
	})

	h := sha256.New()
	for _, f := range files {
		if f.err != nil {
			return "", f.err
		}
		_, err := h.Write(f.hash)
		if err != nil {
			return "", err
		}
	}
	digest := fmt.Sprintf("%x", h.Sum(nil))
	return hexDigest(digest), nil
}

func hexDigest(digest string) string {
	// sha256
	multihashFunction := 0x12 // https://github.com/ziglang/zig/blob/master/src/Manifest.zig#L17-L33
	digestLength := len(digest) / 2

	return fmt.Sprintf("%x%x%s", multihashFunction, digestLength, digest)
}

func hashFile(path, normalizedPath string, fi fs.FileInfo) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, errors.Wrap(err, "Open")
	}
	defer f.Close()

	h := sha256.New()
	h.Write([]byte(normalizedPath))
	h.Write([]byte{0, isExecutable(fi.Mode())})
	if _, err := io.Copy(h, f); err != nil {
		return nil, errors.Wrap(err, "sha256 hash")
	}
	return h.Sum(nil), nil
}

type hashedFile struct {
	path           string
	normalizedPath string
	hash           []byte
	err            error
}

func (h *hashedFile) Less(other *hashedFile) bool {
	return h.normalizedPath < other.normalizedPath
}

func IsExecAny(mode os.FileMode) bool {
	return mode&0o111 != 0
}

func isExecutable(mode fs.FileMode) byte {
	if mode&0x40 != 0 {
		return 1
	}
	return 0
}
