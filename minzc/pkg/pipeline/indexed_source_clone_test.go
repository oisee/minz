package pipeline

import (
	"github.com/minz/minzc/pkg/vir"
	"os"
	"reflect"
	"testing"
)

func TestIndexedSourceCloneProposalReuse(t *testing.T) {
	asset := os.Getenv("INDEXED_ENRT4")
	if asset == "" {
		t.Skip("set INDEXED_ENRT4")
	}
	tab := &vir.RegAllocTable{}
	tab.Init()
	keys := map[string]bool{}
	var shape *vir.EnrichedShape
	record := -1
	clones := []struct {
		text, name string
		imm        int
	}{
		{"fun original(x:u8)->u8{return x+3}", "original", 3},
		{"fun renamed(x:u8)->u8{return x+17}", "renamed", 17},
		{"fun preceding(x:u8)->u8{return x+x+x}\nfun shifted(x:u8)->u8{return x+255}", "shifted", 255},
	}
	for _, c := range clones {
		steps, e := CompileHIRSteps(graceMeasureParse(t, c.text), Options{ContractOpt: true, OptSize: true, AssertMode: "none"})
		if e != nil {
			t.Fatal(e)
		}
		f := steps.MIR2Module.FuncByName(c.name)
		p, e := vir.PrepareLocalTableProblem(f, steps.Allocation)
		if e != nil {
			t.Fatal(e)
		}
		key, e := vir.CheckedTableKey(p.Ops, vir.Z80, p.Options)
		if e != nil || keys[key] {
			t.Fatal("fullsource binding not changed", e)
		}
		keys[key] = true
		if _, _, hit := tab.LookupChecked(p.Ops, vir.Z80, p.Options); hit {
			t.Fatal("old exactkey reused")
		}
		ev, e := vir.ImportIndexedENRT(asset, p, tab)
		if e != nil || ev.Import == nil || ev.Import.Admitted != 1 {
			t.Fatal(e, ev)
		}
		if shape == nil {
			shape = ev.Shape
			record = ev.RecordIndex
		} else if !reflect.DeepEqual(shape, ev.Shape) || record != ev.RecordIndex {
			t.Fatal("historical shape changed")
		}
		candidate, e := p.Emit(tab)
		if e != nil {
			t.Fatal(e)
		}
		lo, hi, ok := checkedFunctionSpan(steps.Assembly, c.name)
		if !ok {
			t.Fatal("span")
		}
		asm := steps.Assembly[:lo] + candidate + steps.Assembly[hi:]
		for x := 0; x < 256; x++ {
			runCheckedRealByte(t, asm, f, steps.Allocation, byte(x), byte(x+c.imm))
		}
	}
}
