package abap

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestSALVCapacityBudget(t *testing.T) {
	sizes := map[int]int{}
	counts := map[int]int{}
	for _, capacity := range []int{4, 32} {
		src := strings.Replace(salvFleetDecl, "INITIAL SIZE 32", fmt.Sprintf("INITIAL SIZE %d", capacity), 1) + salvBind + salvDisplay
		hm, e := Compile(src, "zsalv")
		if e != nil {
			t.Fatal(e)
		}
		opts := pipeline.DefaultOptions()
		opts.AssertMode = "none"
		steps, e := pipeline.CompileHIRSteps(hm, opts)
		if e != nil {
			t.Fatal(e)
		}
		assembly, e := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{CPMApplicationEntry: true, UnsignedPairComparePolicy: steps.UnsignedPairComparePolicy})
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString(assembly)
		if e != nil || len(a.Errors) > 0 {
			t.Fatalf("assembly %v %v", e, a.Errors)
		}
		sizes[capacity] = len(a.Binary)
		counts[capacity] = len(steps.MIR2Module.Funcs)
		if 0x100+len(a.Binary) >= 0xfe00 {
			t.Fatal("does not fit measured MZE TPA")
		}
		t.Logf("capacity%d COM%d imageEnd%04x stack%04x..%04x functions%d", capacity, len(a.Binary), 0x100+len(a.Binary), a.Symbols["__cpm_app_stack"], a.Symbols["__cpm_app_stack_top"], counts[capacity])
	}
	// Only the 44-byte row array grows. No capacity-sized renderer expansion.
	if got := sizes[32] - sizes[4]; got != 28*44 {
		t.Fatalf("capacity image growth %d want row storage %d", got, 28*44)
	}
	if counts[4] != counts[32] {
		t.Fatal("capacity changed generated function count")
	}
}
