package pipeline

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

func TestVariantSearchByteMultiplyVMZ80(t *testing.T) {
	// Keep the multi-function pressure/budget context, with a neutral entry name.
	// Earlier Grace toggles must not starve this entry's ordinary-loop trial.
	source := strings.ReplaceAll(countedConstantNanz, "c3", "product")
	h, err := nanz.Parse(source, "multiply_variants.nanz")
	if err != nil {
		t.Fatal(err)
	}
	var steps [2]Steps
	for i := range steps {
		steps[i], err = CompileHIRSteps(hir.CloneModule(h), Options{AssertMode: "none", DisableCountedLoops: i == 0})
		if err != nil {
			t.Fatal(err)
		}
	}
	selected := steps[1]
	found := false
	for _, d := range selected.VariantDecisions {
		if d.Function == "product" {
			found = true
			if d.Recipe.Counted != hir.CountedOrdinary || d.After >= d.Before || d.Candidates == 0 {
				t.Errorf("ordinary multiply-loop winner missing: %+v", d)
			}
		}
	}
	if !found {
		t.Error("missing product decision")
	}
	for _, block := range selected.MIR2Module.FuncByName("product").Blocks {
		if _, ok := block.Term.(*mir2.TermDJNZ); ok {
			t.Error("selected multiply loop still uses DJNZ")
		}
	}
	// Exhaust all byte multipliers: wraparound and helper timing depend on k.
	for k := 0; k < 256; k++ {
		want := byte(1)
		for j := 0; j < 255; j++ {
			want = want*byte(k) + 1
		}
		var cycles [2]int
		for i, s := range steps {
			got, e := mir2.NewVM(s.MIR2Module).Call("product", []mir2.Value{{I: int64(k)}})
			if e != nil || len(got) != 1 || got[0].I != int64(want) {
				t.Fatalf("mode %d k=%d MIR %v %v want %d", i, k, got, e, want)
			}
			cycles[i], _ = measureGraceZ80Compiled(t, h, s, "product", []int64{int64(k)}, nil, int64(want), 2000000)
		}
		if cycles[1] > cycles[0] {
			t.Errorf("k=%d ordinary %dT selected %dT", k, cycles[0], cycles[1])
		}
		if k == 3 {
			t.Logf("k=3: ordinary %dT selected %dT", cycles[0], cycles[1])
		}
	}
}

func TestVariantSearchByteMultiplyHelperCostVMZ80(t *testing.T) {
	h, err := nanz.Parse(`fun probe(a:u8,b:u8)->u8 { return a*b }`, "multiply_cost.nanz")
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h, Options{AssertMode: "none", DisableVariantSearch: true})
	if err != nil {
		t.Fatal(err)
	}
	im, err := assembleVariant(s)
	if err != nil {
		t.Fatal(err)
	}
	score, ok := im.score(s.MIR2Module.FuncByName("probe"))
	if !ok {
		t.Fatal("encoded byte multiply helper is unsupported")
	}
	for k := 0; k < 256; k++ {
		a := int64(byte(k*37 + 11))
		cycles, _ := measureGraceZ80Compiled(t, h, s, "probe", []int64{a, int64(k)}, nil, int64(byte(a*int64(k))), 10000)
		// Exactly 15T separates an add bit from a no-add bit in the helper.
		bits := 0
		for v := k; v != 0; v >>= 1 {
			bits += v & 1
		}
		if score != int64(cycles+15*(8-bits)) {
			t.Fatalf("k=%d score %dT native %dT", k, score, cycles)
		}
	}
	// A renamed helper scores identically: no symbol-name admission rule.
	renamed := s
	renamed.Assembly = strings.ReplaceAll(s.Assembly, "__mul8", "bit_product")
	ri, err := assembleVariant(renamed)
	if err != nil {
		t.Fatal(err)
	}
	if cost, ok := ri.score(s.MIR2Module.FuncByName("probe")); !ok || cost != score {
		t.Fatalf("renamed helper: %d %v", cost, ok)
	}
	// Even a one-byte mutation must lose the recognized helper summary.
	changed := s
	changed.Assembly = strings.Replace(s.Assembly, "    LD D, 8\n", "    LD D, 7\n", 1)
	ci, err := assembleVariant(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ci.score(s.MIR2Module.FuncByName("probe")); ok {
		t.Fatal("changed helper body was scored")
	}
	changed.Assembly = strings.ReplaceAll(s.Assembly, "CALL __mul8", "CALL probe")
	ci, err = assembleVariant(changed)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ci.score(s.MIR2Module.FuncByName("probe")); ok {
		t.Fatal("recursive/unknown call was scored")
	}
}
