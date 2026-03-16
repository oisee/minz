package c89

import (
	"embed"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
)

//go:embed libc/*.h
var libcFS embed.FS

// LibcFS returns the embedded minimal libc headers as an fs.FS
// rooted at the directory containing the .h files.
func LibcFS() fs.FS {
	sub, _ := fs.Sub(libcFS, "libc")
	return sub
}

var (
	libcDirOnce sync.Once
	libcDir     string
)

// libcSysIncludeDir extracts the embedded libc headers to a temp directory
// and returns the path. The directory is created once and reused.
func libcSysIncludeDir() string {
	libcDirOnce.Do(func() {
		dir, err := os.MkdirTemp("", "minz-libc-*")
		if err != nil {
			return
		}
		sub := LibcFS()
		entries, err := fs.ReadDir(sub, ".")
		if err != nil {
			return
		}
		for _, e := range entries {
			data, err := fs.ReadFile(sub, e.Name())
			if err != nil {
				continue
			}
			os.WriteFile(filepath.Join(dir, e.Name()), data, 0644)
		}
		libcDir = dir
	})
	return libcDir
}
