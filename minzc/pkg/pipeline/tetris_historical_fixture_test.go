package pipeline

import (
	"os"
	"path/filepath"
	"testing"
)

// Historical bitmap-shadow proofs must keep their sprite generator as well as
// the game source fixed. New public methods otherwise change the size/layout of
// that old binary, which reserves E000..F7FF for a bitmap shadow.
func historicalTetrisStdlib(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.CopyFS(dir, os.DirFS("../../../stdlib")); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"sprite.nanz", "sprite_address.nanz"} {
		data, err := os.ReadFile(filepath.Join("../../../reports/advanced-demos/tetris/shadow-stdlib/zx", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "zx", name), data, 0644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}
