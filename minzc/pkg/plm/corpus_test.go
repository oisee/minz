package plm_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/plm"
)

// corpusDir is the path to cloned PL/M source files for corpus testing.
// Adjust if your corpus lives elsewhere.
const corpusDir = "/Users/alice/dev/minz-ts/corpus"

// TestCorpus_Parse runs the PL/M parser against every *.plm / *.PLM / *.pl1
// file found under corpusDir. It reports parse errors but does NOT fail the
// test — the goal is to surface which constructs are still unsupported, not
// to gate CI on a corpus we don't control.
func TestCorpus_Parse(t *testing.T) {
	if _, err := os.Stat(corpusDir); os.IsNotExist(err) {
		t.Skipf("corpus directory %q not found — run:\n  git clone https://github.com/ogdenpm/intel80tools %s/intel80tools", corpusDir, corpusDir)
	}

	var files []string
	_ = filepath.Walk(corpusDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		lower := strings.ToLower(filepath.Ext(path))
		if lower == ".plm" || lower == ".pl1" {
			files = append(files, path)
		}
		return nil
	})

	if len(files) == 0 {
		t.Skipf("no .plm/.pl1 files found under %s", corpusDir)
	}

	t.Logf("found %d PL/M source files", len(files))

	pass, fail := 0, 0
	var failSummary []string

	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			t.Logf("SKIP %s: read error: %v", filepath.Base(f), err)
			continue
		}

		rel, _ := filepath.Rel(corpusDir, f)
		_, parseErr := plm.ParseModuleFile(f, string(src))
		if parseErr == nil {
			pass++
			t.Logf("OK   %s", rel)
		} else {
			fail++
			msg := firstLine(parseErr.Error())
			failSummary = append(failSummary, rel+": "+msg)
			t.Logf("FAIL %s: %s", rel, msg)
		}
	}

	t.Logf("result: %d/%d parsed OK", pass, pass+fail)
	if fail > 0 {
		t.Logf("--- failing files summary ---")
		for _, s := range failSummary {
			t.Logf("  %s", s)
		}
		// Don't t.Fail() — corpus has constructs we haven't implemented yet.
	}
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
