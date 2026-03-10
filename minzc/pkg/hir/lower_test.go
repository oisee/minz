package hir_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// ── pipeline helpers ──────────────────────────────────────────────────────────

const testLoadAddr = 0x8000

// compileHIR: HIR → MIR2 → DSE → alloc → Z80 assembly string.
func compileHIR(t *testing.T, hm *hir.Module) string {
	t.Helper()
	m := hir.LowerModule(hm)

	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
		mir2.DeadStoreElim(f)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}
	combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		for r, loc := range ar.Locs {
			combined.Locs[r] = loc
		}
	}
	return mir2.Z80Codegen(m, combined)
}

// runZ80 assembles and runs; returns (A, HL, error).
func runZ80(t *testing.T, src string) (a uint8, hl uint16, err error) {
	t.Helper()
	asm := z80asm.NewAssembler()
	res, asmErr := asm.AssembleString(src)
	if asmErr != nil {
		return 0, 0, fmt.Errorf("assemble: %w", asmErr)
	}
	if len(res.Errors) > 0 {
		var sb strings.Builder
		for _, e := range res.Errors {
			sb.WriteString(e.Error())
			sb.WriteByte('\n')
		}
		return 0, 0, fmt.Errorf("assemble errors:\n%s", sb.String())
	}
	z80 := emulator.NewRemogattoZ80()
	if loadErr := z80.LoadMemory(testLoadAddr, res.Binary); loadErr != nil {
		return 0, 0, fmt.Errorf("load memory: %w", loadErr)
	}
	z80.SetPC(testLoadAddr)
	if runErr := z80.Run(); runErr != nil {
		return 0, 0, fmt.Errorf("run: %w", runErr)
	}
	regs := z80.GetRegisters()
	return regs.A, regs.HL, nil
}

func boot1(fn string, a int) string {
	return fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n    LD A, %d\n    CALL %s\n    DI\n    HALT\n",
		testLoadAddr, a, fn)
}
func boot2(fn string, a, c int) string {
	return fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n    LD A, %d\n    LD C, %d\n    CALL %s\n    DI\n    HALT\n",
		testLoadAddr, a, c, fn)
}
func boot3(fn string, a, c, b int) string {
	return fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n    LD A, %d\n    LD C, %d\n    LD B, %d\n    CALL %s\n    DI\n    HALT\n",
		testLoadAddr, a, c, b, fn)
}

// ── add(a, b u8) u8 ───────────────────────────────────────────────────────────

func TestHIRAdd(t *testing.T) {
	hm := &hir.Module{Name: "test_add"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:   "add",
		Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body: hir.Blk(
			hir.Ret(hir.Add(
				hir.Var("a", mir2.TyU8),
				hir.Var("b", mir2.TyU8),
				mir2.TyU8,
			)),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("generated:\n%s", genAsm)

	a, _, err := runZ80(t, boot2("add", 3, 4)+genAsm)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if a != 7 {
		t.Errorf("add(3,4) = %d, want 7", a)
	}
}

// ── clamp(v, lo, hi u8) u8 ────────────────────────────────────────────────────

func TestHIRClamp(t *testing.T) {
	cases := []struct{ v, lo, hi, want uint8 }{
		{5, 3, 8, 5},
		{1, 3, 8, 3},
		{9, 3, 8, 8},
		{3, 3, 8, 3},
		{8, 3, 8, 8},
	}

	v := hir.Var("v", mir2.TyU8)
	lo := hir.Var("lo", mir2.TyU8)
	hi := hir.Var("hi", mir2.TyU8)

	hm := &hir.Module{Name: "test_clamp"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name: "clamp",
		Params: []hir.Param{
			{Name: "v", Ty: mir2.TyU8},
			{Name: "lo", Ty: mir2.TyU8},
			{Name: "hi", Ty: mir2.TyU8},
		},
		RetTy: mir2.TyU8,
		Body: hir.Blk(
			hir.If(hir.Lt(v, lo), hir.Blk(hir.Ret(lo)), nil),
			hir.If(hir.Gt(v, hi), hir.Blk(hir.Ret(hi)), nil),
			hir.Ret(v),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("generated:\n%s", genAsm)

	for _, tc := range cases {
		got, _, err := runZ80(t, boot3("clamp", int(tc.v), int(tc.lo), int(tc.hi))+genAsm)
		if err != nil {
			t.Fatalf("clamp(%d,%d,%d): %v", tc.v, tc.lo, tc.hi, err)
		}
		if got != tc.want {
			t.Errorf("clamp(%d,%d,%d) = %d, want %d", tc.v, tc.lo, tc.hi, got, tc.want)
		}
	}
}

// ── sum_range(n u8) u8 — while loop ───────────────────────────────────────────

func TestHIRSumRange(t *testing.T) {
	n := hir.Var("n", mir2.TyU8)
	s := hir.Var("sum", mir2.TyU8)
	i := hir.Var("i", mir2.TyU8)
	zero := hir.U8(0)
	one := hir.U8(1)

	hm := &hir.Module{Name: "test_sum"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:   "sum_range",
		Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body: hir.Blk(
			hir.Decl("sum", mir2.TyU8, zero),
			hir.Decl("i", mir2.TyU8, zero),
			hir.While(
				hir.Lt(i, n),
				hir.Blk(
					hir.Assign(hir.Var("sum", mir2.TyU8), hir.Add(s, i, mir2.TyU8)),
					hir.Assign(hir.Var("i", mir2.TyU8), hir.Add(i, one, mir2.TyU8)),
				),
			),
			hir.Ret(s),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("generated:\n%s", genAsm)

	cases := []struct{ n, want uint8 }{
		{0, 0}, {1, 0}, {5, 10}, {6, 15},
	}
	for _, tc := range cases {
		got, _, err := runZ80(t, boot1("sum_range", int(tc.n))+genAsm)
		if err != nil {
			t.Fatalf("sum_range(%d): %v", tc.n, err)
		}
		if got != tc.want {
			t.Errorf("sum_range(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

// ── sum_for(n u8) u8 — for-range loop ────────────────────────────────────────

func TestHIRSumFor(t *testing.T) {
	n := hir.Var("n", mir2.TyU8)
	s := hir.Var("sum", mir2.TyU8)
	i := hir.Var("i", mir2.TyU8)
	zero := hir.U8(0)

	hm := &hir.Module{Name: "test_for"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:   "sum_for",
		Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body: hir.Blk(
			hir.Decl("sum", mir2.TyU8, zero),
			hir.For("i", zero, n,
				hir.Blk(
					hir.Assign(hir.Var("sum", mir2.TyU8), hir.Add(s, i, mir2.TyU8)),
				),
			),
			hir.Ret(s),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("generated:\n%s", genAsm)

	cases := []struct{ n, want uint8 }{
		{0, 0}, {5, 10}, {6, 15},
	}
	for _, tc := range cases {
		got, _, err := runZ80(t, boot1("sum_for", int(tc.n))+genAsm)
		if err != nil {
			t.Fatalf("sum_for(%d): %v", tc.n, err)
		}
		if got != tc.want {
			t.Errorf("sum_for(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

// ── fibonacci(n u8) u8 — iterative, 3-var loop + body-local temp ──────────────
//
// fun fib(n u8) u8 {
//     let a = 0; let b = 1; let i = 0
//     while i < n { let t = a+b; a = b; b = t; i = i+1 }
//     return a
// }
// fib(0)=0, fib(1)=1, fib(2)=1, fib(3)=2, fib(4)=3, fib(5)=5, fib(6)=8, fib(7)=13

func TestHIRFibonacci(t *testing.T) {
	n := hir.Var("n", mir2.TyU8)
	a := hir.Var("a", mir2.TyU8)
	b := hir.Var("b", mir2.TyU8)
	i := hir.Var("i", mir2.TyU8)

	hm := &hir.Module{Name: "test_fib"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:   "fib",
		Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body: hir.Blk(
			hir.Decl("a", mir2.TyU8, hir.U8(0)),
			hir.Decl("b", mir2.TyU8, hir.U8(1)),
			hir.Decl("i", mir2.TyU8, hir.U8(0)),
			hir.While(
				hir.Lt(i, n),
				hir.Blk(
					// t is body-local: not a loop-carried var, just a temp
					hir.Decl("t", mir2.TyU8, hir.Add(a, b, mir2.TyU8)),
					hir.Assign(hir.Var("a", mir2.TyU8), b),
					hir.Assign(hir.Var("b", mir2.TyU8), hir.Var("t", mir2.TyU8)),
					hir.Assign(hir.Var("i", mir2.TyU8), hir.Add(i, hir.U8(1), mir2.TyU8)),
				),
			),
			hir.Ret(a),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("generated:\n%s", genAsm)

	cases := []struct{ n, want uint8 }{
		{0, 0}, {1, 1}, {2, 1}, {3, 2}, {4, 3}, {5, 5}, {6, 8}, {7, 13},
	}
	for _, tc := range cases {
		got, _, err := runZ80(t, boot1("fib", int(tc.n))+genAsm)
		if err != nil {
			t.Fatalf("fib(%d): %v", tc.n, err)
		}
		if got != tc.want {
			t.Errorf("fib(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

// ── gcd(a, b u8) u8 — subtraction-based Euclidean, if/else inside while ──────
//
// fun gcd(a, b u8) u8 {
//     while a != b { if a > b { a = a-b } else { b = b-a } }
//     return a
// }
// Exercises: nested if-inside-while, two vars mutated in different branches.

func TestHIRGCD(t *testing.T) {
	a := hir.Var("a", mir2.TyU8)
	b := hir.Var("b", mir2.TyU8)

	hm := &hir.Module{Name: "test_gcd"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:   "gcd",
		Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body: hir.Blk(
			hir.While(
				hir.Ne(a, b),
				hir.Blk(
					hir.If(
						hir.Gt(a, b),
						hir.Blk(hir.Assign(hir.Var("a", mir2.TyU8), hir.Sub(a, b, mir2.TyU8))),
						hir.Blk(hir.Assign(hir.Var("b", mir2.TyU8), hir.Sub(b, a, mir2.TyU8))),
					),
				),
			),
			hir.Ret(a),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("generated:\n%s", genAsm)

	cases := []struct{ a, b, want uint8 }{
		{12, 8, 4}, {48, 18, 6}, {15, 5, 5}, {7, 7, 7}, {100, 75, 25},
	}
	for _, tc := range cases {
		got, _, err := runZ80(t, boot2("gcd", int(tc.a), int(tc.b))+genAsm)
		if err != nil {
			t.Fatalf("gcd(%d,%d): %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Errorf("gcd(%d,%d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// ── multi-function: double + quad — inter-function calls ──────────────────────
//
// fun double(x u8) u8 { return x + x }
// fun quad(x u8) u8   { return double(double(x)) }
// Tests that HIR→MIR2 correctly handles call instructions between functions
// in the same module, with correct ABI (arg in A, result in A).

func TestHIRMultiFunc(t *testing.T) {
	x := hir.Var("x", mir2.TyU8)

	hm := &hir.Module{Name: "test_multi"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:   "double",
		Params: []hir.Param{{Name: "x", Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body:   hir.Blk(hir.Ret(hir.Add(x, x, mir2.TyU8))),
	})
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:   "quad",
		Params: []hir.Param{{Name: "x", Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body: hir.Blk(
			hir.Ret(hir.Call("double", mir2.TyU8,
				hir.Call("double", mir2.TyU8, x))),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("generated:\n%s", genAsm)

	cases := []struct{ x, want uint8 }{
		{1, 4}, {3, 12}, {7, 28}, {10, 40}, {63, 252},
	}
	for _, tc := range cases {
		got, _, err := runZ80(t, boot1("quad", int(tc.x))+genAsm)
		if err != nil {
			t.Fatalf("quad(%d): %v", tc.x, err)
		}
		if got != tc.want {
			t.Errorf("quad(%d) = %d, want %d", tc.x, got, tc.want)
		}
	}
}

// ── if/else with var mutation ─────────────────────────────────────────────────

// HIR: fun abs_diff(a, b: u8) -> u8 { if a > b { return a - b }; return b - a }
func TestHIRAbsDiff(t *testing.T) {
	a := hir.Var("a", mir2.TyU8)
	b := hir.Var("b", mir2.TyU8)

	hm := &hir.Module{Name: "test_abs"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:   "abs_diff",
		Params: []hir.Param{{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body: hir.Blk(
			hir.If(hir.Gt(a, b), hir.Blk(hir.Ret(hir.Sub(a, b, mir2.TyU8))), nil),
			hir.Ret(hir.Sub(b, a, mir2.TyU8)),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("generated:\n%s", genAsm)

	cases := []struct{ a, b, want uint8 }{
		{5, 3, 2}, {3, 5, 2}, {4, 4, 0}, {0, 10, 10}, {10, 0, 10},
	}
	for _, tc := range cases {
		got, _, err := runZ80(t, boot2("abs_diff", int(tc.a), int(tc.b))+genAsm)
		if err != nil {
			t.Fatalf("abs_diff(%d,%d): %v", tc.a, tc.b, err)
		}
		if got != tc.want {
			t.Errorf("abs_diff(%d,%d) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}

// ── break: early_sum(n u8) u8 ─────────────────────────────────────────────────
//
// sum = 0, i = 0
// while i < n { sum += i; if sum > 5 { break }; i++ }
// return sum
// Expected: early_sum(10)=6, early_sum(3)=3, early_sum(2)=1

func TestHIRBreak(t *testing.T) {
	n := hir.Var("n", mir2.TyU8)
	sum := hir.Var("sum", mir2.TyU8)
	i := hir.Var("i", mir2.TyU8)

	hm := &hir.Module{Name: "test_break"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:   "early_sum",
		Params: []hir.Param{{Name: "n", Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body: hir.Blk(
			hir.Decl("sum", mir2.TyU8, hir.U8(0)),
			hir.Decl("i", mir2.TyU8, hir.U8(0)),
			hir.While(
				hir.Lt(i, n),
				hir.Blk(
					hir.Assign(hir.Var("sum", mir2.TyU8), hir.Add(sum, i, mir2.TyU8)),
					hir.If(
						hir.Gt(hir.Var("sum", mir2.TyU8), hir.U8(5)),
						hir.Blk(hir.Break()),
						nil,
					),
					hir.Assign(hir.Var("i", mir2.TyU8), hir.Add(i, hir.U8(1), mir2.TyU8)),
				),
			),
			hir.Ret(hir.Var("sum", mir2.TyU8)),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("early_sum generated:\n%s", genAsm)

	cases := []struct{ n, want uint8 }{
		{10, 6}, // 0+1+2+3=6 > 5, break
		{3, 3},  // 0+1+2=3, no break
		{2, 1},  // 0+1=1, no break
		{0, 0},  // empty loop
		{1, 0},  // only adds 0
	}
	for _, tc := range cases {
		got, _, err := runZ80(t, boot1("early_sum", int(tc.n))+genAsm)
		if err != nil {
			t.Fatalf("early_sum(%d): %v", tc.n, err)
		}
		if got != tc.want {
			t.Errorf("early_sum(%d) = %d, want %d", tc.n, got, tc.want)
		}
	}
}

// ── switch: classify(x u8) u8 ─────────────────────────────────────────────────
//
// switch x { case 0: return 10; case 1: return 20; case 2: return 30; default: return 0 }

func TestHIRSwitch(t *testing.T) {
	x := hir.Var("x", mir2.TyU8)

	hm := &hir.Module{Name: "test_switch"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:   "classify",
		Params: []hir.Param{{Name: "x", Ty: mir2.TyU8}},
		RetTy:  mir2.TyU8,
		Body: hir.Blk(
			hir.Switch(x, []*hir.SwitchCase{
				hir.Case(0, hir.Blk(hir.Ret(hir.U8(10)))),
				hir.Case(1, hir.Blk(hir.Ret(hir.U8(20)))),
				hir.Case(2, hir.Blk(hir.Ret(hir.U8(30)))),
			}, hir.Blk(hir.Ret(hir.U8(0)))),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("classify generated:\n%s", genAsm)

	cases := []struct{ x, want uint8 }{
		{0, 10}, {1, 20}, {2, 30}, {3, 0}, {255, 0},
	}
	for _, tc := range cases {
		got, _, err := runZ80(t, boot1("classify", int(tc.x))+genAsm)
		if err != nil {
			t.Fatalf("classify(%d): %v", tc.x, err)
		}
		if got != tc.want {
			t.Errorf("classify(%d) = %d, want %d", tc.x, got, tc.want)
		}
	}
}

// ── index: get_byte(arr ^u8, i u8) u8 ────────────────────────────────────────
//
// Return arr[i] using runtime array indexing (OpPtrAdd).
// Calling convention: arr → HL (ClassPointer), i → A (ClassAcc).

func TestHIRIndex(t *testing.T) {
	hm := &hir.Module{Name: "test_index"}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name: "get_byte",
		Params: []hir.Param{
			{Name: "arr", Ty: mir2.TyPtr},
			{Name: "i", Ty: mir2.TyU8},
		},
		RetTy: mir2.TyU8,
		Body: hir.Blk(
			hir.Ret(hir.Index(
				hir.Var("arr", mir2.TyPtr),
				hir.Var("i", mir2.TyU8),
				mir2.TyU8,
			)),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("get_byte generated:\n%s", genAsm)

	// Data array: {10, 20, 30, 40, 50} at label test_arr.
	// Boot: HL = test_arr, A = index.
	data := "test_arr:\n    DB 10, 20, 30, 40, 50\n"

	// arr→HL (ClassPointer), i→C (ClassGeneral, pos=1 after ptr param)
	bootHL := func(fn string, idx int) string {
		return fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n    LD HL, test_arr\n    LD C, %d\n    CALL %s\n    DI\n    HALT\n",
			testLoadAddr, idx, fn)
	}

	cases := []struct{ i int; want uint8 }{
		{0, 10}, {1, 20}, {2, 30}, {3, 40}, {4, 50},
	}
	for _, tc := range cases {
		got, _, err := runZ80(t, bootHL("get_byte", tc.i)+genAsm+data)
		if err != nil {
			t.Fatalf("get_byte(arr, %d): %v", tc.i, err)
		}
		if got != tc.want {
			t.Errorf("get_byte(arr, %d) = %d, want %d", tc.i, got, tc.want)
		}
	}
}

// TestLoadDirectToReturnUsesClassAcc verifies that lowerExprForRet eliminates
// the redundant LD C,(HL); LD A,C pattern for LoadExpr/FieldExpr at return sites.
// Without the fix, classForExpr returns ClassGeneral → LD C,(HL), then LD A,C.
// With the fix, classForRet returns ClassAcc → LD A,(HL) directly.
func TestLoadDirectToReturnUsesClassAcc(t *testing.T) {
	// palette: global u8 = 42
	// get_r() -> u8 { return *palette }   (LoadExpr at return site)
	hm := &hir.Module{Name: "test_ret_load"}
	hm.Globals = []mir2.Global{{Name: "palette", Ty: mir2.TyU8, Init: []byte{42}}}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:  "get_r",
		RetTy: mir2.TyU8,
		Body: hir.Blk(
			hir.Ret(&hir.LoadExpr{
				Ptr: hir.Addr("palette"),
				Ty:  mir2.TyU8,
			}),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("get_r generated:\n%s", genAsm)

	// Must NOT emit the redundant load-to-C-then-copy-to-A pattern.
	if strings.Contains(genAsm, "LD C, (") && strings.Contains(genAsm, "LD A, C") {
		t.Errorf("redundant 'LD C,(addr); LD A,C' pattern; lowerExprForRet should use ClassAcc:\n%s", genAsm)
	}
	// Must load directly into A.
	if !strings.Contains(genAsm, "LD A, (") {
		t.Errorf("expected direct 'LD A,(addr)' load in output:\n%s", genAsm)
	}

	// Run: get_r() should return 42 (the initial value of palette).
	boot := fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n    CALL get_r\n    DI\n    HALT\n", testLoadAddr)
	a, _, err := runZ80(t, boot+genAsm)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if a != 42 {
		t.Errorf("get_r() = %d, want 42", a)
	}
}

// TestFieldExprReturnUsesClassAcc verifies that returning a struct field (FieldExpr)
// at offset 0 also uses ClassAcc directly (no intermediate C register).
func TestFieldExprReturnUsesClassAcc(t *testing.T) {
	// struct Color { r: u8, g: u8, b: u8 }
	// global palette: Color = {7, 8, 9}
	// get_r() -> u8 { return palette.r }   (FieldExpr offset 0)
	colorTy := &mir2.StructTy{
		Name: "Color",
		Fields: []mir2.StructField{
			{Name: "r", Ty: mir2.TyU8},
			{Name: "g", Ty: mir2.TyU8},
			{Name: "b", Ty: mir2.TyU8},
		},
	}
	hm := &hir.Module{Name: "test_field_ret"}
	hm.Globals = []mir2.Global{{Name: "palette", Ty: colorTy, Init: []byte{7, 8, 9}}}
	hm.Funcs = append(hm.Funcs, &hir.Func{
		Name:  "get_r",
		RetTy: mir2.TyU8,
		Body: hir.Blk(
			hir.Ret(&hir.FieldExpr{
				X:      hir.Addr("palette"),
				Field:  "r",
				Offset: 0,
				Ty:     mir2.TyU8,
			}),
		),
	})

	genAsm := compileHIR(t, hm)
	t.Logf("get_r (field) generated:\n%s", genAsm)

	// Must NOT emit the redundant load-to-C-then-copy-to-A pattern.
	if strings.Contains(genAsm, "LD C, (") && strings.Contains(genAsm, "LD A, C") {
		t.Errorf("redundant 'LD C,(addr); LD A,C' for FieldExpr return; lowerExprForRet should use ClassAcc:\n%s", genAsm)
	}

	// Run: get_r() must return 7 (palette.r).
	boot := fmt.Sprintf("    ORG 0x%04X\n    LD SP, 0xFF00\n    CALL get_r\n    DI\n    HALT\n", testLoadAddr)
	a, _, err := runZ80(t, boot+genAsm)
	if err != nil {
		t.Fatalf("run: %v", err)
	}
	if a != 7 {
		t.Errorf("get_r() = %d, want 7 (palette.r)", a)
	}
}
