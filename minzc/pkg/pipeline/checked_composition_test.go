package pipeline

import (
	"github.com/minz/minzc/pkg/vir"
	"testing"
)

func TestCheckedCompositionOrdinaryNative(t *testing.T) {
	base, err := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), Options{OptSize: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := vir.PrepareLocalTableProblem(base.MIR2Module.FuncByName("add5"), base.Allocation)
	if err != nil {
		t.Fatal(err)
	}

	var pool, target vir.RegAllocTable
	pool.Init()
	target.Init()
	assignment := map[int]int{}
	for v, loc := range p.Options.ParamLocs {
		assignment[v] = loc
	}
	for _, op := range p.Ops {
		if op.Dst > 0 {
			assignment[op.Dst] = vir.Z80.LocByName("A")
		}
	}
	cut := 1
	for _, ops := range [][]vir.VIROp{p.Ops[:cut], p.Ops[cut:]} {
		local := map[int]int{}
		for _, op := range ops {
			if op.Dst > 0 {
				local[op.Dst] = assignment[op.Dst]
			}
			for _, v := range op.Src {
				if v > 0 {
					local[v] = assignment[v]
				}
			}
		}
		opts := p.Options
		opts.ParamLocs = map[int]int{}
		for v, loc := range p.Options.ParamLocs {
			if _, ok := local[v]; ok {
				opts.ParamLocs[v] = loc
			}
		}
		if err = pool.AddChecked(ops, local, vir.Z80, opts); err != nil {
			t.Fatal(err)
		}
	}
	count, err := pool.ComposeCheckedCandidates(p, &target, []int{cut}, 32)
	if err != nil || count == 0 {
		t.Fatal(count, err)
	}
	path := t.TempDir() + "/composed.json"
	if err = target.Save(path); err != nil {
		t.Fatal(err)
	}
	selected, err := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), Options{OptSize: true, AssertMode: "none", CheckedLocalTablePath: path})
	if err != nil {
		t.Fatal(err)
	}
	if selected.Traces["add5"].Backend != "checked-local-table" {
		t.Fatal("composed candidate not selected")
	}
	var beforeT, afterT int
	for x := 0; x < 256; x++ {
		beforeT = executeLocalTableFunction(t, base.Assembly, byte(x))
		afterT = executeLocalTableFunction(t, selected.Assembly, byte(x))
	}
	if afterT >= beforeT {
		t.Fatal("native timing did not improve", beforeT, afterT)
	}
	before, err := checkedAsmSize(base.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	after, err := checkedAsmSize(selected.Assembly)
	if err != nil || after >= before {
		t.Fatal(before, after, err)
	}
	t.Logf("composed two checked fragments: full module %d -> %d bytes; fixture T=%d -> %d; all 256 native inputs PASS", before, after, beforeT, afterT)
}
