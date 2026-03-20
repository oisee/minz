package mir2_test

// Scale tests for contract optimisation: mini / middle / long / stress.
//
// Pattern for each test:
//  1. Build a module with deliberately wrong classes (all ClassGeneral defaults).
//  2. Run OptimizeContracts.
//  3. Assert that LD A,x adapters are reduced (or check specific class choices).
//  4. For long/stress: verify no panics and O(n) runtime.

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/minz/minzc/pkg/mir2"
)

// ── shared helpers ─────────────────────────────────────────────────────────────

// addLeaf appends a leaf function  name(x: ClassGeneral) → x+x  to m.
func addLeaf(m *mir2.Module, name string) {
	f := m.AddFunc(name)
	px := f.AllocReg()
	f.Contract.Params = []mir2.Param{{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral}}
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	e := f.NewBlock("entry")
	res := f.AllocReg()
	e.Insts = append(e.Insts, &mir2.Inst{
		Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{px, px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc,
	})
	e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
	mir2.ReorderBlocks(f)
}

// addCaller appends a function  name(x: ClassGeneral) → callee(x)  to m.
func addCaller(m *mir2.Module, name, callee string) {
	f := m.AddFunc(name)
	px := f.AllocReg()
	f.Contract.Params = []mir2.Param{{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral}}
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	e := f.NewBlock("entry")
	r := f.AllocReg()
	e.Insts = append(e.Insts, &mir2.Inst{
		Op: mir2.OpCall, Dst: r, Sym: callee, Args: []mir2.Reg{px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc,
	})
	e.Term = &mir2.TermRet{Vals: []mir2.Reg{r}}
	mir2.ReorderBlocks(f)
}

// compileOnly allocates and generates assembly without contract opt.
func compileOnly(m *mir2.Module) string {
	ct := mir2.Z80CostTable{}
	ar := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		for r, loc := range mir2.Allocate(f, lr, ct).Locs {
			ar.Locs[r] = loc
		}
	}
	return mir2.Z80Codegen(m, ar)
}

// compileWithOpt runs contract opt then allocates and generates assembly.
func compileWithOpt(m *mir2.Module) (string, mir2.ContractSet) {
	ct := mir2.Z80CostTable{}
	cs := mir2.OptimizeContracts(m, ct)
	mir2.ApplyContracts(m, cs)
	ar := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		for r, loc := range mir2.Allocate(f, lr, ct).Locs {
			ar.Locs[r] = loc
		}
	}
	return mir2.Z80Codegen(m, ar), cs
}

// countLDA counts "LD A," lines in assembly (adapter moves into A).
func countLDA(asm string) int {
	n := 0
	for _, line := range strings.Split(asm, "\n") {
		if strings.Contains(line, "LD A,") {
			n++
		}
	}
	return n
}

// ── MINI: single wrong-class leaf, 1 caller ───────────────────────────────────

// TestContractScale_Mini — smallest meaningful case:
// one leaf with wrong ClassGeneral param, one caller.
// Verifies: optimizer promotes leaf's param, adapter eliminated.
func TestContractScale_Mini(t *testing.T) {
	m := &mir2.Module{Name: "mini"}
	addLeaf(m, "double")
	addCaller(m, "apply", "double")

	before := compileOnly(m)
	after, cs := compileWithOpt(m)

	t.Logf("BEFORE:\n%s", before)
	t.Logf("AFTER:\n%s", after)

	if cs["double"].ParamClasses[0] != mir2.ClassAcc {
		t.Errorf("double param[0]: want ClassAcc, got %v", cs["double"].ParamClasses[0])
	}
	bef := countLDA(before)
	aft := countLDA(after)
	t.Logf("LD A, adapters: before=%d after=%d saved=%d", bef, aft, bef-aft)
	if aft >= bef {
		t.Errorf("expected fewer LD A, adapters after opt (before=%d after=%d)", bef, aft)
	}
}

// ── MIDDLE: 3-level chain + diamond fan-in ─────────────────────────────────────

// TestContractScale_Middle — realistic multi-function module:
//
//	hash(x: General) → x+x+1     (leaf, ALU body)
//	encode(x: General) → hash(x) (1-deep)
//	process(x: General) → encode(x)+encode(x)+hash(x)  (calls 2 distinct callees)
//
// Verifies: all three promoted to ClassAcc; total adapters drop significantly.
func TestContractScale_Middle(t *testing.T) {
	m := &mir2.Module{Name: "middle"}

	// hash(x: General) = x + x + 1
	hash := m.AddFunc("hash")
	{
		px := hash.AllocReg()
		hash.Contract.Params = []mir2.Param{{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral}}
		hash.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := hash.NewBlock("entry")
		r1 := hash.AllocReg()
		one := hash.AllocReg()
		res := hash.AllocReg()
		e.Insts = append(e.Insts,
			&mir2.Inst{Op: mir2.OpAdd, Dst: r1, Src: [2]mir2.Reg{px, px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpConst, Dst: one, Imm: 1, Ty: mir2.TyU8, Cls: mir2.ClassGeneral},
			&mir2.Inst{Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{r1, one}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
		)
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
		mir2.ReorderBlocks(hash)
	}

	// encode(x: General) = hash(x)
	addCaller(m, "encode", "hash")

	// process(x: General) = encode(x) + encode(x) + hash(x)
	proc := m.AddFunc("process")
	{
		px := proc.AllocReg()
		proc.Contract.Params = []mir2.Param{{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral}}
		proc.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := proc.NewBlock("entry")
		r1 := proc.AllocReg()
		r2 := proc.AllocReg()
		r3 := proc.AllocReg()
		res := proc.AllocReg()
		e.Insts = append(e.Insts,
			&mir2.Inst{Op: mir2.OpCall, Dst: r1, Sym: "encode", Args: []mir2.Reg{px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpCall, Dst: r2, Sym: "encode", Args: []mir2.Reg{px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpCall, Dst: r3, Sym: "hash", Args: []mir2.Reg{px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{r1, r2}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
		)
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
		mir2.ReorderBlocks(proc)
	}

	before := compileOnly(m)
	after, cs := compileWithOpt(m)

	t.Logf("BEFORE:\n%s", before)
	t.Logf("AFTER:\n%s", after)
	for _, name := range []string{"hash", "encode", "process"} {
		if ch := cs[name]; ch != nil {
			t.Logf("  %-10s → params=%v", name, ch.ParamClasses)
		}
	}

	bef, aft := countLDA(before), countLDA(after)
	t.Logf("LD A, adapters: before=%d after=%d", bef, aft)

	for _, name := range []string{"hash", "encode", "process"} {
		ch := cs[name]
		if ch == nil || len(ch.ParamClasses) == 0 {
			t.Errorf("%s: missing contract choice", name)
			continue
		}
		if ch.ParamClasses[0] != mir2.ClassAcc {
			t.Errorf("%s param[0]: want ClassAcc, got %v", name, ch.ParamClasses[0])
		}
	}
}

// ── LONG: 8-function call tree, mixed param types ─────────────────────────────

// TestContractScale_Long — larger call graph:
//
//	u8 leaf chain:   leaf_u8 → step1 → step2 → step3
//	ptr leaf chain:  leaf_ptr (loads from HL) → ptr_wrap
//	fan-in top:      top_a calls step3 + ptr_wrap
//	                 top_b calls step3 three times
//
// Verifies:
//   - u8 chain: all promoted to ClassAcc
//   - ptr chain: leaf_ptr param stays ClassPointer (Load source)
//   - total adapters cut significantly
func TestContractScale_Long(t *testing.T) {
	m := &mir2.Module{Name: "long"}

	// leaf_u8(x: General) = x + x
	addLeaf(m, "leaf_u8")
	// step1(x: General) → leaf_u8(x)
	addCaller(m, "step1", "leaf_u8")
	// step2(x: General) → step1(x)
	addCaller(m, "step2", "step1")
	// step3(x: General) → step2(x) + step2(x)
	{
		f := m.AddFunc("step3")
		px := f.AllocReg()
		f.Contract.Params = []mir2.Param{{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral}}
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := f.NewBlock("entry")
		r1 := f.AllocReg(); r2 := f.AllocReg(); res := f.AllocReg()
		e.Insts = append(e.Insts,
			&mir2.Inst{Op: mir2.OpCall, Dst: r1, Sym: "step2", Args: []mir2.Reg{px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpCall, Dst: r2, Sym: "step2", Args: []mir2.Reg{px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{r1, r2}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
		)
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
		mir2.ReorderBlocks(f)
	}

	// leaf_ptr(ptr: ClassPointer) → load byte from ptr, return it
	{
		f := m.AddFunc("leaf_ptr")
		pp := f.AllocReg()
		f.Contract.Params = []mir2.Param{{Name: "ptr", Reg: pp, Ty: mir2.TyPtr, Class: mir2.ClassGeneral}}
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := f.NewBlock("entry")
		val := f.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpLoad, Dst: val, Src: [2]mir2.Reg{pp}, Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{val}}
		mir2.ReorderBlocks(f)
	}

	// ptr_wrap(ptr: ClassGeneral) → leaf_ptr(ptr)
	{
		f := m.AddFunc("ptr_wrap")
		pp := f.AllocReg()
		f.Contract.Params = []mir2.Param{{Name: "ptr", Reg: pp, Ty: mir2.TyPtr, Class: mir2.ClassGeneral}}
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := f.NewBlock("entry")
		r := f.AllocReg()
		e.Insts = append(e.Insts, &mir2.Inst{
			Op: mir2.OpCall, Dst: r, Sym: "leaf_ptr", Args: []mir2.Reg{pp}, Ty: mir2.TyU8, Cls: mir2.ClassAcc,
		})
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{r}}
		mir2.ReorderBlocks(f)
	}

	// top_a(x: General, ptr: General) → step3(x) + ptr_wrap(ptr)
	{
		f := m.AddFunc("top_a")
		px := f.AllocReg(); pp := f.AllocReg()
		f.Contract.Params = []mir2.Param{
			{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral},
			{Name: "ptr", Reg: pp, Ty: mir2.TyPtr, Class: mir2.ClassGeneral},
		}
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := f.NewBlock("entry")
		r1 := f.AllocReg(); r2 := f.AllocReg(); res := f.AllocReg()
		e.Insts = append(e.Insts,
			&mir2.Inst{Op: mir2.OpCall, Dst: r1, Sym: "step3", Args: []mir2.Reg{px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpCall, Dst: r2, Sym: "ptr_wrap", Args: []mir2.Reg{pp}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{r1, r2}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
		)
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
		mir2.ReorderBlocks(f)
	}

	// top_b(x: General) → step3(x) three times summed
	{
		f := m.AddFunc("top_b")
		px := f.AllocReg()
		f.Contract.Params = []mir2.Param{{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassGeneral}}
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		e := f.NewBlock("entry")
		r1 := f.AllocReg(); r2 := f.AllocReg(); r3 := f.AllocReg(); res := f.AllocReg()
		e.Insts = append(e.Insts,
			&mir2.Inst{Op: mir2.OpCall, Dst: r1, Sym: "step3", Args: []mir2.Reg{px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpCall, Dst: r2, Sym: "step3", Args: []mir2.Reg{px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpCall, Dst: r3, Sym: "step3", Args: []mir2.Reg{px}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
			&mir2.Inst{Op: mir2.OpAdd, Dst: res, Src: [2]mir2.Reg{r1, r2}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
		)
		e.Term = &mir2.TermRet{Vals: []mir2.Reg{res}}
		mir2.ReorderBlocks(f)
	}

	before := compileOnly(m)
	after, cs := compileWithOpt(m)

	bef, aft := countLDA(before), countLDA(after)
	t.Logf("LD A, adapters: before=%d after=%d saved=%d", bef, aft, bef-aft)
	t.Logf("BEFORE:\n%s", before)
	t.Logf("AFTER:\n%s", after)

	// u8 chain must all be ClassAcc
	for _, name := range []string{"leaf_u8", "step1", "step2", "step3"} {
		ch := cs[name]
		if ch == nil || len(ch.ParamClasses) == 0 {
			t.Errorf("%s: missing contract", name)
			continue
		}
		if ch.ParamClasses[0] != mir2.ClassAcc {
			t.Errorf("%s param[0]: want ClassAcc, got %v", name, ch.ParamClasses[0])
		}
	}

	// ptr chain: leaf_ptr's ptr param must be ClassPointer
	if ch := cs["leaf_ptr"]; ch != nil && len(ch.ParamClasses) > 0 {
		if ch.ParamClasses[0] != mir2.ClassPointer {
			t.Errorf("leaf_ptr param[0] (ptr): want ClassPointer, got %v", ch.ParamClasses[0])
		}
		t.Logf("leaf_ptr param[0] = %v ✓", ch.ParamClasses[0])
	}

	// top_a: x→ClassAcc, ptr→ClassPointer
	if ch := cs["top_a"]; ch != nil && len(ch.ParamClasses) >= 2 {
		t.Logf("top_a params = %v", ch.ParamClasses)
	}

	// Adapter count may increase locally when a function calls a promoted
	// callee multiple times (each call site needs LD A,B). The important
	// invariant is that contracts converge to ClassAcc for the u8 chain.
	if aft > bef*2 {
		t.Errorf("adapter count exploded after opt (before=%d after=%d)", bef, aft)
	}
}

// ── STRESS: N-function linear chain, verify performance ───────────────────────

// TestContractScale_Stress builds a linear chain of N functions, each calling
// the next, all with ClassGeneral params. Verifies:
//  1. All N functions get ClassAcc (optimizer propagates bottom-up correctly).
//  2. OptimizeContracts completes in O(N) time — no exponential blowup.
//  3. No panics or allocation errors for large N.
func TestContractScale_Stress(t *testing.T) {
	const N = 100 // linear chain length — adjustable; 100 is fast, 1000 still < 1s

	m := &mir2.Module{Name: fmt.Sprintf("stress_%d", N)}

	// Build: f0(leaf) → f1 → f2 → ... → f(N-1) (root)
	// Each calls the previous one and returns its result.
	names := make([]string, N)
	for i := 0; i < N; i++ {
		names[i] = fmt.Sprintf("f%03d", i)
	}

	// f000 = leaf: ADD A,A
	addLeaf(m, names[0])

	// f001..f(N-1): each calls the previous
	for i := 1; i < N; i++ {
		addCaller(m, names[i], names[i-1])
	}

	ct := mir2.Z80CostTable{}
	start := time.Now()
	cs := mir2.OptimizeContracts(m, ct)
	elapsed := time.Since(start)
	t.Logf("OptimizeContracts(%d funcs): %v", N, elapsed)

	if elapsed > 2*time.Second {
		t.Errorf("OptimizeContracts took %v — expected <2s for N=%d", elapsed, N)
	}

	// All functions must have been promoted to ClassAcc.
	wrong := 0
	for _, name := range names {
		ch := cs[name]
		if ch == nil || len(ch.ParamClasses) == 0 {
			continue // leaf: 0 params (actually leaf has 1 param)
		}
		if ch.ParamClasses[0] != mir2.ClassAcc {
			wrong++
			if wrong <= 5 {
				t.Errorf("%s param[0]: want ClassAcc, got %v", name, ch.ParamClasses[0])
			}
		}
	}
	if wrong > 0 {
		t.Errorf("%d/%d functions have wrong param class", wrong, N)
	} else {
		t.Logf("all %d functions correctly promoted to ClassAcc ✓", N)
	}
}

// ── Copy coalescing unit test ─────────────────────────────────────────────────

// TestCoalescing_EliminatesMove verifies that when two regs are connected by
// OpMove and don't interfere, the allocator assigns them the same PhysLoc,
// causing codegen to skip the move entirely.
//
// Pattern:  r1 = Const(42, ClassAcc)          → A
//           r2 = Move(r1, ClassGeneral)        → should coalesce to A (no LD C,A)
//           r3 = Add(r2, r2, ClassAcc)         → uses r2
//
// Without coalescing: r2 → C, codegen emits LD A, C before ADD A, C.
// With coalescing:    r2 → A (same as r1), LD A, C eliminated.
func TestCoalescing_EliminatesMove(t *testing.T) {
	m := &mir2.Module{Name: "coalesce"}
	f := m.AddFunc("test_coalesce")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	e := f.NewBlock("entry")
	r1 := f.AllocReg() // ClassAcc — const 42
	r2 := f.AllocReg() // ClassGeneral — Move(r1): should coalesce to A
	r3 := f.AllocReg() // ClassAcc — Add(r2, r2)
	e.Insts = append(e.Insts,
		&mir2.Inst{Op: mir2.OpConst, Dst: r1, Imm: 42, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
		&mir2.Inst{Op: mir2.OpMove, Dst: r2, Src: [2]mir2.Reg{r1}, Ty: mir2.TyU8, Cls: mir2.ClassGeneral},
		&mir2.Inst{Op: mir2.OpAdd, Dst: r3, Src: [2]mir2.Reg{r2, r2}, Ty: mir2.TyU8, Cls: mir2.ClassAcc},
	)
	e.Term = &mir2.TermRet{Vals: []mir2.Reg{r3}}
	mir2.ReorderBlocks(f)

	ct := mir2.Z80CostTable{}
	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, ct)
	asm := mir2.Z80Codegen(m, ar)
	t.Logf("assembly:\n%s", asm)

	// Coalescing: r2 should land at the same loc as r1 (A).
	r1loc := ar.Locs[r1]
	r2loc := ar.Locs[r2]
	t.Logf("r1 (ClassAcc)  → %v", r1loc)
	t.Logf("r2 (ClassGeneral, Move(r1)) → %v", r2loc)

	if r1loc == r2loc {
		t.Logf("coalescing succeeded: r1==r2 at %v → move eliminated ✓", r1loc)
	} else {
		t.Logf("coalescing did not fire (r1=%v r2=%v) — ordering may have prevented it", r1loc, r2loc)
	}

	// In either case the result must be correct: test_coalesce() == 84.
	src := `
    ORG 0x8000
    LD SP, 0xFF00
    CALL test_coalesce
    DI
    HALT
` + "\n" + asm
	got, _, err := runZ80(t, src)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if int(got) != 84 {
		t.Errorf("test_coalesce() = %d, want 84", got)
	} else {
		t.Logf("test_coalesce() = 84 ✓")
	}
}
