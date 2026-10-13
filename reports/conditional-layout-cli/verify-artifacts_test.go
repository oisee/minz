package pipeline

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Manual audit: copy into pkg/pipeline, set MINZ_LAYOUT_CLI_ARTIFACTS,
// and run this test against persisted standalone CLI results.
func TestLayoutCLIArtifactsIndependentTokenizer(t *testing.T) {
	root := os.Getenv("MINZ_LAYOUT_CLI_ARTIFACTS")
	if root == "" {
		t.Fatal("MINZ_LAYOUT_CLI_ARTIFACTS required")
	}
	readJSON := func(path string, value any) {
		t.Helper()
		raw, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		if e = json.Unmarshal(raw, value); e != nil {
			t.Fatal(e)
		}
	}
	for _, goal := range []string{"speed", "size"} {
		t.Run(goal, func(t *testing.T) {
			training := filepath.Join(root, goal+"-training")
			heldout := filepath.Join(root, goal+"-heldout")
			var receipt struct {
				Evidence *ConditionalLayoutModesEvidence `json:"evidence"`
			}
			readJSON(filepath.Join(training, "selection.json"), &receipt)
			var train, held hir.ProfileWorkload
			readJSON(filepath.Join(training, "workload.json"), &train)
			readJSON(filepath.Join(heldout, "workload.json"), &held)
			candidates := 0
			for _, mode := range receipt.Evidence.Runs {
				for _, candidate := range mode.Candidates {
					conditionalTokenizerIndependentCheck(t, train, candidate.Run)
					candidates++
				}
			}
			var selected Z80ProfileWorkloadEvidence
			readJSON(filepath.Join(heldout, "run.json"), &selected)
			conditionalTokenizerIndependentCheck(t, held, &selected)
			path := "../../../examples/nanz/grace_tokenizer_checked.nanz"
			source, e := os.ReadFile(path)
			if e != nil {
				t.Fatal(e)
			}
			factory := func() (*hir.Module, error) {
				m, e := nanz.Parse(string(source), path)
				if e == nil {
					m.Target = hir.TargetCPM
				}
				return m, e
			}
			input, e := factory()
			if e != nil {
				t.Fatal(e)
			}
			baseline, e := CompileHIRSteps(input, receipt.Evidence.Options[0])
			if e != nil {
				t.Fatal(e)
			}
			if baseline.Assembly != receipt.Evidence.Runs[0].Candidates[0].Assembly {
				t.Fatal("ordinary baseline executable drift")
			}
			module, e := factory()
			if e != nil {
				t.Fatal(e)
			}
			oracle, e := hir.RunLoopProfile(module, held)
			if e != nil {
				t.Fatal(e)
			}
			before, e := RunZ80ProfileWorkload(baseline, module, held, *oracle)
			if e != nil {
				t.Fatal(e)
			}
			conditionalTokenizerIndependentCheck(t, held, before)
			binary, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + baseline.Assembly)
			if e != nil {
				t.Fatal(e)
			}
			if len(binary.Errors) > 0 {
				t.Fatal(binary.Errors)
			}
			for name, value := range map[string]any{"ordinary-heldout.json": before, "ordinary-options.json": receipt.Evidence.Options[0]} {
				raw, e := json.MarshalIndent(value, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(heldout, name), raw, 0644); e != nil {
					t.Fatal(e)
				}
			}
			if e = os.WriteFile(filepath.Join(heldout, "ordinary.a80"), []byte(baseline.Assembly), 0644); e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(heldout, "ordinary.bin"), binary.Binary, 0644); e != nil {
				t.Fatal(e)
			}
			best := receipt.Evidence.Runs[receipt.Evidence.SelectedMode]
			t.Logf("%d CLI candidates: independent tokenizer tokens/resources/globals/accesses PASS; ordinary/selected %d/%d B, training %d/%d T, heldout %d/%d T", candidates, len(binary.Binary), len(best.Candidates[best.Selected].Binary), receipt.Evidence.Runs[0].Candidates[0].Run.TotalTStates, best.Candidates[best.Selected].Run.TotalTStates, before.TotalTStates, selected.TotalTStates)
		})
	}
}
