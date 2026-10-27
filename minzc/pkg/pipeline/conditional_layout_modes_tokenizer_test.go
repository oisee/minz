package pipeline

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestConditionalLayoutModesTokenizerHeldout(t *testing.T) {
	// Measured layout enumeration compiles the exact requested configuration
	// (A1 forces DisableVariantSearch inside MeasureConditionalLayoutModes and
	// MeasureConditionalLayoutWorkload). The ordinary-baseline recompile below
	// must request that same configuration or it drifts from the evidence.
	modes := []Options{}
	for _, strategy := range []string{"greedy", "pbqp"} {
		for _, mirGrace := range []bool{false, true} {
			for _, hirGrace := range []bool{false, true} {
				modes = append(modes, Options{OptSize: true, ContractOpt: true, ContractStrategy: strategy, AssertMode: "none", UseGrace: mirGrace, UseHIRGrace: hirGrace, DisableVariantSearch: true})
			}
		}
	}
	for _, goal := range []string{"speed", "size"} {
		t.Run(goal, func(t *testing.T) {
			directory := ""
			if root := os.Getenv("MINZ_CONDITIONAL_MODES_EXPORT"); root != "" {
				directory = filepath.Join(root, goal)
			}
			var savedFactory func() (*hir.Module, error)
			var globalEvidence *ConditionalLayoutModesEvidence
			if directory == "" {
				directory = t.TempDir()
			}
			selector := func(factory func() (*hir.Module, error), w hir.ProfileWorkload, _ Options) (*ConditionalLayoutResult, error) {
				result, e := MeasureConditionalLayoutModes(factory, "is_space", w, modes, goal, 4)
				if e != nil {
					return nil, e
				}
				evidence := result.Evidence
				savedFactory = factory
				globalEvidence = evidence
				chosen := evidence.Runs[evidence.SelectedMode]
				best := chosen.Candidates[chosen.Selected]
				for _, run := range evidence.Runs {
					for _, candidate := range run.Candidates {
						if goal == "speed" && candidate.Run.TotalTStates < best.Run.TotalTStates || goal == "size" && len(candidate.Binary) < len(result.Binary) {
							return nil, fmt.Errorf("not global minimum")
						}
					}
				}
				t.Logf("global mode%d, layout%d: %d B / %d training T; ordinary %d B / %d T", evidence.SelectedMode, chosen.Selected, len(result.Binary), best.Run.TotalTStates, len(evidence.Runs[0].Candidates[0].Binary), evidence.Runs[0].Candidates[0].Run.TotalTStates)
				if directory != "" {
					if e = os.MkdirAll(directory, 0755); e != nil {
						return nil, e
					}
					payload, e := json.MarshalIndent(evidence, "", "  ")
					if e != nil {
						return nil, e
					}
					f, e := os.Create(filepath.Join(directory, "global-evidence.json.gz"))
					if e != nil {
						return nil, e
					}
					z := gzip.NewWriter(f)
					_, e = z.Write(payload)
					closeZip := z.Close()
					closeFile := f.Close()
					if e != nil {
						return nil, e
					}
					if closeZip != nil {
						return nil, closeZip
					}
					if closeFile != nil {
						return nil, closeFile
					}
				}
				return &ConditionalLayoutResult{Steps: result.Steps, Binary: result.Binary, Evidence: chosen}, nil
			}
			conditionalLayoutTokenizerWithSelector(t, modes[0], directory, selector)
			for name, options := range map[string]Options{"options.json": globalEvidence.Options[globalEvidence.SelectedMode], "ordinary-options.json": modes[0]} {
				data, e := json.MarshalIndent(options, "", "  ")
				if e != nil {
					t.Fatal(e)
				}
				if e = os.WriteFile(filepath.Join(directory, name), data, 0644); e != nil {
					t.Fatal(e)
				}
			}
			workloadBytes, e := os.ReadFile(filepath.Join(directory, "heldout.json"))
			if e != nil {
				t.Fatal(e)
			}
			var heldout hir.ProfileWorkload
			if e = json.Unmarshal(workloadBytes, &heldout); e != nil {
				t.Fatal(e)
			}
			source, e := savedFactory()
			if e != nil {
				t.Fatal(e)
			}
			oracle, e := hir.RunLoopProfile(source, heldout)
			if e != nil {
				t.Fatal(e)
			}
			input, e := savedFactory()
			if e != nil {
				t.Fatal(e)
			}
			baseline, e := CompileHIRSteps(input, modes[0])
			if e != nil {
				t.Fatal(e)
			}
			if baseline.Assembly != globalEvidence.Runs[0].Candidates[0].Assembly {
				t.Fatal("ordinary baseline executable drift")
			}
			run, e := RunZ80ProfileWorkload(baseline, source, heldout, *oracle)
			if e != nil {
				t.Fatal(e)
			}
			conditionalTokenizerIndependentCheck(t, heldout, run)
			selectedBytes, e := os.ReadFile(filepath.Join(directory, "heldout-after.json"))
			if e != nil {
				t.Fatal(e)
			}
			var selected Z80ProfileWorkloadEvidence
			if e = json.Unmarshal(selectedBytes, &selected); e != nil {
				t.Fatal(e)
			}
			t.Logf("global heldout ordinary %d / selected %d T", run.TotalTStates, selected.TotalTStates)
			data, e := json.MarshalIndent(run, "", "  ")
			if e != nil {
				t.Fatal(e)
			}
			if e = os.WriteFile(filepath.Join(directory, "ordinary-heldout.json"), data, 0644); e != nil {
				t.Fatal(e)
			}
			binary, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + baseline.Assembly)
			if e != nil {
				t.Fatal(e)
			}
			if len(binary.Errors) > 0 {
				t.Fatal(binary.Errors)
			}
			for name, b := range map[string][]byte{"ordinary.a80": []byte(baseline.Assembly), "ordinary.bin": binary.Binary} {
				if e = os.WriteFile(filepath.Join(directory, name), b, 0644); e != nil {
					t.Fatal(e)
				}
			}

		})
	}
}
