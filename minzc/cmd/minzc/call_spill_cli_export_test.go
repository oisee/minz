package main

import (
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

// Persist the complete collect/replay fixture before TempDir cleanup. The hook
// runs on failures too; it does not turn a partial directory into valid evidence.
func retainCallSpillCLIFixture(t *testing.T, source, label string) {
	root := os.Getenv("MINZ_CALL_SPILL_CLI_ARTIFACTS")
	if root == "" {
		return
	}
	t.Cleanup(func() {
		from := filepath.Dir(source)
		to := filepath.Join(root, label)
		if err := filepath.WalkDir(from, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			rel, err := filepath.Rel(from, p)
			if err != nil {
				return err
			}
			dst := filepath.Join(to, rel)
			if d.IsDir() {
				return os.MkdirAll(dst, 0755)
			}
			b, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			return os.WriteFile(dst, b, 0644)
		}); err != nil {
			t.Errorf("preserve CLI fixture: %v", err)
		}
	})
}
