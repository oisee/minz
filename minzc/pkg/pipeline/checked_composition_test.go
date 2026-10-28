package pipeline

import (
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/vir"
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

func TestCheckedCompositionMoveBridgePipelineNative(t *testing.T) {
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
	for _, ops := range [][]vir.VIROp{p.Ops[:cut]} {
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
	count, err := pool.ComposeCheckedCandidatesWithMoves(p, &target, []int{cut}, 32)
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
	t.Logf("composed table proposal plus explicit move/RET bridge: full module %d -> %d bytes; fixture T=%d -> %d; all 256 native inputs PASS", before, after, beforeT, afterT)
}

func TestCheckedCompositionWeightedNativeCaller(t *testing.T) {
	checkedCompositionWeightedNativeCaller(t, false)
}

func TestCheckedCompositionMoveBridgeWeightedCaller(t *testing.T) {
	checkedCompositionWeightedNativeCaller(t, true)
}

func TestCheckedCompositionAutomaticWeightedCaller(t *testing.T) {
	checkedCompositionWeightedNativeCallerMode(t, true, true)
}

func checkedCompositionWeightedNativeCaller(t *testing.T, moves bool) {
	checkedCompositionWeightedNativeCallerMode(t, moves, false)
}

func checkedCompositionWeightedNativeCallerMode(t *testing.T, moves, automatic bool) {
	t.Helper()
	source, err := os.ReadFile("../../../examples/nanz/06_pbqp_weighted.nanz")
	if err != nil {
		t.Fatal(err)
	}
	// The showcase source returns heavy + light, whose constant RHS is
	// rematerialised (F6) and no longer reserves a register, which makes the
	// ordinary allocation optimal for this function.  Swapping the final
	// operand keeps a register-needing constant use, so the composed table
	// still has something to beat.
	text := strings.Replace(string(source), "return heavy + light", "return light + heavy", 1) + "\nfun weighted_caller(x:u8)->u8 { return weighted(x)+x }\n"
	for _, strategy := range []string{"greedy", "pbqp"} {
		for _, grace := range []bool{false, true} {
			opts := Options{OptSize: true, AssertMode: "none", ContractOpt: true, ContractStrategy: strategy, UseGrace: grace, UseHIRGrace: grace}
			base, err := CompileHIRSteps(graceMeasureParse(t, text), opts)
			if err != nil {
				t.Fatal(err)
			}
			p, err := vir.PrepareLocalTableProblem(base.MIR2Module.FuncByName("weighted"), base.Allocation)
			if err != nil {
				t.Fatal(err)
			}
			assignment := map[int]int{}
			for v, loc := range p.Options.ParamLocs {
				assignment[v] = loc
			}
			for _, op := range p.Ops {
				if op.Dst > 0 {
					assignment[op.Dst] = vir.Z80.LocByName("A")
					if op.Op == vir.OpMove && op.DstHint.IsEmpty() {
						assignment[op.Dst] = vir.Z80.LocByName("D")
					}
				}
			}
			var pool vir.RegAllocTable
			target := &vir.RegAllocTable{}
			pool.Init()
			target.Init()
			cut := len(p.Ops) / 2
			parts := [][]vir.VIROp{p.Ops[:cut], p.Ops[cut:]}
			if moves {
				cut = len(p.Ops) - 2
				parts = [][]vir.VIROp{p.Ops[:cut]}
			}
			for _, part := range parts {
				local := map[int]int{}
				for _, op := range part {
					if op.Dst > 0 {
						local[op.Dst] = assignment[op.Dst]
					}
					for _, v := range op.Src {
						if v > 0 {
							local[v] = assignment[v]
						}
					}
				}
				fragmentOpts := p.Options
				fragmentOpts.ParamLocs = map[int]int{}
				for v, loc := range p.Options.ParamLocs {
					if _, exists := local[v]; exists {
						fragmentOpts.ParamLocs[v] = loc
					}
				}
				if err = pool.AddChecked(part, local, vir.Z80, fragmentOpts); err != nil {
					t.Fatal(strategy, grace, err, p.Ops)
				}
			}
			compose := pool.ComposeCheckedCandidates
			if moves {
				compose = pool.ComposeCheckedCandidatesWithMoves
			}
			n := 0
			if automatic {
				var notes []string
				prepared, ns, e := ComposeCheckedLocalTables(graceMeasureParse(t, text), opts, &pool, 64)
				err = e
				notes = ns
				if prepared != nil {
					target = prepared
					n = target.Size()
				}
				t.Log(notes)
			} else {
				n, err = compose(p, target, []int{cut}, 64)
			}
			if err != nil || n == 0 {
				t.Fatal(n, err)
			}
			path := t.TempDir() + "/composed.json"
			if err = target.Save(path); err != nil {
				t.Fatal(err)
			}
			opts.CheckedLocalTablePath = path
			selected, err := CompileHIRSteps(graceMeasureParse(t, text), opts)
			if err != nil {
				t.Fatal(err)
			}
			if selected.Traces["weighted"].Backend != "checked-local-table" {
				t.Fatal("composed weighted not selected")
			}
			var beforeT, afterT int
			for x := 0; x < 256; x++ {
				want := byte(11*x + 1)
				beforeT = runCheckedRealByte(t, base.Assembly, base.MIR2Module.FuncByName("weighted_caller"), base.Allocation, byte(x), want)
				afterT = runCheckedRealByte(t, selected.Assembly, selected.MIR2Module.FuncByName("weighted_caller"), selected.Allocation, byte(x), want)
			}
			beforeB, err := checkedAsmSize(base.Assembly)
			if err != nil {
				t.Fatal(err)
			}
			afterB, err := checkedAsmSize(selected.Assembly)
			if err != nil || afterB >= beforeB || afterT >= beforeT {
				t.Fatal(beforeB, afterB, beforeT, afterT, err)
			}
			t.Logf("PFCCO=%s grace=%v composed weighted: module %d -> %d bytes, caller %d -> %d T-states; all 256 inputs PASS", strategy, grace, beforeB, afterB, beforeT, afterT)
		}
	}
}

func TestCheckedCompositionHeldoutOperationsNative(t *testing.T) {
	checkedCompositionHeldoutOperationsNative(t, false)
}

func TestCheckedCompositionAutomaticHeldoutNative(t *testing.T) {
	checkedCompositionHeldoutOperationsNative(t, true)
}

func checkedCompositionHeldoutOperationsNative(t *testing.T, automatic bool) {
	t.Helper()
	// Prepare the assignment pool only on add5; reuse it without regenerating
	// records on the different constants/operators and live caller workload.
	seed, err := CompileHIRSteps(graceMeasureParse(t, checkedTableSource), Options{OptSize: true, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	p, err := vir.PrepareLocalTableProblem(seed.MIR2Module.FuncByName("add5"), seed.Allocation)
	if err != nil {
		t.Fatal(err)
	}
	var pool vir.RegAllocTable
	pool.Init()
	for _, part := range [][]vir.VIROp{p.Ops[:1], p.Ops[1:]} {
		assignment := map[int]int{}
		for _, op := range part {
			if op.Dst > 0 {
				assignment[op.Dst] = vir.Z80.LocByName("A")
			}
			for _, v := range op.Src {
				if v > 0 {
					assignment[v] = vir.Z80.LocByName("A")
				}
			}
		}
		opts := p.Options
		opts.ParamLocs = map[int]int{}
		for v, loc := range p.Options.ParamLocs {
			if _, exists := assignment[v]; exists {
				opts.ParamLocs[v] = loc
			}
		}
		if err = pool.AddChecked(part, assignment, vir.Z80, opts); err != nil {
			t.Fatal(err)
		}
	}
	for _, test := range []struct {
		expression string
		oracle     func(byte) byte
	}{
		{"x+19", func(x byte) byte { return x + 19 }},
		{"x^90", func(x byte) byte { return x ^ 90 }},
		{"x-11", func(x byte) byte { return x - 11 }},
	} {
		source := "fun tweak(x:u8)->u8 { return " + test.expression + " }\nfun caller(x:u8)->u8 { return tweak(x)+x }"
		opts := Options{OptSize: true, AssertMode: "none"}
		baseline, err := CompileHIRSteps(graceMeasureParse(t, source), opts)
		if err != nil {
			t.Fatal(err)
		}
		current, err := vir.PrepareLocalTableProblem(baseline.MIR2Module.FuncByName("tweak"), baseline.Allocation)
		if err != nil {
			t.Fatal(err)
		}
		var composed vir.RegAllocTable
		composed.Init()
		n := 0
		if automatic {
			n, err = pool.ComposeCheckedAutomatically(current, &composed, 32)
		} else {
			n, err = pool.ComposeCheckedCandidates(current, &composed, []int{1}, 32)
		}
		if err != nil || n == 0 {
			t.Fatal(test.expression, n, err)
		}
		path := t.TempDir() + "/composed.json"
		if err = composed.Save(path); err != nil {
			t.Fatal(err)
		}
		opts.CheckedLocalTablePath = path
		selected, err := CompileHIRSteps(graceMeasureParse(t, source), opts)
		if err != nil {
			t.Fatal(err)
		}
		var before, after int
		for x := 0; x < 256; x++ {
			want := test.oracle(byte(x)) + byte(x)
			before = runCheckedRealByte(t, baseline.Assembly, baseline.MIR2Module.FuncByName("caller"), baseline.Allocation, byte(x), want)
			after = runCheckedRealByte(t, selected.Assembly, selected.MIR2Module.FuncByName("caller"), selected.Allocation, byte(x), want)
		}
		if after > before {
			t.Fatal("heldout caller regressed", test.expression, before, after)
		}
		t.Logf("%s reused add5 fragments: actual caller %d -> %d T; backend=%s; all 256 inputs PASS", test.expression, before, after, selected.Traces["tweak"].Backend)
	}
}
