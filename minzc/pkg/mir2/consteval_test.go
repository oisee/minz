package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// ── EvalPure / Eval1 harness tests ───────────────────────────────────────────

// TestEvalPure_SimpleArith: verify the harness works for a trivial function.
func TestEvalPure_SimpleArith(t *testing.T) {
	// fun double(x: u8) -> u8 { return x + x }
	m := buildDoubleModule()
	got, err := mir2.Eval1(m, "double", 21)
	if err != nil {
		t.Fatalf("Eval1(double, 21): %v", err)
	}
	if got != 42 {
		t.Errorf("double(21) = %d, want 42", got)
	}
}

func TestEvalPure_Fibonacci(t *testing.T) {
	m := &mir2.Module{Name: "fib"}
	buildFib(m)

	cases := [][2]int64{{0, 0}, {1, 1}, {5, 5}, {10, 55}, {12, 144}}
	for _, tc := range cases {
		got, err := mir2.Eval1(m, "fibonacci", tc[0])
		if err != nil {
			t.Errorf("fibonacci(%d): %v", tc[0], err)
			continue
		}
		if got != tc[1] {
			t.Errorf("fibonacci(%d) = %d, want %d", tc[0], got, tc[1])
		}
	}
}

func TestEvalPure_Popcount(t *testing.T) {
	m := buildPopcountModule()
	cases := [][2]int64{
		{0, 0}, {1, 1}, {0xFF, 8}, {0b10110111, 6}, {0b11111111, 8},
	}
	for _, tc := range cases {
		got, err := mir2.Eval1(m, "popcount", tc[0])
		if err != nil {
			t.Errorf("popcount(%08b): %v", tc[0], err)
			continue
		}
		if got != tc[1] {
			t.Errorf("popcount(%08b) = %d, want %d", tc[0], got, tc[1])
		}
	}
}

func TestEvalTable_Double(t *testing.T) {
	// EvalTable computes double(0..7) = [0,2,4,6,8,10,12,14].
	m := buildDoubleModule()
	table, err := mir2.EvalTable(m, "double", 0, 8)
	if err != nil {
		t.Fatalf("EvalTable: %v", err)
	}
	want := []int64{0, 2, 4, 6, 8, 10, 12, 14}
	for i, w := range want {
		if table[i] != w {
			t.Errorf("table[%d] = %d, want %d", i, table[i], w)
		}
	}
}

func TestEvalTable_FibSmall(t *testing.T) {
	// EvalTable for fibonacci over 0..10 — useful for LUT generation.
	m := &mir2.Module{Name: "fib"}
	buildFib(m)
	table, err := mir2.EvalTable(m, "fibonacci", 0, 11)
	if err != nil {
		t.Fatalf("EvalTable(fibonacci): %v", err)
	}
	want := []int64{0, 1, 1, 2, 3, 5, 8, 13, 21, 34, 55}
	for i, w := range want {
		if table[i] != w {
			t.Errorf("fib[%d] = %d, want %d", i, table[i], w)
		}
	}
}

// ── ConstantCallElim tests ────────────────────────────────────────────────────

func TestConstCallElim_PureDouble(t *testing.T) {
	// caller() { return double(42) }
	// double(42) → 84 at compile time → caller emits LD A, 84; RET
	m := buildDoubleModule()

	caller := m.AddFunc("caller")
	caller.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(caller)
	b.SwitchToNewBlock("entry")
	arg := b.Const(42, mir2.TyU8, mir2.ClassAcc)
	result := b.Call("double", []mir2.Reg{arg}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})
	b.Ret(result)

	changed := mir2.ConstantCallElim(m, caller)
	if !changed {
		t.Fatal("ConstantCallElim: expected call double(42) to be folded")
	}
	mir2.DeadStoreElim(caller)

	lr := mir2.ComputeLiveness(caller)
	ar := mir2.Allocate(caller, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	// caller should now be just: LD A, 84; RET (no CALL double)
	if strings.Contains(asm, "CALL double") {
		t.Errorf("CALL double should be eliminated; got:\n%s", asm)
	}
	if !strings.Contains(asm, "LD A, 84") {
		t.Errorf("expected LD A, 84 (double(42) evaluated); got:\n%s", asm)
	}
}

func TestConstCallElim_NonConstArg(t *testing.T) {
	// caller(x: u8) { return double(x) }
	// x is a function param, not a constant — call must be KEPT.
	m := buildDoubleModule()

	caller := m.AddFunc("caller2")
	caller.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(caller)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	result := b.Call("double", []mir2.Reg{x}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})
	b.Ret(result)

	changed := mir2.ConstantCallElim(m, caller)
	if changed {
		t.Error("ConstantCallElim: must NOT fold call when arg is a non-constant param")
	}
}

func TestConstCallElim_FibonacciConstArg(t *testing.T) {
	// Calling fibonacci(10) at compile time → Const(55).
	// (fibonacci is non-recursive in our iterative implementation)
	m := &mir2.Module{Name: "fib"}
	buildFib(m)

	caller := m.AddFunc("get_fib10")
	caller.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(caller)
	b.SwitchToNewBlock("entry")
	n := b.Const(10, mir2.TyU8, mir2.ClassAcc)
	result := b.Call("fibonacci", []mir2.Reg{n}, mir2.TyU16, mir2.ClassPointer, mir2.CallAttrs{})
	b.Ret(result)

	// Note: fibonacci is iterative, not marked IsRecursive.
	changed := mir2.ConstantCallElim(m, caller)
	if !changed {
		t.Fatal("ConstantCallElim: fibonacci(10) with const arg should be folded")
	}

	// Verify result is correct (55).
	mir2.DeadStoreElim(caller)
	lr := mir2.ComputeLiveness(caller)
	ar := mir2.Allocate(caller, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	if strings.Contains(asm, "CALL fibonacci") {
		t.Errorf("CALL fibonacci should be eliminated; got:\n%s", asm)
	}
	if !strings.Contains(asm, "55") {
		t.Errorf("expected 55 (fib(10)) in output; got:\n%s", asm)
	}
}

func TestConstCallElim_ChainedCalls(t *testing.T) {
	// apply_twice(42): double(double(42)) = double(84) = 168
	// Both calls fold at compile time in one fixpoint iteration.
	m := buildDoubleModule()

	caller := m.AddFunc("apply_twice_const")
	caller.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(caller)
	b.SwitchToNewBlock("entry")
	arg := b.Const(42, mir2.TyU8, mir2.ClassAcc)
	r1 := b.Call("double", []mir2.Reg{arg}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})
	r2 := b.Call("double", []mir2.Reg{r1}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})
	b.Ret(r2)

	// Both calls fold in ONE pass: after folding double(42)→84, consts[r1]=84
	// is updated immediately, so the next instruction double(r1) sees r1=84 → 168.
	changed := mir2.ConstantCallElim(m, caller)
	if !changed {
		t.Fatal("ConstantCallElim: expected both chained calls to be folded")
	}
	// Second pass must return false — nothing left to fold.
	if mir2.ConstantCallElim(m, caller) {
		t.Error("second ConstantCallElim pass should be a no-op (already fully folded)")
	}
	mir2.DeadStoreElim(caller)

	lr := mir2.ComputeLiveness(caller)
	ar := mir2.Allocate(caller, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)
	t.Log("\n" + asm)

	if strings.Contains(asm, "CALL double") {
		t.Errorf("both CALL double should be eliminated; got:\n%s", asm)
	}
	if !strings.Contains(asm, "168") {
		t.Errorf("expected 168 (double(double(42))) in output; got:\n%s", asm)
	}
}

// ── Helpers ───────────────────────────────────────────────────────────────────

// buildDoubleModule builds a module with a single `double` function.
func buildDoubleModule() *mir2.Module {
	m := &mir2.Module{Name: "double_mod"}
	f := m.AddFunc("double")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	r := b.Add(x, x, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r)
	return m
}

// buildPopcountModule builds a module with a popcount(x: u8) → u8 function.
//
// fun popcount(x: u8) -> u8 {
//     var n: u8 = 0
//     while x != 0 {
//         n += x & 1
//         x >>= 1
//     }
//     return n
// }
func buildPopcountModule() *mir2.Module {
	m := &mir2.Module{Name: "popcount_mod"}
	f := m.AddFunc("popcount")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x0 := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	n0 := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
	b.Jmp("loop_head", x0, n0)

	head := b.SwitchToNewBlock("loop_head")
	xH := b.BlockParam(head, mir2.TyU8, mir2.ClassAcc)
	nH := b.BlockParam(head, mir2.TyU8, mir2.ClassGeneral)
	zero := b.Const(0, mir2.TyU8, mir2.ClassAcc)
	cond := b.Cmp(mir2.CmpNe, xH, zero, mir2.ClassFlag, false)
	b.BrIf(cond, "loop_body", []mir2.Reg{xH, nH}, "exit", []mir2.Reg{nH})

	body := b.SwitchToNewBlock("loop_body")
	xB := b.BlockParam(body, mir2.TyU8, mir2.ClassAcc)
	nB := b.BlockParam(body, mir2.TyU8, mir2.ClassGeneral)
	one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	bit := b.And(xB, one, mir2.TyU8, mir2.ClassGeneral) // x & 1
	nNext := b.Add(nB, bit, mir2.TyU8, mir2.ClassGeneral) // n + (x&1)
	xNext := b.Shr(xB, one, mir2.TyU8, mir2.ClassAcc)    // x >> 1
	b.Jmp("loop_head", xNext, nNext)

	exit := b.SwitchToNewBlock("exit")
	result := b.BlockParam(exit, mir2.TyU8, mir2.ClassAcc)
	b.Ret(result)

	return m
}
