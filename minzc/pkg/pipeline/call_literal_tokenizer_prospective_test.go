package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"testing"
)

func TestCallLiteralTokenizerProspective(t *testing.T) {
	const report = "../../../reports/call-literal-rematerialization/frozen"
	const path = "../../../examples/nanz/self_tokenizer.nanz"
	dir := os.Getenv("MINZ_CALL_LITERAL_TOKENIZER_EXPORT")
	if dir == "" {
		// Always exercise frozen training/heldout in the normal suite.
		// An explicit directory retains exports beyond the test lifetime.
		dir = t.TempDir()
	}
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	var f struct {
		Source string            `json:"source_sha256"`
		Inputs map[string]string `json:"inputs_sha256"`
	}
	b, e := os.ReadFile(report + "/freeze.json")
	if e != nil {
		t.Fatal(e)
	}
	if e = json.Unmarshal(b, &f); e != nil {
		t.Fatal(e)
	}
	if fmt.Sprintf("%x", sha256.Sum256(src)) != f.Source {
		t.Fatal("source changed")
	}
	load := func(name string) []string {
		b, e := os.ReadFile(report + "/" + name + ".json")
		if e != nil {
			t.Fatal(e)
		}
		if fmt.Sprintf("%x", sha256.Sum256(b)) != f.Inputs[name+".json"] {
			t.Fatal("workload changed")
		}
		var v []string
		if e = json.Unmarshal(b, &v); e != nil {
			t.Fatal(e)
		}
		return v
	}
	train, held := load("training"), load("heldout")
	type candidate struct {
		Name     string
		T, Bytes int
		Assembly string      `json:"-"`
		Steps    Steps       `json:"-"`
		HIR      *hir.Module `json:"-"`
	}
	var rows []candidate
	for _, mode := range []string{"greedy", "pbqp"} {
		strategy := ""
		if mode == "pbqp" {
			strategy = mode
		}
		h, e := nanz.Parse(string(src), path)
		if e != nil {
			t.Fatal(e)
		}
		s, e := CompileHIRSteps(h, Options{ContractOpt: true, ContractStrategy: strategy, OptSize: true, AssertMode: "none"})
		if e != nil {
			t.Fatal(e)
		}
		out := filepath.Join(dir, mode, "training")
		if e = os.MkdirAll(out, 0755); e != nil {
			t.Fatal(e)
		}
		cost, size := runTokenizerABICandidate(t, h, s, train, out, mode)
		rows = append(rows, candidate{Name: mode, T: cost, Bytes: size, Assembly: s.Assembly, Steps: s, HIR: h})
		t.Logf("%s entiretrain %dT/%dB", mode, cost, size)
	}
	for _, goal := range []string{"speed", "size"} {
		best := 0
		for i := 1; i < len(rows); i++ {
			if goal == "speed" && rows[i].T < rows[best].T || goal == "size" && rows[i].Bytes < rows[best].Bytes {
				best = i
			}
		}
		r := rows[best]
		out := filepath.Join(dir, goal+"-heldout")
		if e = os.MkdirAll(out, 0755); e != nil {
			t.Fatal(e)
		}
		cost, size := runTokenizerABICandidate(t, r.HIR, r.Steps, held, out, r.Name)
		if r.Steps.Assembly != r.Assembly || size != r.Bytes {
			t.Fatal("selected executable changed")
		}
		t.Logf("%s selected%s fixedBIN heldout %dT/%dB", goal, r.Name, cost, size)
	}
	b, e = json.MarshalIndent(rows, "", "  ")
	if e != nil {
		t.Fatal(e)
	}
	if e = os.WriteFile(filepath.Join(dir, "candidates.json"), b, 0644); e != nil {
		t.Fatal(e)
	}
}
