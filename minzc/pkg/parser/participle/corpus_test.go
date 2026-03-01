package participle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// CorpusMetadata represents the metadata.json structure
type CorpusMetadata struct {
	Name       string `json:"name"`
	SourcePath string `json:"source_path"`
	HasErrors  bool   `json:"has_errors"`
	ErrorNodes int    `json:"error_nodes"`
	NodeCount  int    `json:"node_count"`
}

func TestParserCorpus(t *testing.T) {
	corpusDir := "../../../tests/parser_corpus"

	// Find all test directories — skip if corpus not generated
	entries, err := os.ReadDir(corpusDir)
	if err != nil {
		t.Skip("Parser corpus not available (generate with ast-gen tool): ", err)
	}

	pass, fail, skip := 0, 0, 0
	var failures []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		testDir := filepath.Join(corpusDir, entry.Name())
		inputPath := filepath.Join(testDir, "input.minz")
		metaPath := filepath.Join(testDir, "metadata.json")

		// Skip if no input file
		if _, err := os.Stat(inputPath); os.IsNotExist(err) {
			continue
		}

		// Read metadata to check if tree-sitter had errors
		var meta CorpusMetadata
		if metaData, err := os.ReadFile(metaPath); err == nil {
			json.Unmarshal(metaData, &meta)
		}

		// Read input file
		content, err := os.ReadFile(inputPath)
		if err != nil {
			skip++
			continue
		}

		// Parse with Participle
		parser := New()
		_, parseErr := parser.ParseString(inputPath, string(content))

		if parseErr != nil {
			if meta.HasErrors {
				// Tree-sitter also had errors, so this is expected
				skip++
			} else {
				fail++
				errStr := parseErr.Error()
				// Truncate error message
				if len(errStr) > 80 {
					errStr = errStr[:80] + "..."
				}
				failures = append(failures, entry.Name()+": "+errStr)
			}
		} else {
			if meta.HasErrors {
				// We succeeded where tree-sitter failed - that's good!
				pass++
			} else {
				pass++
			}
		}
	}

	t.Logf("Corpus Results: Pass=%d, Fail=%d, Skip=%d (tree-sitter had errors)", pass, fail, skip)
	t.Logf("Success rate: %.1f%% (%d/%d)", float64(pass)/float64(pass+fail)*100, pass, pass+fail)

	if len(failures) > 0 {
		t.Logf("\nFirst 20 failures:")
		for i, f := range failures {
			if i >= 20 {
				t.Logf("  ... and %d more", len(failures)-20)
				break
			}
			// Clean up the test name for readability
			name := strings.ReplaceAll(f, ".._", "../")
			t.Logf("  %s", name)
		}
	}
}
