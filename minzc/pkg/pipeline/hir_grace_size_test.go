package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestHIRGraceSizePolicy(t *testing.T) {
	sizes := map[bool]int{}
	for _, size := range []bool{false, true} {
		h := graceMeasureParse(t, graceSumSource)
		result, err := hir.RunGraceWithOptions(h, hir.GraceOptions{OptimizeForSize: size})
		if err != nil {
			t.Fatal(err)
		}
		want := 1
		if size {
			want = 0
		}
		if result.ByRule["hir-guarded-pointer-walk"] != want {
			t.Fatalf("size=%v rules=%v", size, result.ByRule)
		}
		for _, direct := range []bool{false, true} {
			h = graceMeasureParse(t, graceSumSource)
			opt := Options{UseHIRGrace: true, OptSize: size, AssertMode: "none"}
			var asm string
			if direct {
				asm, err = CompileHIRWithOptions(h, opt)
			} else {
				var s Steps
				s, err = CompileHIRSteps(h, opt)
				asm = s.Assembly
				if strings.Contains(s.HIR, "__grace_element_") == size {
					t.Fatalf("size=%v: wrong HIR choice", size)
				}
				vm := mir2.NewVM(s.MIR2Module)
				vm.EnsureHeap(16)
				wantSum := int64(0)
				for i := int64(0); i < 16; i++ {
					v := byte(i*7 + 3)
					vm.WriteHeap(i, v)
					wantSum = (wantSum + int64(v)) & 255
				}
				got, callErr := vm.Call("sum_array", []mir2.Value{{I: 0}, {I: 16}})
				if callErr != nil || len(got) != 1 || got[0].I != wantSum {
					t.Fatalf("size=%v: got %v %v want %d", size, got, callErr, wantSum)
				}
			}
			if err != nil {
				t.Fatal(err)
			}
			r, asmErr := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
			if asmErr != nil || len(r.Errors) > 0 {
				t.Fatalf("assemble %v %v", asmErr, r.Errors)
			}
			if !direct {
				sizes[size] = len(r.Binary)
			}
		}
	}
	if sizes[true] >= sizes[false] {
		t.Fatalf("size mode failed to avoid measured growth: %v", sizes)
	}
	h := graceMeasureParse(t, graceRegionSource)
	result, err := hir.RunGraceWithOptions(h, hir.GraceOptions{OptimizeForSize: true})
	if err != nil || result.ByRule["hir-invariant-base"] != 1 {
		t.Fatalf("size mode lost invariant-base: %v %v", result, err)
	}
	t.Logf("sum_array code bytes speed/default=%d size=%d", sizes[false], sizes[true])
}
