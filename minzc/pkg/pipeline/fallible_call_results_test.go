package pipeline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

// fallibleCallProfiles are the configurations every repro below runs in: the
// speed and size profiles, each with the ordinary variant search and with a
// forced per-function recipe for run. The recipes use the accumulator-pressure
// allocation that --Osize selects for these sources on unfixed main; forcing
// one makes the bug reproducible in the speed profile too, whose search never
// picks it for these sources.
func fallibleCallProfiles(recipe FunctionVariant) map[string]Options {
	forced := map[string]FunctionVariant{"run": recipe}
	return map[string]Options{
		"speed":        {ContractOpt: true, UseGrace: true, AssertMode: "z80"},
		"size":         {ContractOpt: true, UseGrace: true, AssertMode: "z80", OptSize: true},
		"speed-forced": {ContractOpt: true, UseGrace: true, AssertMode: "z80", DisableVariantSearch: true, FunctionVariants: forced},
		"size-forced":  {ContractOpt: true, UseGrace: true, AssertMode: "z80", OptSize: true, DisableVariantSearch: true, FunctionVariants: forced},
	}
}

// runFallibleCallRepro compiles a testdata source with every assert forced to
// the Z80 emulator and requires each of them to execute and pass.
func runFallibleCallRepro(t *testing.T, file string, o Options) string {
	t.Helper()
	src, err := os.ReadFile(filepath.Join("testdata", "fallible_calls", file))
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.Parse(string(src), file)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Asserts) == 0 {
		t.Fatal("repro has no asserts")
	}
	for i := range h.Asserts {
		h.Asserts[i].Via = "z80"
	}
	stats := &hir.AssertStats{}
	h.AssertStats = stats
	s, err := CompileHIRSteps(h, o)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Executed != len(h.Asserts) || stats.Failed != 0 {
		t.Fatalf("asserts %+v, want %d executed and none failed", *stats, len(h.Asserts))
	}
	return s.Assembly
}

// Bug B: after a fallible call the result pickup reads every result from the
// AF snapshot; the failed bit bound for A may be written into A's snapshot
// byte only after the value and error code have been read from it.
func TestFallibleCallResultSnapshotNative(t *testing.T) {
	for _, file := range []string{"nw2.nanz", "nw19.nanz"} {
		// The size allocation table makes the speed profile reach this bug too.
		for name, o := range fallibleCallProfiles(FunctionVariant{MIRGrace: true, SizeAllocation: true, AccPressure: true}) {
			t.Run(file+"/"+name, func(t *testing.T) {
				runFallibleCallRepro(t, file, o)
			})
		}
	}
}
