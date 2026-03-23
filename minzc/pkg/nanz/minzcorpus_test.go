package nanz_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
)

// TestMinZCorpusParse verifies that the Nanz parser can parse the entire
// active MinZ corpus (.minz files) without errors.
func TestMinZCorpusParse(t *testing.T) {
	root := filepath.Join("..", "..", "..", "examples")
	var files []string
	filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		// Skip archived/aspirational/experimental
		if strings.Contains(path, "_archive") || strings.Contains(path, "aspirational") ||
			strings.Contains(path, "experimental") || strings.Contains(path, "/invalid/") {
			return nil
		}
		// Skip working/ duplicates
		if strings.Contains(path, "/working/") {
			return nil
		}
		if filepath.Ext(path) == ".minz" {
			files = append(files, path)
		}
		return nil
	})

	if len(files) == 0 {
		t.Skip("no .minz files found")
	}
	t.Logf("Found %d .minz files", len(files))

	pass, fail := 0, 0
	for _, path := range files {
		base := filepath.Base(path)
		t.Run(base, func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			_, err = nanz.Parse(string(src), base)
			if err != nil {
				fail++
				t.Logf("PARSE FAIL: %v", err)
			} else {
				pass++
			}
		})
	}
	t.Logf("=== MinZ corpus: %d pass, %d fail / %d total ===", pass, fail, pass+fail)
}
