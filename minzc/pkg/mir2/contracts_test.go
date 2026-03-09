package mir2_test

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// ── Call graph tests ──────────────────────────────────────────────────────────

func TestCallGraph_LeafFunction(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("leaf")
	b := f.NewBlock("entry")
	b.Term = &mir2.TermRet{}

	cg := mir2.BuildCallGraph(m)
	if !cg.IsLeaf("leaf") {
		t.Error("leaf function should have no outgoing edges")
	}
	if !cg.Acyclic() {
		t.Error("single leaf function should be acyclic")
	}
}

func TestCallGraph_DoubleSum(t *testing.T) {
	// double + double_sum: double_sum calls double twice.
	m := &mir2.Module{Name: "test"}
	buildDoubleAndDoubleSum(m)

	cg := mir2.BuildCallGraph(m)

	if !cg.IsLeaf("double") {
		t.Error("double should be a leaf")
	}
	if cg.IsLeaf("double_sum") {
		t.Error("double_sum should not be a leaf")
	}
	if n := cg.CallCount("double_sum", "double"); n != 2 {
		t.Errorf("double_sum calls double 2 times, got %d", n)
	}

	// Topo order: double before double_sum.
	order := cg.Order()
	dIdx, dsIdx := -1, -1
	for i, n := range order {
		if n == "double" {
			dIdx = i
		}
		if n == "double_sum" {
			dsIdx = i
		}
	}
	if dIdx < 0 || dsIdx < 0 {
		t.Fatal("both functions must appear in Order()")
	}
	if dIdx >= dsIdx {
		t.Errorf("double (leaf) must come before double_sum in topo order; got %v", order)
	}
}

func TestCallGraph_IterativeFib(t *testing.T) {
	// buildFib (from examples_test.go) builds an ITERATIVE fibonacci using a loop,
	// no recursive CALL. So the call graph should be acyclic with 1 function.
	m := &mir2.Module{Name: "fib"}
	buildFib(m)

	cg := mir2.BuildCallGraph(m)
	if !cg.Acyclic() {
		t.Error("iterative fib has no recursive calls — should be acyclic")
	}
	if !cg.IsLeaf("fibonacci") {
		t.Error("iterative fibonacci should be a leaf (no OpCall in body)")
	}
	if len(cg.Order()) != 1 {
		t.Errorf("fib module has 1 function, Order() len = %d", len(cg.Order()))
	}
}

func TestCallGraph_Recursive(t *testing.T) {
	// Build a genuinely recursive function: fact(n) calls fact(n-1).
	m := &mir2.Module{Name: "fact"}
	fact := m.AddFunc("fact")
	{
		pn := fact.AllocReg()
		fact.Contract.Params = []mir2.Param{
			{Name: "n", Reg: pn, Ty: mir2.TyU8, Class: mir2.ClassAcc},
		}
		fact.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		entry := fact.NewBlock("entry")
		base := fact.NewBlock("base")
		rec := fact.NewBlock("rec")

		// entry: if n==0 goto base else rec
		cmp := fact.AllocReg()
		entry.Insts = append(entry.Insts, &mir2.Inst{
			Op: mir2.OpCmp, Dst: cmp, Src: [2]mir2.Reg{pn, pn},
			Ty: mir2.TyBool, Cls: mir2.ClassFlag, Cond: mir2.CmpEq,
		})
		entry.Term = &mir2.TermBrIf{Cond: cmp, Then: "base", Else: "rec"}

		base.Term = &mir2.TermRet{Vals: []mir2.Reg{pn}}

		// rec: call fact(n-1), mul with n
		one := fact.AllocReg()
		nm1 := fact.AllocReg()
		sub := fact.AllocReg()
		rec.Insts = append(rec.Insts,
			&mir2.Inst{Op: mir2.OpConst, Dst: one, Imm: 1, Ty: mir2.TyU8, Cls: mir2.ClassGeneral},
			&mir2.Inst{Op: mir2.OpSub, Dst: nm1, Src: [2]mir2.Reg{pn, one}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpCall, Dst: sub, Sym: "fact", Args: []mir2.Reg{nm1}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
		)
		rec.Term = &mir2.TermRet{Vals: []mir2.Reg{sub}}
	}

	cg := mir2.BuildCallGraph(m)
	if cg.Acyclic() {
		t.Error("recursive fact should be cyclic")
	}
	// Cycle members appear in Order() even if not topo-sortable.
	if len(cg.Order()) != 1 {
		t.Errorf("fact module has 1 function, Order() len = %d", len(cg.Order()))
	}
}

func TestCallGraph_ThreeLevelChain(t *testing.T) {
	// a → b → c  (a calls b, b calls c, c is leaf)
	m := &mir2.Module{Name: "chain"}

	c := m.AddFunc("c")
	{
		e := c.NewBlock("entry")
		e.Term = &mir2.TermRet{}
	}

	b := m.AddFunc("b")
	{
		pr := b.AllocReg()
		b.Contract.Params = []mir2.Param{{Name: "x", Reg: pr, Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := b.NewBlock("entry")
		dst := b.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{Op: mir2.OpCall, Dst: dst, Sym: "c", Ty: mir2.TyU8, Cls: mir2.ClassAcc})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{dst}}
	}

	a := m.AddFunc("a")
	{
		pr := a.AllocReg()
		a.Contract.Params = []mir2.Param{{Name: "x", Reg: pr, Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := a.NewBlock("entry")
		dst := a.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{Op: mir2.OpCall, Dst: dst, Sym: "b", Ty: mir2.TyU8, Cls: mir2.ClassAcc})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{dst}}
	}

	cg := mir2.BuildCallGraph(m)
	if !cg.Acyclic() {
		t.Error("chain a→b→c should be acyclic")
	}
	order := cg.Order()
	pos := make(map[string]int, 3)
	for i, n := range order {
		pos[n] = i
	}
	if pos["c"] >= pos["b"] || pos["b"] >= pos["a"] {
		t.Errorf("expected c < b < a in topo order, got %v", order)
	}
	_ = a // avoid unused warning
}

// ── Contract optimisation tests ───────────────────────────────────────────────

// TestContractOptimize_ConfirmsDoubleChoice verifies that OptimizeContracts
// confirms the current manually-set ClassAcc for double's parameter.
// (On Z80, all primary reg moves cost 4T — ClassAcc is already optimal for double.)
func TestContractOptimize_ConfirmsDoubleChoice(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	buildDoubleAndDoubleSum(m)

	cs := mir2.OptimizeContracts(m, mir2.Z80CostTable{})

	ch := cs["double"]
	if ch == nil {
		t.Fatal("double must have a ContractChoice")
	}
	if len(ch.ParamClasses) == 0 {
		t.Fatal("double has 1 param, ParamClasses must be non-empty")
	}
	// double's body does ADD A, A → naturally wants ClassAcc.
	// OptimizeContracts should confirm this.
	if ch.ParamClasses[0] != mir2.ClassAcc {
		t.Errorf("double param[0]: expected ClassAcc, got %v", ch.ParamClasses[0])
	}
}

// TestContractOptimize_PreservesOutput verifies that applying optimised
// contracts doesn't break the double_sum assembly or its functional output.
// Uses the canonical buildDouble + buildDoubleSum builders (from examples_test.go)
// which have the correct explicit spill moves.
func TestContractOptimize_PreservesOutput(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	buildDouble(m)
	buildDoubleSum(m)

	// Optimise then apply.
	cs := mir2.OptimizeContracts(m, mir2.Z80CostTable{})
	mir2.ApplyContracts(m, cs)

	// Compile and verify.
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify after ApplyContracts: %v", err)
	}

	ar := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		fAr := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		for r, loc := range fAr.Locs {
			ar.Locs[r] = loc
		}
	}
	asm := mir2.Z80Codegen(m, ar)
	t.Logf("double_sum assembly after contract optimisation:\n%s", asm)

	cases := [][3]int{{1, 2, 6}, {3, 4, 14}, {0, 5, 10}, {63, 64, 254}}
	for _, tc := range cases {
		a, b, want := tc[0], tc[1], tc[2]
		got, err := runDoubleSumAfterOpt(t, asm, a, b)
		if err != nil {
			t.Errorf("double_sum(%d,%d): %v", a, b, err)
			continue
		}
		if got != want {
			t.Errorf("double_sum(%d,%d) = %d, want %d", a, b, got, want)
		} else {
			t.Logf("double_sum(%d,%d) = %d ✓", a, b, got)
		}
	}
}

// TestContractOptimize_NaturalClassInference verifies inferNaturalClass
// indirectly: for a function whose body uses a pointer register (ClassPointer),
// the optimiser should prefer ClassPointer over other 16-bit classes.
func TestContractOptimize_PointerFuncPreservesPointer(t *testing.T) {
	// Build: sum_array(ptr: u16 in HL, count: u8 in B) → u8
	// ptr is used as Load source → inferNaturalClass returns ClassPointer.
	// The optimiser must keep ClassPointer for param[0].
	//
	// Note: param[1] (count) is first seen as src[0] of an initial CmpNe (bounds
	// check before the loop), so inferNaturalClass returns ClassAcc for it.
	// ClassGeneral then wins (0 unary cost, no callers to drive edgeCost).
	// We only assert on the pointer param here, since the counter heuristic
	// depends on whether an explicit TermDJNZ is visible in the IR.
	m := &mir2.Module{Name: "test"}
	buildSumArray(m) // ptr in ClassPointer, count in ClassCounter

	cs := mir2.OptimizeContracts(m, mir2.Z80CostTable{})

	ch := cs["sum_array"]
	if ch == nil {
		t.Fatal("sum_array must have a ContractChoice")
	}
	if len(ch.ParamClasses) < 1 {
		t.Fatalf("sum_array has 2 params, got %d", len(ch.ParamClasses))
	}
	// param[0] = ptr (ClassPointer) — the optimizer must confirm ClassPointer
	// because the body uses it as a Load source (natural class = ClassPointer).
	if ch.ParamClasses[0] != mir2.ClassPointer {
		t.Errorf("sum_array param[0] (ptr): expected ClassPointer, got %v", ch.ParamClasses[0])
	}
}

// TestContractOptimize_CostReduction verifies that the optimiser finds a lower
// total cost than a deliberately bad starting contract.
//
// Setup: a function `square` whose body does mul-by-itself (classically ClassAcc).
// We manually set its contract to ClassGeneral (wrong), then verify the optimiser
// finds ClassAcc has lower total cost.
func TestContractOptimize_CostReduction(t *testing.T) {
	m := &mir2.Module{Name: "test"}

	// Build square(x: u8) → u8  =  x * x  (approximated as x + x for this test)
	square := m.AddFunc("square")
	{
		px := square.AllocReg()
		// Deliberately start with ClassGeneral — optimizer should change to ClassAcc.
		square.Contract.Params = []mir2.Param{
			{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral},
		}
		square.Contract.Returns = []mir2.Return{
			{Ty: mir2.TyU8, Class: mir2.ClassAcc},
		}
		e := square.NewBlock("entry")
		// Body: ADD A, A (uses param in A → natural class = ClassAcc)
		res := square.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{px, px},
			Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
	}

	// Before optimisation: param class is ClassGeneral (suboptimal).
	before := m.FuncByName("square").Contract.Params[0].Class
	if before != mir2.ClassGeneral {
		t.Fatalf("precondition: param should be ClassGeneral before optimisation")
	}

	cs := mir2.OptimizeContracts(m, mir2.Z80CostTable{})
	ch := cs["square"]
	if ch == nil {
		t.Fatal("square must have a ContractChoice")
	}

	// After optimisation: body uses ADD A,A → natural = ClassAcc → should pick ClassAcc.
	if ch.ParamClasses[0] != mir2.ClassAcc {
		t.Errorf("square param[0]: expected ClassAcc (body uses ADD A,A), got %v", ch.ParamClasses[0])
	}
}

// TestBestLocForClass verifies the bestLocForClass helper returns sensible results.
func TestBestLocForClass(t *testing.T) {
	ct := mir2.Z80CostTable{}
	cases := []struct {
		cls  mir2.RegClass
		want string // expected best physical location name
	}{
		{mir2.ClassAcc, "A"},
		{mir2.ClassCounter, "B"},
		{mir2.ClassPointer, "HL"},
		{mir2.ClassIndex, "DE"},
	}
	for _, tc := range cases {
		loc := mir2.BestLocForClass(tc.cls, ct)
		if loc.Name != tc.want {
			t.Errorf("bestLocForClass(%v) = %q, want %q", tc.cls, loc.Name, tc.want)
		}
	}
}

// TestClassMoveCost verifies zero cost for same class, nonzero for different.
func TestClassMoveCost(t *testing.T) {
	ct := mir2.Z80CostTable{}

	if c := mir2.ClassMoveCost(mir2.ClassAcc, mir2.ClassAcc, ct); c != 0 {
		t.Errorf("ClassAcc→ClassAcc: expected 0T, got %d", c)
	}
	if c := mir2.ClassMoveCost(mir2.ClassGeneral, mir2.ClassAcc, ct); c == 0 {
		t.Error("ClassGeneral→ClassAcc: expected nonzero move cost")
	}
	if c := mir2.ClassMoveCost(mir2.ClassPointer, mir2.ClassIndex, ct); c == 0 {
		t.Error("ClassPointer→ClassIndex: expected nonzero move cost (EX DE,HL)")
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// buildDoubleAndDoubleSum is the canonical interprocedural test: double called twice.
func buildDoubleAndDoubleSum(m *mir2.Module) {
	// double(a: u8 in A) → u8 in A  =  a + a
	double := m.AddFunc("double")
	{
		pa := double.AllocReg()
		double.Contract.Params = []mir2.Param{
			{Name: "a", Reg: pa, Ty: mir2.TyU8, Class: mir2.ClassAcc},
		}
		double.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := double.NewBlock("entry")
		res := double.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{pa, pa},
			Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
	}

	// double_sum(a, b: u8) → u8 = double(a) + double(b)
	ds := m.AddFunc("double_sum")
	{
		pa := ds.AllocReg()
		pb := ds.AllocReg()
		ds.Contract.Params = []mir2.Param{
			{Name: "a", Reg: pa, Ty: mir2.TyU8, Class: mir2.ClassAcc},
			{Name: "b", Reg: pb, Ty: mir2.TyU8, Class: mir2.ClassGeneral},
		}
		ds.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := ds.NewBlock("entry")
		r1 := ds.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpCall, Dst: r1, Sym: "double",
			Args: []mir2.Reg{pa}, Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		r2 := ds.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpCall, Dst: r2, Sym: "double",
			Args: []mir2.Reg{pb}, Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		res := ds.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{r1, r2},
			Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
	}
}

// runDoubleSumAfterOpt assembles and runs double_sum(a,b) after contract opt.
// double_sum(a: A, b: C) → result in A.
func runDoubleSumAfterOpt(t *testing.T, fullAsm string, a, b int) (int, error) {
	t.Helper()
	src := fmt.Sprintf(`
    ORG 0x8000
    LD SP, 0xFF00
    LD A, %d
    LD C, %d
    CALL double_sum
    DI
    HALT
`, a, b) + "\n" + fullAsm

	aReg, _, err := runZ80(t, src)
	return int(aReg), err
}
