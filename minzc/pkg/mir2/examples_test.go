package mir2_test

// This file contains MIR2 builder examples beyond fibonacci, used to evaluate
// Z80 code quality from the pipeline:
//   BuildXxx → ReorderBlocks → Verify → ComputeLiveness → Allocate → Z80Codegen
//
// Each function is built with the Builder API, assembled with MZA, run through
// MZE (the Z80 emulator), and verified against expected results.
//
// Functions:
//   clamp(val, lo, hi: u8) → u8   — 3-arg, 2 conditional branches
//   abs_diff(a, b: u8) → u8       — absolute difference, move to save reg
//   sum_range(n: u8) → u16        — count-down sum, Ext + ADD HL,DE
//   mul8(a, b: u8) → u16          — repeated-addition multiply

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// ── clamp ─────────────────────────────────────────────────────────────────────
//
// fun clamp(val: u8, lo: u8, hi: u8) -> u8 {
//     if val < lo { return lo }
//     if val > hi { return hi }
//     return val
// }
//
// MIR2 block structure:
//   entry → below_lo? → above_hi? → done
//   below_lo: ret lo
//   above_hi: ret hi
//   done:     ret val
//
// Calling convention: val→A, lo→B, hi→C (all u8, ClassAcc/ClassCounter/ClassGeneral).
// Return: A = result.
func buildClamp(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("clamp")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	// entry
	b.SwitchToNewBlock("entry")
	val := b.Param("val", mir2.TyU8, mir2.ClassAcc)
	lo := b.Param("lo", mir2.TyU8, mir2.ClassCounter)
	hi := b.Param("hi", mir2.TyU8, mir2.ClassGeneral)

	// if val < lo → below_lo   (CmpLt → C flag, clean condition)
	cmpLo := b.Cmp(mir2.CmpLt, val, lo, mir2.ClassFlag, false)
	b.BrIf(cmpLo, "below_lo", nil, "check_hi", nil)

	// check_hi: if hi < val → above_hi   (use CmpLt with operands hi, val
	// so CC=C fires when hi < val, i.e., val > hi; avoids CmpGt's NC ambiguity)
	b.SwitchToNewBlock("check_hi")
	cmpHi := b.Cmp(mir2.CmpLt, hi, val, mir2.ClassFlag, false)
	b.BrIf(cmpHi, "above_hi", nil, "done", nil)

	// below_lo: return lo
	b.SwitchToNewBlock("below_lo")
	b.Ret(lo)

	// above_hi: return hi
	b.SwitchToNewBlock("above_hi")
	b.Ret(hi)

	// done: return val
	b.SwitchToNewBlock("done")
	b.Ret(val)

	return f
}

// ── abs_diff ──────────────────────────────────────────────────────────────────
//
// fun abs_diff(a: u8, b: u8) -> u8 {
//     if a >= b { return a - b }
//     return b - a
// }
func buildAbsDiff(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("abs_diff")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)

	// if a >= b → a_ge_b
	cmp := b.Cmp(mir2.CmpGe, a, bv, mir2.ClassFlag, false)
	b.BrIf(cmp, "a_ge_b", nil, "b_gt_a", nil)

	// a_ge_b: return a - b
	b.SwitchToNewBlock("a_ge_b")
	diff1 := b.Sub(a, bv, mir2.TyU8, mir2.ClassAcc)
	b.Ret(diff1)

	// b_gt_a: return b - a
	b.SwitchToNewBlock("b_gt_a")
	diff2 := b.Sub(bv, a, mir2.TyU8, mir2.ClassAcc)
	b.Ret(diff2)

	return f
}

// ── sum_range ─────────────────────────────────────────────────────────────────
//
// fun sum_range(n: u8) -> u16 {
//     // Returns 0 + 1 + ... + n  (= n*(n+1)/2)
//     var acc: u16 = 0
//     var i: u8 = n
//     while i > 0 {
//         acc = acc + (i as u16)
//         i--
//     }
//     return acc
// }
func buildSumRange(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("sum_range")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)

	// n arrives in A (ClassAcc per calling convention).
	// Move it to B (ClassCounter) for use as the DJNZ-style loop counter.
	b.SwitchToNewBlock("entry")
	n := b.Param("n", mir2.TyU8, mir2.ClassAcc)
	nB := b.Move(n, mir2.TyU8, mir2.ClassCounter) // n → B for loop use
	zero := b.Const(0, mir2.TyU16, mir2.ClassPointer)
	b.Jmp("loop_head", nB, zero)

	head := b.SwitchToNewBlock("loop_head")
	iP := b.BlockParam(head, mir2.TyU8, mir2.ClassCounter)
	accP := b.BlockParam(head, mir2.TyU16, mir2.ClassPointer)

	zero8 := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
	cond := b.Cmp(mir2.CmpNe, iP, zero8, mir2.ClassFlag, false) // i != 0 ≡ i > 0 (unsigned)
	b.BrIf(cond, "loop_body", []mir2.Reg{iP, accP}, "exit", []mir2.Reg{accP})

	body := b.SwitchToNewBlock("loop_body")
	iB := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)
	accB := b.BlockParam(body, mir2.TyU16, mir2.ClassPointer)

	i16 := b.Ext(iB, mir2.TyU8, mir2.TyU16, mir2.ClassIndex) // zero-extend i
	newAcc := b.Add(accB, i16, mir2.TyU16, mir2.ClassPointer)
	one8 := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	iNext := b.Sub(iB, one8, mir2.TyU8, mir2.ClassCounter) // i--
	b.Jmp("loop_head", iNext, newAcc)

	exit := b.SwitchToNewBlock("exit")
	result := b.BlockParam(exit, mir2.TyU16, mir2.ClassPointer)
	b.Ret(result)

	return f
}

// ── mul8 ──────────────────────────────────────────────────────────────────────
//
// fun mul8(a: u8, b: u8) -> u16 {
//     // Returns a * b by repeated addition.
//     var acc: u16 = 0
//     var i: u8 = b
//     while i > 0 {
//         acc = acc + (a as u16)
//         i--
//     }
//     return acc
// }
func buildMul8(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("mul8")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)

	// a arrives in A (ClassAcc), b arrives in B (ClassCounter).
	// We move a to a ClassGeneral reg (C/D/E) so the loop comparison
	// (LD A,B; CP 0) doesn't clobber the multiplier value in A.
	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	aC := b.Move(a, mir2.TyU8, mir2.ClassGeneral) // save a to C/D/E
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)
	zero := b.Const(0, mir2.TyU16, mir2.ClassPointer)
	b.Jmp("loop_head", bv, zero)

	head := b.SwitchToNewBlock("loop_head")
	iP := b.BlockParam(head, mir2.TyU8, mir2.ClassCounter)
	accP := b.BlockParam(head, mir2.TyU16, mir2.ClassPointer)

	zero8 := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
	cond := b.Cmp(mir2.CmpNe, iP, zero8, mir2.ClassFlag, false) // i != 0 ≡ i > 0 (unsigned)
	b.BrIf(cond, "loop_body", []mir2.Reg{iP, accP}, "exit", []mir2.Reg{accP})

	body := b.SwitchToNewBlock("loop_body")
	iB := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)
	accB := b.BlockParam(body, mir2.TyU16, mir2.ClassPointer)

	a16 := b.Ext(aC, mir2.TyU8, mir2.TyU16, mir2.ClassIndex) // zero-extend a (in C)
	newAcc := b.Add(accB, a16, mir2.TyU16, mir2.ClassPointer)
	one8 := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	iNext := b.Sub(iB, one8, mir2.TyU8, mir2.ClassCounter) // i--
	b.Jmp("loop_head", iNext, newAcc)

	exit := b.SwitchToNewBlock("exit")
	result := b.BlockParam(exit, mir2.TyU16, mir2.ClassPointer)
	b.Ret(result)

	return f
}

// ── min8 ──────────────────────────────────────────────────────────────────────
//
// fun min8(a: u8, b: u8) -> u8 { if a < b { a } else { b } }
//
// Clean baseline: one comparison, two exit paths.
// Calling convention: a→A, b→B.  Return: A.
func buildMin8(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("min8")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)

	// a < b: carry set after CP B
	clt := b.Cmp(mir2.CmpLt, a, bv, mir2.ClassFlag, false)
	b.BrIf(clt, "a_wins", nil, "b_wins", nil)

	b.SwitchToNewBlock("a_wins")
	b.Ret(a)

	b.SwitchToNewBlock("b_wins")
	bA := b.Move(bv, mir2.TyU8, mir2.ClassAcc) // LD A, B
	b.Ret(bA)

	return f
}

// ── gcd ───────────────────────────────────────────────────────────────────────
//
// fun gcd(a: u8, b: u8) -> u8 {
//     while a != b {
//         if a < b { b -= a } else { a -= b }
//     }
//     return a
// }
//
// Euclidean subtraction, terminates when a == b.
// Block params carry mutable (a, b) through the loop.
// Calling convention: a→A, b→B. Return: A.
func buildGCD(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("gcd")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)
	b.Jmp("loop", a, bv)

	// loop(aP: A, bP: B): test a < b, a == b, or a > b
	loop := b.SwitchToNewBlock("loop")
	aP := b.BlockParam(loop, mir2.TyU8, mir2.ClassAcc)
	bP := b.BlockParam(loop, mir2.TyU8, mir2.ClassCounter)

	// a < b → carry set (C)
	clt := b.Cmp(mir2.CmpLt, aP, bP, mir2.ClassFlag, false)
	b.BrIf(clt, "a_smaller", []mir2.Reg{aP, bP}, "ge", []mir2.Reg{aP, bP})

	// a_smaller: b = b - a.
	// Save aS to ClassGeneral (C) before Sub so A is free for LD A,B;SUB C.
	// The Jmp passes aSafe (in C) back to loop's ClassAcc param; the parallel-copy
	// resolver emits LD A,C.
	asmaller := b.SwitchToNewBlock("a_smaller")
	aS := b.BlockParam(asmaller, mir2.TyU8, mir2.ClassAcc)
	bS := b.BlockParam(asmaller, mir2.TyU8, mir2.ClassCounter)
	aSafe := b.Move(aS, mir2.TyU8, mir2.ClassGeneral)       // LD C, A  (save a)
	bNew := b.Sub(bS, aSafe, mir2.TyU8, mir2.ClassCounter)  // LD A,B; SUB C; LD B,A
	b.Jmp("loop", aSafe, bNew)                               // LD A,C (restore a)

	// ge: a >= b. Equal → done. Greater → a = a - b.
	ge := b.SwitchToNewBlock("ge")
	aG := b.BlockParam(ge, mir2.TyU8, mir2.ClassAcc)
	bG := b.BlockParam(ge, mir2.TyU8, mir2.ClassCounter)
	ceq := b.Cmp(mir2.CmpEq, aG, bG, mir2.ClassFlag, false)
	b.BrIf(ceq, "done", []mir2.Reg{aG}, "a_bigger", []mir2.Reg{aG, bG})

	// a_bigger: a = a - b
	abigger := b.SwitchToNewBlock("a_bigger")
	aB := b.BlockParam(abigger, mir2.TyU8, mir2.ClassAcc)
	bB := b.BlockParam(abigger, mir2.TyU8, mir2.ClassCounter)
	aNew := b.Sub(aB, bB, mir2.TyU8, mir2.ClassAcc) // A - B
	b.Jmp("loop", aNew, bB)

	done := b.SwitchToNewBlock("done")
	result := b.BlockParam(done, mir2.TyU8, mir2.ClassAcc)
	b.Ret(result)

	return f
}

// ── max3 ──────────────────────────────────────────────────────────────────────
//
// fun max3(a: u8, b: u8, c: u8) -> u8 {
//     let m = if a < b { b } else { a }
//     if m < c { c } else { m }
// }
//
// Two comparison stages; uses CmpLt (carry flag) throughout.
// Calling convention: a→A, b→B, c→C. Return: A.
func buildMax3(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("max3")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)
	c := b.Param("c", mir2.TyU8, mir2.ClassGeneral) // → C register

	// Stage 1: a < b?
	clt1 := b.Cmp(mir2.CmpLt, a, bv, mir2.ClassFlag, false) // CP B; C if a<b
	b.BrIf(clt1, "b_leads", nil, "a_leads", nil)

	// a_leads: a >= b; compare a with c
	b.SwitchToNewBlock("a_leads")
	clt2 := b.Cmp(mir2.CmpLt, a, c, mir2.ClassFlag, false) // CP C; C if a<c
	b.BrIf(clt2, "ret_c_from_a", nil, "ret_a", nil)

	// b_leads: b > a; compare b with c
	// Need b in A for return, so move here for the comparison path.
	b.SwitchToNewBlock("b_leads")
	bA := b.Move(bv, mir2.TyU8, mir2.ClassAcc) // LD A, B (b → A for CP C)
	clt3 := b.Cmp(mir2.CmpLt, bA, c, mir2.ClassFlag, false) // CP C; C if b<c
	b.BrIf(clt3, "ret_c_from_b", nil, "ret_b", nil)

	b.SwitchToNewBlock("ret_a")
	b.Ret(a)

	b.SwitchToNewBlock("ret_b")
	b.Ret(bA)

	b.SwitchToNewBlock("ret_c_from_a")
	b.Ret(c)

	b.SwitchToNewBlock("ret_c_from_b")
	b.Ret(c)

	return f
}

// ── popcount ──────────────────────────────────────────────────────────────────
//
// fun popcount(x: u8) -> u8 {
//     var count: u8 = 0
//     while x != 0 { count += x & 1; x >>= 1 }
//     return count
// }
//
// Shift-and-count, all 8-bit.
// x in ClassGeneral (C) to keep A free for comparisons; count in ClassCounter (B).
// Calling convention: x→A. Return: A.
func buildPopcount(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("popcount")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	xC := b.Move(x, mir2.TyU8, mir2.ClassGeneral)          // x → C (preserve A)
	cnt0 := b.Const(0, mir2.TyU8, mir2.ClassCounter)       // count = 0 → B
	b.Jmp("loop_head", xC, cnt0)

	head := b.SwitchToNewBlock("loop_head")
	xP := b.BlockParam(head, mir2.TyU8, mir2.ClassGeneral) // x in C
	cntP := b.BlockParam(head, mir2.TyU8, mir2.ClassCounter) // count in B

	zero := b.Const(0, mir2.TyU8, mir2.ClassAcc)           // DSE'd; enables CP 0 peephole
	cne := b.Cmp(mir2.CmpNe, xP, zero, mir2.ClassFlag, false)
	b.BrIf(cne, "body", []mir2.Reg{xP, cntP}, "done", []mir2.Reg{cntP})

	body := b.SwitchToNewBlock("body")
	xB := b.BlockParam(body, mir2.TyU8, mir2.ClassGeneral)
	cntB := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)

	one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)          // AND 1 — kept (actual operand)
	bit := b.And(xB, one, mir2.TyU8, mir2.ClassGeneral)      // A=C AND D; → E (bit)
	newCnt := b.Add(cntB, bit, mir2.TyU8, mir2.ClassCounter) // B = B + E

	oneS := b.Const(1, mir2.TyU8, mir2.ClassGeneral)         // shift count — DSE'd (genShift ignores it)
	xNew := b.Shr(xB, oneS, mir2.TyU8, mir2.ClassGeneral)   // SRL C
	b.Jmp("loop_head", xNew, newCnt)

	done := b.SwitchToNewBlock("done")
	cntD := b.BlockParam(done, mir2.TyU8, mir2.ClassCounter)
	resultA := b.Move(cntD, mir2.TyU8, mir2.ClassAcc) // LD A, B
	b.Ret(resultA)

	return f
}

// ── Assembly + emulator helpers ───────────────────────────────────────────────

// compileFunc runs the full pipeline on a single-function module and returns
// the generated Z80 assembly string.
func compileFunc(t *testing.T, m *mir2.Module) string {
	t.Helper()
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}
	var asm string
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		asm = mir2.Z80Codegen(m, ar)
	}
	return asm
}

const testLoadAddr = 0x8000

// runZ80 assembles src (bootstrap + function body) and runs it; returns A, HL,
// and any error.  The caller writes the bootstrap to set up args and do
// CALL funcName / DI / HALT.
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

func bootstrap1(funcName string, arg0 int) string {
	return fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD A, %d
    CALL %s
    DI
    HALT
`, testLoadAddr, arg0, funcName)
}

func bootstrap2(funcName string, arg0, arg1 int) string {
	return fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD A, %d
    LD B, %d
    CALL %s
    DI
    HALT
`, testLoadAddr, arg0, arg1, funcName)
}

func bootstrap3(funcName string, arg0, arg1, arg2 int) string {
	return fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD A, %d
    LD B, %d
    LD C, %d
    CALL %s
    DI
    HALT
`, testLoadAddr, arg0, arg1, arg2, funcName)
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestClampZ80(t *testing.T) {
	m := &mir2.Module{Name: "clamp"}
	buildClamp(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct{ val, lo, hi, want int }{
		{5, 3, 10, 5},  // in range
		{1, 3, 10, 3},  // below lo → lo
		{15, 3, 10, 10}, // above hi → hi
		{3, 3, 10, 3},  // at lo boundary
		{10, 3, 10, 10}, // at hi boundary
	}
	for _, tc := range cases {
		src := bootstrap3("clamp", tc.val, tc.lo, tc.hi) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("clamp(%d,%d,%d): %v", tc.val, tc.lo, tc.hi, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("clamp(%d,%d,%d) = %d, want %d", tc.val, tc.lo, tc.hi, int(a), tc.want)
		} else {
			t.Logf("clamp(%d,%d,%d) = %d ✓", tc.val, tc.lo, tc.hi, int(a))
		}
	}
}

func TestAbsDiffZ80(t *testing.T) {
	m := &mir2.Module{Name: "abs_diff"}
	buildAbsDiff(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct{ a, b, want int }{
		{10, 3, 7},
		{3, 10, 7},
		{5, 5, 0},
		{0, 200, 200},
		{255, 0, 255},
	}
	for _, tc := range cases {
		// abs_diff: a→A, b→B
		src := bootstrap2("abs_diff", tc.a, tc.b) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("abs_diff(%d,%d): %v", tc.a, tc.b, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("abs_diff(%d,%d) = %d, want %d", tc.a, tc.b, int(a), tc.want)
		} else {
			t.Logf("abs_diff(%d,%d) = %d ✓", tc.a, tc.b, int(a))
		}
	}
}

func TestSumRangeZ80(t *testing.T) {
	m := &mir2.Module{Name: "sum_range"}
	buildSumRange(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	// sum_range(n) = n*(n+1)/2
	cases := []struct{ n, want int }{
		{0, 0},
		{1, 1},
		{5, 15},
		{10, 55},
		{20, 210},
	}
	for _, tc := range cases {
		src := bootstrap1("sum_range", tc.n) + "\n" + asm
		_, hl, err := runZ80(t, src)
		if err != nil {
			t.Errorf("sum_range(%d): %v", tc.n, err)
			continue
		}
		if int(hl) != tc.want {
			t.Errorf("sum_range(%d) = %d, want %d", tc.n, int(hl), tc.want)
		} else {
			t.Logf("sum_range(%d) = %d ✓", tc.n, int(hl))
		}
	}
}

func TestMul8Z80(t *testing.T) {
	m := &mir2.Module{Name: "mul8"}
	buildMul8(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	// mul8(a, b) = a*b via repeated addition; result in HL
	// a→A, b→B
	cases := []struct{ a, b, want int }{
		{0, 5, 0},
		{5, 0, 0},
		{3, 4, 12},
		{7, 8, 56},
		{12, 12, 144},
		{25, 10, 250},
	}
	for _, tc := range cases {
		src := bootstrap2("mul8", tc.a, tc.b) + "\n" + asm
		_, hl, err := runZ80(t, src)
		if err != nil {
			t.Errorf("mul8(%d,%d): %v", tc.a, tc.b, err)
			continue
		}
		if int(hl) != tc.want {
			t.Errorf("mul8(%d,%d) = %d, want %d", tc.a, tc.b, int(hl), tc.want)
		} else {
			t.Logf("mul8(%d,%d) = %d ✓", tc.a, tc.b, int(hl))
		}
	}
}

func TestMin8Z80(t *testing.T) {
	m := &mir2.Module{Name: "min8"}
	buildMin8(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct{ a, b, want int }{
		{3, 7, 3},
		{7, 3, 3},
		{5, 5, 5},
		{0, 255, 0},
		{255, 0, 0},
		{100, 101, 100},
	}
	for _, tc := range cases {
		src := bootstrap2("min8", tc.a, tc.b) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("min8(%d,%d): %v", tc.a, tc.b, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("min8(%d,%d) = %d, want %d", tc.a, tc.b, int(a), tc.want)
		} else {
			t.Logf("min8(%d,%d) = %d ✓", tc.a, tc.b, int(a))
		}
	}
}

func TestGCDZ80(t *testing.T) {
	m := &mir2.Module{Name: "gcd"}
	buildGCD(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct{ a, b, want int }{
		{12, 8, 4},
		{7, 13, 1},
		{48, 18, 6},
		{100, 75, 25},
		{1, 1, 1},
		{15, 5, 5},
		{5, 15, 5},
	}
	for _, tc := range cases {
		src := bootstrap2("gcd", tc.a, tc.b) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("gcd(%d,%d): %v", tc.a, tc.b, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("gcd(%d,%d) = %d, want %d", tc.a, tc.b, int(a), tc.want)
		} else {
			t.Logf("gcd(%d,%d) = %d ✓", tc.a, tc.b, int(a))
		}
	}
}

func TestMax3Z80(t *testing.T) {
	m := &mir2.Module{Name: "max3"}
	buildMax3(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct{ a, b, c, want int }{
		{1, 2, 3, 3},
		{3, 2, 1, 3},
		{2, 3, 1, 3},
		{1, 3, 2, 3},
		{5, 5, 5, 5},
		{0, 128, 200, 200},
		{255, 0, 128, 255},
	}
	for _, tc := range cases {
		src := bootstrap3("max3", tc.a, tc.b, tc.c) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("max3(%d,%d,%d): %v", tc.a, tc.b, tc.c, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("max3(%d,%d,%d) = %d, want %d", tc.a, tc.b, tc.c, int(a), tc.want)
		} else {
			t.Logf("max3(%d,%d,%d) = %d ✓", tc.a, tc.b, tc.c, int(a))
		}
	}
}

func TestPopcountZ80(t *testing.T) {
	m := &mir2.Module{Name: "popcount"}
	buildPopcount(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct{ x, want int }{
		{0, 0},
		{1, 1},
		{255, 8},
		{0b10101010, 4},
		{0b11110000, 4},
		{0b00001111, 4},
		{7, 3},
		{128, 1},
	}
	for _, tc := range cases {
		src := bootstrap1("popcount", tc.x) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("popcount(%d): %v", tc.x, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("popcount(0b%08b) = %d, want %d", tc.x, int(a), tc.want)
		} else {
			t.Logf("popcount(0b%08b) = %d ✓", tc.x, int(a))
		}
	}
}
