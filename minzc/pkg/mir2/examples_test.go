package mir2_test

// This file contains MIR2 builder examples beyond fibonacci, used to evaluate
// Z80 code quality from the pipeline:
//   BuildXxx → ReorderBlocks → Verify → ComputeLiveness → Allocate → Z80Codegen
//
// Each function is built with the Builder API, assembled with MZA, run through
// MZE (the Z80 emulator), and verified against expected results.
//
// Functions:
//   clamp(val, lo, hi: u8) → u8       — 3-arg, 2 conditional branches
//   clamp_u16(val, lo, hi: u16) → u16 — same logic, 16-bit
//   abs_diff(a, b: u8) → u8           — absolute difference, u8
//   abs_diff_u16(a, b: u16) → u16     — 16-bit absolute difference
//   abs_diff_u32(a, b: u32) → u32     — 32-bit via EXX shadow pair
//   sum_range(n: u8) → u16            — count-down sum, Ext + ADD HL,DE
//   mul8(a, b: u8) → u16              — repeated-addition multiply
//   mul_u16(a, b: u16) → u16          — 16-bit multiply

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
//
// Uses TermBrIf2 so one CP sets both Z (eq) and C (lt) flags —
// eliminating the redundant second CP B that the old two-branch version emitted.
func buildGCD(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("gcd")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)
	b.Jmp("loop", a, bv)

	// loop(aP: A, bP: B): one CP → three-way branch (eq/lt/gt).
	loop := b.SwitchToNewBlock("loop")
	aP := b.BlockParam(loop, mir2.TyU8, mir2.ClassAcc)
	bP := b.BlockParam(loop, mir2.TyU8, mir2.ClassCounter)

	// TermBrIf2: CP B sets Z (a==b) and C (a<b) in one instruction.
	b.BrIf2(aP, bP,
		"done", []mir2.Reg{aP},       // Z set: a == b → return a
		"a_smaller", []mir2.Reg{aP, bP}, // C set: a <  b → b -= a
		"a_bigger", []mir2.Reg{aP, bP},  // neither: a > b → a -= b
	)

	// a_smaller: b = b - a.
	// Save aS to ClassGeneral (C) before Sub so A is free for LD A,B;SUB C.
	// The Jmp passes aSafe (in C) back to loop's ClassAcc param; the parallel-copy
	// resolver emits LD A,C.
	asmaller := b.SwitchToNewBlock("a_smaller")
	aS := b.BlockParam(asmaller, mir2.TyU8, mir2.ClassAcc)
	bS := b.BlockParam(asmaller, mir2.TyU8, mir2.ClassCounter)
	aSafe := b.Move(aS, mir2.TyU8, mir2.ClassGeneral)      // LD C, A  (save a)
	bNew := b.Sub(bS, aSafe, mir2.TyU8, mir2.ClassCounter) // LD A,B; SUB C; LD B,A
	b.Jmp("loop", aSafe, bNew)                             // LD A,C (restore a)

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
	if len(m.Funcs) != 1 {
		t.Fatalf("compileFunc: module has %d functions, want 1; use compileModule", len(m.Funcs))
	}
	return compileModule(t, m)
}

// compileModule runs the full pipeline on a module with any number of functions
// and returns the generated Z80 assembly.  All functions share a combined
// AllocResult so cross-function calls resolve correctly.
func compileModule(t *testing.T, m *mir2.Module) string {
	t.Helper()
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}
	// Allocate each function, then merge all AllocResults into one map
	// (virtual regs are unique across functions) and codegen the module once.
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

// compileOptModule runs the full optimization pass (CondRetSink + fusionSubCmpInBlock)
// before allocation.  Used for tests that require CondRet / IAR-pattern optimization.
func compileOptModule(t *testing.T, m *mir2.Module) string {
	t.Helper()
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}
	for _, f := range m.Funcs {
		for {
			changed := mir2.CondRetSink(f)
			if changed {
				mir2.EliminateDeadBlocks(f)
			}
			if !changed {
				break
			}
		}
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

func TestAbsDiffIARZ80(t *testing.T) {
	m := &mir2.Module{Name: "abs_diff_iar"}
	buildAbsDiffIAR(m)
	asm := compileOptModule(t, m)
	t.Log("\n" + asm)

	// Ideal: SUB B / RET NC / NEG / RET  (CondRetSink + Sub+CmpGe flag fusion)
	hasSubB := strings.Contains(asm, "SUB B")
	hasRetNC := strings.Contains(asm, "RET NC")
	hasNEG := strings.Contains(asm, "NEG")
	if hasSubB && hasRetNC && hasNEG {
		t.Log("✓ IAR-ideal codegen: SUB B / RET NC / NEG / RET")
	} else {
		t.Logf("note: not yet IAR-ideal (SUB B=%v RET NC=%v NEG=%v) — acceptable", hasSubB, hasRetNC, hasNEG)
	}

	cases := []struct{ a, b, want int }{
		{10, 3, 7},
		{3, 10, 7},
		{5, 5, 0},
		{0, 200, 200},
		{255, 0, 255},
	}
	for _, tc := range cases {
		src := bootstrap2("abs_diff_iar", tc.a, tc.b) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("abs_diff_iar(%d,%d): %v", tc.a, tc.b, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("abs_diff_iar(%d,%d) = %d, want %d", tc.a, tc.b, int(a), tc.want)
		} else {
			t.Logf("abs_diff_iar(%d,%d) = %d ✓", tc.a, tc.b, int(a))
		}
	}
}

func TestAbsDiffIARU16Z80(t *testing.T) {
	m := &mir2.Module{Name: "abs_diff_iar_u16"}
	buildAbsDiffIARU16(m)
	asm := compileOptModule(t, m)
	t.Log("\n" + asm)

	// Ideal: AND A / SBC HL,DE / RET NC / (NEG HL via SBC A,A) / RET
	hasAndA := strings.Contains(asm, "AND A") || strings.Contains(asm, "OR A")
	hasSBCHL := strings.Contains(asm, "SBC HL, DE")
	hasRetNC := strings.Contains(asm, "RET NC")
	hasSBCAA := strings.Contains(asm, "SBC A, A")
	if hasAndA && hasSBCHL && hasRetNC && hasSBCAA {
		t.Log("✓ IAR-ideal u16 codegen: AND A / SBC HL,DE / RET NC / SBC A,A / RET")
	} else {
		t.Logf("note: not yet IAR-ideal u16 (AND/OR A=%v SBC HL,DE=%v RET NC=%v SBC A,A=%v)",
			hasAndA, hasSBCHL, hasRetNC, hasSBCAA)
	}

	cases := []struct{ a, b, want int }{
		{1000, 300, 700},
		{300, 1000, 700},
		{500, 500, 0},
		{0, 65535, 65535},
		{65535, 0, 65535},
		{1, 2, 1},
	}
	for _, tc := range cases {
		src := bootstrapHL2DE("abs_diff_iar_u16", tc.a, tc.b) + "\n" + asm
		_, hl, err := runZ80(t, src)
		if err != nil {
			t.Errorf("abs_diff_iar_u16(%d,%d): %v", tc.a, tc.b, err)
			continue
		}
		if int(hl) != tc.want {
			t.Errorf("abs_diff_iar_u16(%d,%d) = %d, want %d", tc.a, tc.b, int(hl), tc.want)
		} else {
			t.Logf("abs_diff_iar_u16(%d,%d) = %d ✓", tc.a, tc.b, int(hl))
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

// ── rol8 ──────────────────────────────────────────────────────────────────────
//
// fun rol8(x: u8) -> u8 { (x << 1) | (x >> 7) }
//
// Rotate left by 1.  Expressed as two shifts + OR so the genShift constant-count
// path is exercised.  On Z80 this would ideally be RLCA (1 instruction), but
// Phase 2 is about correctness, not yet optimality.
//
// Calling convention: x→A. Return: A.
func buildRol8(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("rol8")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)

	// x << 1 in a general register so A is free for x >> 7
	one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	xShl := b.Shl(x, one, mir2.TyU8, mir2.ClassGeneral) // LD C,A; SLA C

	// x >> 7 in another general register
	seven := b.Const(7, mir2.TyU8, mir2.ClassGeneral)
	xShr := b.Shr(x, seven, mir2.TyU8, mir2.ClassGeneral) // LD D,A; SRL D ×7

	// (x << 1) | (x >> 7)
	result := b.Or(xShl, xShr, mir2.TyU8, mir2.ClassAcc) // LD A,C; OR D
	b.Ret(result)

	return f
}

// ── is_pow2 ───────────────────────────────────────────────────────────────────
//
// fun is_pow2(x: u8) -> u8 {  // returns 1 if power of 2, 0 otherwise
//     if x == 0 { return 0 }
//     return (x & (x - 1)) == 0 ? 1 : 0
// }
//
// The classic bit-trick: a power of 2 has exactly one set bit, so x & (x-1) == 0.
// Exercises: CP 0 peephole, AND, SUB, two-branch structure, constant returns.
//
// Calling convention: x→A. Return: A (0 or 1).
func buildIsPow2(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("is_pow2")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	zero := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
	// if x == 0 → not a power of 2 (CP 0 peephole fires)
	czero := b.Cmp(mir2.CmpEq, x, zero, mir2.ClassFlag, false)
	b.BrIf(czero, "return_false", nil, "check", nil)

	// check: compute x & (x-1), test if zero.
	// Save x to ClassGeneral (C) before Sub so A is free for the subtraction.
	// Then AND C computes (x-1) & x directly in one instruction.
	b.SwitchToNewBlock("check")
	xSave := b.Move(x, mir2.TyU8, mir2.ClassGeneral)   // LD C, A  (save x)
	one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	x1 := b.Sub(x, one, mir2.TyU8, mir2.ClassAcc)      // SUB 1  → A = x-1
	masked := b.And(x1, xSave, mir2.TyU8, mir2.ClassAcc) // AND C → A = (x-1) & x
	zero2 := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
	cmask := b.Cmp(mir2.CmpEq, masked, zero2, mir2.ClassFlag, false)
	b.BrIf(cmask, "return_true", nil, "return_false", nil)

	b.SwitchToNewBlock("return_true")
	t1 := b.Const(1, mir2.TyU8, mir2.ClassAcc)
	b.Ret(t1)

	b.SwitchToNewBlock("return_false")
	f0 := b.Const(0, mir2.TyU8, mir2.ClassAcc)
	b.Ret(f0)

	return f
}

// ── swap_nibbles ──────────────────────────────────────────────────────────────
//
// fun swap_nibbles(x: u8) -> u8 { (x << 4) | (x >> 4) }
//
// Swaps the high and low 4-bit nibbles.  Same shift+OR pattern as rol8 but
// with count 4, which exercises genShift emitting 4 copies of SLA/SRL.
// On Z80 this could be done with 4× RLCA (4T each = 16T total).
//
// Calling convention: x→A. Return: A.
func buildSwapNibbles(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("swap_nibbles")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)

	four := b.Const(4, mir2.TyU8, mir2.ClassGeneral)
	xShl := b.Shl(x, four, mir2.TyU8, mir2.ClassGeneral) // LD C,A; SLA C ×4
	xShr := b.Shr(x, four, mir2.TyU8, mir2.ClassGeneral) // LD D,A; SRL D ×4
	result := b.Or(xShl, xShr, mir2.TyU8, mir2.ClassAcc) // LD A,C; OR D
	b.Ret(result)

	return f
}

// ── global_counter ────────────────────────────────────────────────────────────
//
// global counter: u8 = 0
//
// fun add_to_counter(n: u8) -> u8 {
//     counter += n
//     return counter
// }
//
// First Phase 3 example: global variable read, ALU, store.
// Tests: OpAddrOf → LD HL,label, OpLoad → LD A,(HL), OpStore → LD (HL),A.
// Also exercises the global data emission (DB label) in Z80Codegen.
//
// Calling convention: n→A. Return: A.
func buildGlobalCounter(m *mir2.Module) *mir2.Func {
	m.AddGlobal(mir2.Global{
		Name: "counter",
		Ty:   mir2.TyU8,
		Init: []byte{0},
	})

	f := m.AddFunc("add_to_counter")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	n := b.Param("n", mir2.TyU8, mir2.ClassAcc)

	nSave := b.Move(n, mir2.TyU8, mir2.ClassGeneral) // LD C, A — save n; HL needed for ptr
	ptr := b.AddrOf("counter", mir2.ClassPointer)    // LD HL, counter
	old := b.Load(ptr, mir2.TyU8, mir2.ClassAcc)     // LD A, (HL)
	sum := b.Add(old, nSave, mir2.TyU8, mir2.ClassAcc) // ADD A, C
	b.Store(ptr, sum, mir2.TyU8)                       // LD (HL), A
	b.Ret(sum)

	return f
}

// ── struct_point ──────────────────────────────────────────────────────────────
//
// struct Point { x: u8, y: u8 }
// global pt: Point = {3, 4}
//
// fun sum_xy() -> u8 { pt.x + pt.y }
// fun dist2() -> u8 { pt.x * pt.x + pt.y * pt.y }  // simplified: squares via mul8-style
//
// First Phase 3b example: global struct + OpField for field access.
// Tests: AddrOf → LD HL,pt, Load field 0 (offset 0), Field(+1) → INC HL, Load field 1.
//
// sum_xy: no args. Return: A.
func buildStructPoint(m *mir2.Module) *mir2.Func {
	pt := &mir2.StructTy{
		Name: "Point",
		Fields: []mir2.StructField{
			{Name: "x", Ty: mir2.TyU8},
			{Name: "y", Ty: mir2.TyU8},
		},
	}

	// Global: Point{x:3, y:4} — packed, little-endian by field order.
	m.AddGlobal(mir2.Global{
		Name: "pt",
		Ty:   pt,
		Init: []byte{3, 4}, // x=3, y=4
	})

	f := m.AddFunc("sum_xy")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	base := b.AddrOf("pt", mir2.ClassPointer)                     // LD HL, pt
	px := b.Load(base, mir2.TyU8, mir2.ClassAcc)                  // LD A, (HL)  — pt.x
	yPtr := b.FieldOf(base, pt, 1, mir2.ClassPointer)             // INC HL      — &pt.y
	py := b.Load(yPtr, mir2.TyU8, mir2.ClassGeneral)              // LD C, (HL)  — pt.y
	result := b.Add(px, py, mir2.TyU8, mir2.ClassAcc)             // ADD A, C
	b.Ret(result)

	return f
}

func TestGlobalCounterZ80(t *testing.T) {
	m := &mir2.Module{Name: "global_counter"}
	buildGlobalCounter(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	// Single-call test: each fresh emulator starts with counter=0.
	// add_to_counter(n) → should return n (0+n).
	for _, tc := range []struct{ n, want int }{
		{5, 5}, {3, 3}, {42, 42}, {255, 255}, {0, 0},
	} {
		src := bootstrap1("add_to_counter", tc.n) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("add_to_counter(%d): %v", tc.n, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("add_to_counter(%d) = %d, want %d", tc.n, int(a), tc.want)
		} else {
			t.Logf("add_to_counter(%d) = %d ✓", tc.n, int(a))
		}
	}

	// Multi-call test: three calls in one run to verify global persists across calls.
	// add_to_counter(5), add_to_counter(3), add_to_counter(10) → final A = 18.
	multiSrc := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD A, 5
    CALL add_to_counter
    LD A, 3
    CALL add_to_counter
    LD A, 10
    CALL add_to_counter
    DI
    HALT
`, testLoadAddr) + "\n" + asm
	a, _, err := runZ80(t, multiSrc)
	if err != nil {
		t.Fatalf("multi-call: %v", err)
	}
	if int(a) != 18 {
		t.Errorf("multi-call result = %d, want 18", int(a))
	} else {
		t.Logf("multi-call: 5+3+10 = %d ✓ (global persists across calls)", int(a))
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

func TestRol8Z80(t *testing.T) {
	m := &mir2.Module{Name: "rol8"}
	buildRol8(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct{ x, want int }{
		{0b00000001, 0b00000010}, // bit 0 → bit 1
		{0b10000000, 0b00000001}, // bit 7 wraps to bit 0
		{0b10110101, 0b01101011}, // arbitrary
		{0b00000000, 0b00000000},
		{0b11111111, 0b11111111},
		{0b01010101, 0b10101010},
		{0b10101010, 0b01010101},
	}
	for _, tc := range cases {
		src := bootstrap1("rol8", tc.x) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("rol8(0b%08b): %v", tc.x, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("rol8(0b%08b) = 0b%08b, want 0b%08b", tc.x, int(a), tc.want)
		} else {
			t.Logf("rol8(0b%08b) = 0b%08b ✓", tc.x, int(a))
		}
	}
}

func TestIsPow2Z80(t *testing.T) {
	m := &mir2.Module{Name: "is_pow2"}
	buildIsPow2(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct{ x, want int }{
		{0, 0},   // 0 is not a power of 2
		{1, 1},   // 2^0
		{2, 1},   // 2^1
		{4, 1},   // 2^2
		{8, 1},   // 2^3
		{16, 1},  // 2^4
		{32, 1},  // 2^5
		{64, 1},  // 2^6
		{128, 1}, // 2^7
		{3, 0},   // 11 in binary
		{5, 0},
		{6, 0},
		{7, 0},
		{255, 0},
		{100, 0},
	}
	for _, tc := range cases {
		src := bootstrap1("is_pow2", tc.x) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("is_pow2(%d): %v", tc.x, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("is_pow2(%d) = %d, want %d", tc.x, int(a), tc.want)
		} else {
			t.Logf("is_pow2(%d) = %d ✓", tc.x, int(a))
		}
	}
}

func TestStructPointZ80(t *testing.T) {
	m := &mir2.Module{Name: "struct_point"}
	buildStructPoint(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	// sum_xy() takes no args, returns pt.x + pt.y = 3 + 4 = 7.
	// Bootstrap: just CALL sum_xy / DI / HALT (no args to set up).
	src := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    CALL sum_xy
    DI
    HALT
`, testLoadAddr) + "\n" + asm
	a, _, err := runZ80(t, src)
	if err != nil {
		t.Fatalf("sum_xy: %v", err)
	}
	if int(a) != 7 {
		t.Errorf("sum_xy() = %d, want 7 (3+4)", int(a))
	} else {
		t.Logf("sum_xy() = %d ✓  (pt.x=3 + pt.y=4)", int(a))
	}

	// Also test with different init values by running the raw MIR2 VM.
	// The VM resolves the global symbol to a heap slot and operates on it.
	vm := mir2.NewVM(m)
	res, vmErr := vm.Call("sum_xy", nil)
	if vmErr != nil {
		t.Fatalf("VM sum_xy: %v", vmErr)
	}
	if len(res) != 1 || res[0].I != 7 {
		t.Errorf("VM sum_xy() = %v, want 7", res)
	} else {
		t.Logf("VM sum_xy() = %d ✓", res[0].I)
	}
}

func TestSwapNibblesZ80(t *testing.T) {
	m := &mir2.Module{Name: "swap_nibbles"}
	buildSwapNibbles(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct{ x, want int }{
		{0x00, 0x00},
		{0xFF, 0xFF},
		{0xAB, 0xBA},
		{0x12, 0x21},
		{0xF0, 0x0F},
		{0x0F, 0xF0},
		{0x37, 0x73},
		{0xA0, 0x0A},
	}
	for _, tc := range cases {
		src := bootstrap1("swap_nibbles", tc.x) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("swap_nibbles(0x%02X): %v", tc.x, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("swap_nibbles(0x%02X) = 0x%02X, want 0x%02X", tc.x, int(a), tc.want)
		} else {
			t.Logf("swap_nibbles(0x%02X) = 0x%02X ✓", tc.x, int(a))
		}
	}
}

// ── sum_array ─────────────────────────────────────────────────────────────────
//
// fun sum_array(base: *u8, count: u8) -> u8 {
//
//	// Modular (u8) sum of count bytes starting at base.
//	var acc: u8 = 0
//	while count > 0 { acc += *base; base++; count-- }
//	return acc
//
// }
//
// Phase 3c: first OpPtrBump example.
// Register layout through the loop: base→HL, count→B, acc→C (ClassGeneral).
// Using ClassGeneral for acc keeps it in C so it survives the CP comparison.
//
// Expected Z80 per iteration: LD A,(HL) / ADD A,C / LD C,A / INC HL / DEC B
func buildSumArray(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("sum_array")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	// entry: initial accumulator = 0; will be passed via trampoline to ClassGeneral.
	b.SwitchToNewBlock("entry")
	base := b.Param("base", mir2.TyPtr, mir2.ClassPointer)   // HL
	count := b.Param("count", mir2.TyU8, mir2.ClassCounter)  // B
	acc0 := b.Const(0, mir2.TyU8, mir2.ClassAcc)             // A (trampoline: LD C, A)
	b.Jmp("loop_head", base, count, acc0)

	// loop_head(ptr, cnt, acc[general]) — acc stays in C, not A, so CP doesn't destroy it.
	head := b.SwitchToNewBlock("loop_head")
	ptrP := b.BlockParam(head, mir2.TyPtr, mir2.ClassPointer)  // HL
	cntP := b.BlockParam(head, mir2.TyU8, mir2.ClassCounter)   // B
	accP := b.BlockParam(head, mir2.TyU8, mir2.ClassGeneral)   // C

	zero8 := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
	cond := b.Cmp(mir2.CmpNe, cntP, zero8, mir2.ClassFlag, false)
	b.BrIf(cond, "loop_body", []mir2.Reg{ptrP, cntP, accP}, "exit", []mir2.Reg{accP})

	// loop_body: elem into A (LD A,(HL)), add to acc (ADD A,C), bump ptr, dec count.
	body := b.SwitchToNewBlock("loop_body")
	ptrB := b.BlockParam(body, mir2.TyPtr, mir2.ClassPointer)  // HL
	cntB := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)   // B
	accB := b.BlockParam(body, mir2.TyU8, mir2.ClassGeneral)   // C

	elem := b.Load(ptrB, mir2.TyU8, mir2.ClassAcc)                          // LD A, (HL)
	sum := b.Add(elem, accB, mir2.TyU8, mir2.ClassAcc)                      // ADD A, C
	accNew := b.Move(sum, mir2.TyU8, mir2.ClassGeneral)                     // LD C, A
	ptr2 := b.PtrBump(ptrB, 1, mir2.ClassPointer)                           // INC HL
	one8 := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	cnt2 := b.Sub(cntB, one8, mir2.TyU8, mir2.ClassCounter)                 // DEC B
	b.Jmp("loop_head", ptr2, cnt2, accNew)

	// exit: result is in ClassGeneral (C); move to A before return.
	exit := b.SwitchToNewBlock("exit")
	resultC := b.BlockParam(exit, mir2.TyU8, mir2.ClassGeneral)
	resultA := b.Move(resultC, mir2.TyU8, mir2.ClassAcc)                    // LD A, C
	b.Ret(resultA)

	return f
}

func TestSumArrayZ80(t *testing.T) {
	m := &mir2.Module{Name: "sum_array"}
	buildSumArray(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	// Bootstrap: HL = address of test array, B = count.
	// test_arr is a forward-reference label resolved by MZA.
	cases := []struct {
		data []int
		want int
	}{
		{[]int{10, 20, 30, 40, 50}, 150},   // 5 elements
		{[]int{1, 2, 3, 4, 5, 6, 7}, 28},  // 7 elements
		{[]int{255, 1}, 0},                 // overflow: 256 mod 256 = 0
		{[]int{100}, 100},                  // single element
	}
	for _, tc := range cases {
		n := len(tc.data)
		dbArgs := ""
		for i, v := range tc.data {
			if i > 0 {
				dbArgs += ", "
			}
			dbArgs += fmt.Sprintf("%d", v)
		}
		bootstrap := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD HL, test_arr
    LD B, %d
    CALL sum_array
    DI
    HALT
test_arr:
    DB %s
`, testLoadAddr, n, dbArgs)
		src := bootstrap + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("sum_array(%v): %v", tc.data, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("sum_array(%v) = %d, want %d", tc.data, int(a), tc.want)
		} else {
			t.Logf("sum_array(%v) = %d ✓", tc.data, int(a))
		}
	}

	// VM test: verify via interpreter (pointer = heap offset).
	vm := mir2.NewVM(m)
	data := []byte{5, 10, 15, 20}
	ptr := vm.AllocHeap(data)
	res, err := vm.Call("sum_array", []mir2.Value{ptr, {I: 4}})
	if err != nil {
		t.Fatalf("VM sum_array: %v", err)
	}
	if len(res) != 1 || res[0].I != 50 {
		t.Errorf("VM sum_array([5,10,15,20]) = %v, want 50", res)
	} else {
		t.Logf("VM sum_array([5,10,15,20]) = %d ✓", res[0].I)
	}
}

// ── memcpy ────────────────────────────────────────────────────────────────────
//
// fun memcpy(dst: *u8, src: *u8, n: u8) {
//
//	while n > 0 { *dst = *src; dst++; src++; n-- }
//
// }
//
// Demonstrates OpPtrBump on two simultaneous pointers: HL (dst) and DE (src).
// Z80 inner loop: LD A,(DE) / LD (HL),A / INC HL / INC DE / DEC B
func buildMemcpy(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("memcpy")
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	dst := b.Param("dst", mir2.TyPtr, mir2.ClassPointer)  // HL
	src := b.Param("src", mir2.TyPtr, mir2.ClassIndex)    // DE
	n := b.Param("n", mir2.TyU8, mir2.ClassCounter)       // B
	b.Jmp("loop_head", dst, src, n)

	head := b.SwitchToNewBlock("loop_head")
	dstP := b.BlockParam(head, mir2.TyPtr, mir2.ClassPointer)  // HL
	srcP := b.BlockParam(head, mir2.TyPtr, mir2.ClassIndex)    // DE
	cntP := b.BlockParam(head, mir2.TyU8, mir2.ClassCounter)   // B

	zero8 := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
	cond := b.Cmp(mir2.CmpNe, cntP, zero8, mir2.ClassFlag, false)
	b.BrIf(cond, "loop_body", []mir2.Reg{dstP, srcP, cntP}, "exit", nil)

	body := b.SwitchToNewBlock("loop_body")
	dstB := b.BlockParam(body, mir2.TyPtr, mir2.ClassPointer)  // HL
	srcB := b.BlockParam(body, mir2.TyPtr, mir2.ClassIndex)    // DE
	cntB := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)   // B

	elem := b.Load(srcB, mir2.TyU8, mir2.ClassAcc)             // LD A, (DE)
	b.Store(dstB, elem, mir2.TyU8)                             // LD (HL), A
	dst2 := b.PtrBump(dstB, 1, mir2.ClassPointer)              // INC HL
	src2 := b.PtrBump(srcB, 1, mir2.ClassIndex)                // INC DE
	one8 := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	cnt2 := b.Sub(cntB, one8, mir2.TyU8, mir2.ClassCounter)    // DEC B
	b.Jmp("loop_head", dst2, src2, cnt2)

	b.SwitchToNewBlock("exit")
	b.Ret()

	return f
}

// ── double / quad — multi-function OpCall ─────────────────────────────────────
//
// fun double(x: u8) -> u8 { return x + x }   // ADD A, A — no CALL
// fun quad(x: u8) -> u8 { return double(double(x)) }  // two CALLs
//
// Phase 3d: first OpCall test in MIR2.  Both CALLs in quad operate on A
// (ClassAcc) throughout — no live values cross the call boundary, so no
// PUSH/POP spill is needed.  The allocator places x, x2, x4 all in A.
//
// Expected Z80:
//
//	double:  ADD A, A / RET
//	quad:    CALL double / CALL double / RET
func buildDouble(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("double")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc) // A
	result := b.Add(x, x, mir2.TyU8, mir2.ClassAcc)
	b.Ret(result)
	return f
}

func buildQuad(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("quad")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassAcc)
	// x is in A; double expects A; result lands in A — no pre-call move needed.
	x2 := b.Call("double", []mir2.Reg{x}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})
	x4 := b.Call("double", []mir2.Reg{x2}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})
	b.Ret(x4)
	return f
}

func TestQuadZ80(t *testing.T) {
	m := &mir2.Module{Name: "quad"}
	buildDouble(m)
	buildQuad(m)
	asm := compileModule(t, m)
	t.Log("\n" + asm)

	cases := []struct{ x, want int }{
		{1, 4},
		{2, 8},
		{5, 20},
		{10, 40},
		{32, 128},
		{63, 252},
	}
	for _, tc := range cases {
		src := bootstrap1("quad", tc.x) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("quad(%d): %v", tc.x, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("quad(%d) = %d, want %d", tc.x, int(a), tc.want)
		} else {
			t.Logf("quad(%d) = %d ✓", tc.x, int(a))
		}
	}

	// VM test.
	vm := mir2.NewVM(m)
	res, err := vm.Call("quad", []mir2.Value{{I: 3}})
	if err != nil {
		t.Fatalf("VM quad(3): %v", err)
	}
	if len(res) != 1 || res[0].I != 12 {
		t.Errorf("VM quad(3) = %v, want 12", res)
	} else {
		t.Logf("VM quad(3) = %d ✓", res[0].I)
	}
}

// ── string_len — string literal + OpAddrOf + OpLoad iteration ─────────────────
//
// fun string_len(s: *u8) -> u8 {
//
//	var n: u8 = 0
//	while *s != 0 { s++; n++ }
//	return n
//
// }
//
// Tests: string pool emission (DB bytes, 0), OpAddrOf of a string symbol,
// OpLoad + CmpEq zero (AND A), OpPtrBump stride=1.
func buildStrLen(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("strlen")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	ptr := b.Param("s", mir2.TyPtr, mir2.ClassPointer)   // HL
	zero8 := b.Const(0, mir2.TyU8, mir2.ClassGeneral)    // const 0 for comparison
	n0 := b.Const(0, mir2.TyU8, mir2.ClassAcc)           // initial count = 0 (A)
	b.Jmp("loop_head", ptr, n0)

	head := b.SwitchToNewBlock("loop_head")
	ptrP := b.BlockParam(head, mir2.TyPtr, mir2.ClassPointer)  // HL
	nP := b.BlockParam(head, mir2.TyU8, mir2.ClassGeneral)     // count in C (not A, to survive load)

	ch := b.Load(ptrP, mir2.TyU8, mir2.ClassAcc)                       // LD A, (HL)
	notNull := b.Cmp(mir2.CmpNe, ch, zero8, mir2.ClassFlag, false)     // AND A (CP 0 → AND A)
	b.BrIf(notNull, "loop_body", []mir2.Reg{ptrP, nP}, "exit", []mir2.Reg{nP})

	body := b.SwitchToNewBlock("loop_body")
	ptrB := b.BlockParam(body, mir2.TyPtr, mir2.ClassPointer)  // HL
	nB := b.BlockParam(body, mir2.TyU8, mir2.ClassGeneral)     // C

	ptr2 := b.PtrBump(ptrB, 1, mir2.ClassPointer)                       // INC HL
	one8 := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	n2 := b.Add(nB, one8, mir2.TyU8, mir2.ClassGeneral)                 // INC C (add 1 in-place)
	b.Jmp("loop_head", ptr2, n2)

	exit := b.SwitchToNewBlock("exit")
	resultC := b.BlockParam(exit, mir2.TyU8, mir2.ClassGeneral)
	resultA := b.Move(resultC, mir2.TyU8, mir2.ClassAcc)                // LD A, C
	b.Ret(resultA)

	return f
}

func TestStrLenZ80(t *testing.T) {
	// Intern a test string into the module's string pool.
	m := &mir2.Module{Name: "strlen"}
	buildStrLen(m)
	helloSym, _ := m.Strings.InternKind("Hello", mir2.StrCString)
	worldSym, _ := m.Strings.InternKind("World!", mir2.StrCString)

	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	// Verify string data appears in the assembly output.
	if !strings.Contains(asm, "mir2_str_0:") {
		t.Errorf("expected string label mir2_str_0 in assembly")
	}
	if !strings.Contains(asm, "72, 101") { // H, e
		t.Errorf("expected 'Hello' bytes (72, 101...) in assembly")
	}

	// Z80 test: load HL = address of "Hello", call strlen, check A = 5.
	// sanitizeIdent("@mir2.str.0") → "_mir2_str_0" (@ and . become _).
	strBootstrap := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD HL, _mir2_str_0
    CALL strlen
    DI
    HALT
`, testLoadAddr)
	src := strBootstrap + "\n" + asm
	a, _, err := runZ80(t, src)
	if err != nil {
		t.Fatalf("strlen(\"Hello\"): %v", err)
	}
	if int(a) != 5 {
		t.Errorf("strlen(\"Hello\") = %d, want 5", int(a))
	} else {
		t.Logf("strlen(\"Hello\") = %d ✓", int(a))
	}

	_ = helloSym
	_ = worldSym

	// VM test: allocate strings in heap, call with pointer.
	vm := mir2.NewVM(m)
	hAddr := vm.AllocHeap([]byte("Hello\x00"))
	wAddr := vm.AllocHeap([]byte("World!\x00"))
	for _, tc := range []struct {
		ptr  mir2.Value
		want int64
		s    string
	}{
		{hAddr, 5, "Hello"},
		{wAddr, 6, "World!"},
	} {
		res, vmErr := vm.Call("strlen", []mir2.Value{tc.ptr})
		if vmErr != nil {
			t.Fatalf("VM strlen(%q): %v", tc.s, vmErr)
		}
		if len(res) != 1 || res[0].I != tc.want {
			t.Errorf("VM strlen(%q) = %v, want %d", tc.s, res, tc.want)
		} else {
			t.Logf("VM strlen(%q) = %d ✓", tc.s, res[0].I)
		}
	}
}

func TestMemcpyZ80(t *testing.T) {
	m := &mir2.Module{Name: "memcpy"}
	buildMemcpy(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	// Bootstrap: HL=dst_buf, DE=src_buf, B=4.
	// After call, LD A,(dst_buf) reads the first copied byte.
	bootstrap := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD HL, dst_buf
    LD DE, src_buf
    LD B, 4
    CALL memcpy
    LD A, (dst_buf)
    DI
    HALT
dst_buf:
    DB 0, 0, 0, 0
src_buf:
    DB 7, 14, 21, 28
`, testLoadAddr)
	src := bootstrap + "\n" + asm
	a, _, err := runZ80(t, src)
	if err != nil {
		t.Fatalf("memcpy Z80: %v", err)
	}
	if int(a) != 7 {
		t.Errorf("memcpy: dst_buf[0] = %d, want 7", int(a))
	} else {
		t.Logf("memcpy: dst_buf[0] = %d ✓  (7 copied from src)", int(a))
	}

	// VM test: verify full copy.
	vm := mir2.NewVM(m)
	dstPtr := vm.AllocHeap([]byte{0, 0, 0, 0})
	srcPtr := vm.AllocHeap([]byte{11, 22, 33, 44})
	_, vmErr := vm.Call("memcpy", []mir2.Value{dstPtr, srcPtr, {I: 4}})
	if vmErr != nil {
		t.Fatalf("VM memcpy: %v", vmErr)
	}
	copied := vm.ReadHeap(dstPtr.I, 4)
	want := []byte{11, 22, 33, 44}
	if string(copied) != string(want) {
		t.Errorf("VM memcpy: dst = %v, want %v", copied, want)
	} else {
		t.Logf("VM memcpy: dst = %v ✓", copied)
	}
}

// ── double_sum — live-across-call spill pattern ───────────────────────────────
//
// fun double_sum(a: u8 [acc], b: u8 [general]) -> u8 [acc] {
//     return double(a) + double(b)
// }
//
// Phase 3d: demonstrates the canonical spill pattern when a value is live
// across a function call.
//
//  - b (ClassGeneral → C) survives the first CALL double naturally, because
//    double only touches A.
//  - After CALL double, A = 2a.  We must spill A to ClassGeneral (D) before
//    the second call, then load b (C) back into A.
//
// Expected Z80:
//
//	double_sum:
//	    CALL double    ; A = 2a;  C = b (untouched)
//	    LD D, A        ; spill 2a → D
//	    LD A, C        ; b → A for second call
//	    CALL double    ; A = 2b
//	    ADD A, D       ; A = 2a + 2b
//	    RET
func buildDoubleSum(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("double_sum")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)

	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)          // A
	bParam := bld.Param("b", mir2.TyU8, mir2.ClassGeneral) // C

	// First call: double(a) → A.  b is in C (ClassGeneral), survives the call.
	a2 := bld.Call("double", []mir2.Reg{a}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})

	// Spill a2 (in A) to ClassGeneral (D) before the second call clobbers A.
	a2Saved := bld.Move(a2, mir2.TyU8, mir2.ClassGeneral) // LD D, A

	// Load b into A for the second call.
	bInA := bld.Move(bParam, mir2.TyU8, mir2.ClassAcc) // LD A, C

	// Second call: double(b) → A.
	b2 := bld.Call("double", []mir2.Reg{bInA}, mir2.TyU8, mir2.ClassAcc, mir2.CallAttrs{})

	// Sum: 2a + 2b.
	result := bld.Add(b2, a2Saved, mir2.TyU8, mir2.ClassAcc) // ADD A, D
	bld.Ret(result)

	return f
}

func TestDoubleSumZ80(t *testing.T) {
	m := &mir2.Module{Name: "double_sum"}
	buildDouble(m)
	buildDoubleSum(m)
	asm := compileModule(t, m)
	t.Log("\n" + asm)

	// Verify the spill pattern appears: CALL double, LD D A, LD A C, CALL double.
	if !strings.Contains(asm, "LD D, A") {
		t.Errorf("expected 'LD D, A' spill in assembly:\n%s", asm)
	}
	if !strings.Contains(asm, "LD A, C") {
		t.Errorf("expected 'LD A, C' reload in assembly:\n%s", asm)
	}

	cases := []struct{ a, b, want int }{
		{1, 2, 6},    // 2 + 4
		{3, 4, 14},   // 6 + 8
		{0, 5, 10},   // 0 + 10
		{10, 0, 20},  // 20 + 0
		{63, 64, 254}, // 126 + 128
	}
	for _, tc := range cases {
		// Bootstrap: load a into A, b into C, then call double_sum.
		src := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD A, %d
    LD C, %d
    CALL double_sum
    DI
    HALT
`, testLoadAddr, tc.a, tc.b) + "\n" + asm
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("double_sum(%d,%d): %v", tc.a, tc.b, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("double_sum(%d,%d) = %d, want %d", tc.a, tc.b, int(a), tc.want)
		} else {
			t.Logf("double_sum(%d,%d) = %d ✓", tc.a, tc.b, int(a))
		}
	}

	// VM test.
	vm := mir2.NewVM(m)
	res, err := vm.Call("double_sum", []mir2.Value{{I: 3}, {I: 4}})
	if err != nil {
		t.Fatalf("VM double_sum(3,4): %v", err)
	}
	if len(res) != 1 || res[0].I != 14 {
		t.Errorf("VM double_sum(3,4) = %v, want 14", res)
	} else {
		t.Logf("VM double_sum(3,4) = %d ✓", res[0].I)
	}
}

// ── min16 ─────────────────────────────────────────────────────────────────────
//
// fun min16(a: u16, b: u16) -> u16 { if a < b { return a } else { return b } }
//
// First 16-bit comparison test: uses SBC HL, DE to set C flag.
// Convention: a → HL (ClassPointer), b → DE (ClassIndex), return → HL.
func buildMin16(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("min16")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	bld := mir2.NewBuilder(f)

	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU16, mir2.ClassPointer)
	bv := bld.Param("b", mir2.TyU16, mir2.ClassIndex)

	// a < b (unsigned): SBC HL(=a), DE(=b) → C flag
	clt := bld.Cmp(mir2.CmpUlt, a, bv, mir2.ClassFlag, false)
	bld.BrIf(clt, "a_wins", nil, "b_wins", nil)

	bld.SwitchToNewBlock("a_wins")
	bld.Ret(a)

	bld.SwitchToNewBlock("b_wins")
	bvHL := bld.Move(bv, mir2.TyU16, mir2.ClassPointer) // EX DE,HL or LD HL,DE
	bld.Ret(bvHL)

	return f
}

// bootstrapHL_DE loads HL and DE then calls funcName.
// The result is read from HL after HALT.
func bootstrapHL_DE(funcName string, hlVal, deVal int) string {
	return fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD HL, %d
    LD DE, %d
    CALL %s
    DI
    HALT
`, testLoadAddr, hlVal, deVal, funcName)
}

func TestMin16Z80(t *testing.T) {
	m := &mir2.Module{Name: "min16"}
	buildMin16(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	// Verify SBC HL, DE is in the output (16-bit comparison).
	if !strings.Contains(asm, "SBC HL, DE") {
		t.Errorf("expected 'SBC HL, DE' in assembly:\n%s", asm)
	}

	cases := []struct{ a, b, want int }{
		{3, 7, 3},
		{7, 3, 3},
		{5, 5, 5},
		{0, 1000, 0},
		{1000, 0, 0},
		{300, 301, 300},
		{1000, 999, 999},
		{0, 65535, 0},
		{65535, 0, 0},
		{32767, 32768, 32767},
	}
	for _, tc := range cases {
		src := bootstrapHL_DE("min16", tc.a, tc.b) + "\n" + asm
		_, hl, err := runZ80(t, src)
		if err != nil {
			t.Errorf("min16(%d,%d): %v", tc.a, tc.b, err)
			continue
		}
		if int(hl) != tc.want {
			t.Errorf("min16(%d,%d) = %d, want %d", tc.a, tc.b, int(hl), tc.want)
		} else {
			t.Logf("min16(%d,%d) = %d ✓", tc.a, tc.b, int(hl))
		}
	}
}

// ── @print intrinsic ──────────────────────────────────────────────────────────
//
// TestPrintStrZ80 verifies the @mir.io.print.str intrinsic:
//
//	fun greet(s: *u8) -> void {
//	    @mir.io.print.str(s)
//	}
//
// The intrinsic is inlined as a NUL-terminated string output loop via
// OUT ($23), A — the mze/mzx standard console port.
// The test verifies that the emulator captures the expected string.

// buildGreet builds a simple function that calls @mir.io.print.str with its
// string-pointer argument.
func buildGreet(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("greet")
	// No return value: void function
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	ptr := b.Param("s", mir2.TyPtr, mir2.ClassPointer) // HL = string pointer

	// Emit @mir.io.print.str(ptr) — inlined by codegen
	b.IntrinsicVoid("io.print.str", []mir2.Reg{ptr})
	b.Ret()

	return f
}

// runZ80WithOutput assembles and runs src; returns captured OUT($23) output and error.
func runZ80WithOutput(t *testing.T, src string) (string, error) {
	t.Helper()
	asm := z80asm.NewAssembler()
	res, asmErr := asm.AssembleString(src)
	if asmErr != nil {
		return "", fmt.Errorf("assemble: %w", asmErr)
	}
	if len(res.Errors) > 0 {
		var sb strings.Builder
		for _, e := range res.Errors {
			sb.WriteString(e.Error())
			sb.WriteByte('\n')
		}
		return "", fmt.Errorf("assemble errors:\n%s", sb.String())
	}
	z80 := emulator.NewRemogattoZ80()
	if loadErr := z80.LoadMemory(testLoadAddr, res.Binary); loadErr != nil {
		return "", fmt.Errorf("load memory: %w", loadErr)
	}
	z80.SetPC(testLoadAddr)
	if runErr := z80.Run(); runErr != nil {
		return "", fmt.Errorf("run: %w", runErr)
	}
	return string(z80.GetOutput()), nil
}

func TestPrintStrZ80(t *testing.T) {
	m := &mir2.Module{Name: "print_test"}
	buildGreet(m)
	// Intern the string into the module's string pool so it gets emitted as data.
	strSym, _ := m.Strings.InternKind("Hello, MIR2!", mir2.StrCString)

	asm := compileFunc(t, m)
	t.Logf("generated assembly:\n%s", asm)

	// The sanitized symbol for "@mir2.str.0" → "_mir2_str_0"
	_ = strSym

	// Bootstrap: load HL = address of "Hello, MIR2!", call greet, HALT.
	boot := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD HL, _mir2_str_0
    CALL greet
    DI
    HALT
`, testLoadAddr)

	src := boot + "\n" + asm
	got, err := runZ80WithOutput(t, src)
	if err != nil {
		t.Fatalf("greet: %v", err)
	}
	want := "Hello, MIR2!"
	if got != want {
		t.Errorf("print output = %q, want %q", got, want)
	} else {
		t.Logf("print output = %q ✓", got)
	}
}

// ── sum_array_djnz ─────────────────────────────────────────────────────────────
//
// Same as sum_array but uses TermDJNZ explicitly (do-while loop structure).
// Assumes count >= 1 (caller guarantees non-empty array).
// The loop body ends with DJNZ instead of DEC B + JP loop_head.
func buildSumArrayDJNZ(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("sum_array_djnz")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	base := b.Param("base", mir2.TyPtr, mir2.ClassPointer)   // HL
	count := b.Param("count", mir2.TyU8, mir2.ClassCounter)  // B
	acc0 := b.Const(0, mir2.TyU8, mir2.ClassAcc)
	accG := b.Move(acc0, mir2.TyU8, mir2.ClassGeneral) // acc in C
	// Initial check: if count == 0, skip loop.
	zero8 := b.Const(0, mir2.TyU8, mir2.ClassGeneral)
	notZero := b.Cmp(mir2.CmpNe, count, zero8, mir2.ClassFlag, false)
	// body.Params order: cnt(B), ptr(HL), acc(C)
	b.BrIf(notZero, "loop_body", []mir2.Reg{count, base, accG}, "exit", []mir2.Reg{accG})

	// loop_body(cnt: ClassCounter, ptr: ClassPointer, acc: ClassGeneral)
	// Counter MUST be Params[0] for DJNZ (body.Params[0] receives decremented B).
	body := b.SwitchToNewBlock("loop_body")
	cntB := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)   // B (counter, implicit in DJNZ) — must be Params[0]
	ptrB := b.BlockParam(body, mir2.TyPtr, mir2.ClassPointer)  // HL
	accB := b.BlockParam(body, mir2.TyU8, mir2.ClassGeneral)   // C

	elem := b.Load(ptrB, mir2.TyU8, mir2.ClassAcc)             // LD A, (HL)
	sum := b.Add(elem, accB, mir2.TyU8, mir2.ClassAcc)         // ADD A, C
	accNew := b.Move(sum, mir2.TyU8, mir2.ClassGeneral)        // LD C, A
	ptr2 := b.PtrBump(ptrB, 1, mir2.ClassPointer)              // INC HL
	// TermDJNZ: decrement cntB (B), jump to loop_body if non-zero.
	// BodyArgs = args for body.Params[1:] (ptr and acc; counter is implicit as Params[0]).
	b.DJNZ(cntB, "loop_body", []mir2.Reg{ptr2, accNew}, "exit", []mir2.Reg{accNew})

	exit := b.SwitchToNewBlock("exit")
	resultC := b.BlockParam(exit, mir2.TyU8, mir2.ClassGeneral)
	resultA := b.Move(resultC, mir2.TyU8, mir2.ClassAcc)       // LD A, C
	b.Ret(resultA)

	return f
}

func TestSumArrayDJNZZ80(t *testing.T) {
	m := &mir2.Module{Name: "sum_array_djnz"}
	buildSumArrayDJNZ(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	// Verify DJNZ instruction is in the generated code.
	if !strings.Contains(asm, "DJNZ") {
		t.Errorf("expected DJNZ in generated assembly:\n%s", asm)
	}

	cases := []struct {
		data []int
		want int
	}{
		{[]int{10, 20, 30, 40, 50}, 150},
		{[]int{1, 2, 3, 4, 5, 6, 7}, 28},
		{[]int{100}, 100},
		{[]int{255, 1}, 0}, // overflow: 256 mod 256 = 0
	}
	for _, tc := range cases {
		// Build data section.
		var dataLines []string
		for _, v := range tc.data {
			dataLines = append(dataLines, fmt.Sprintf("    DB %d", v))
		}
		data := "test_arr_djnz:\n" + strings.Join(dataLines, "\n")
		boot := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD HL, test_arr_djnz
    LD B, %d
    CALL sum_array_djnz
    DI
    HALT
`, testLoadAddr, len(tc.data))
		src := boot + "\n" + asm + "\n" + data
		a, _, err := runZ80(t, src)
		if err != nil {
			t.Errorf("sum_array_djnz(%v): %v", tc.data, err)
			continue
		}
		if int(a) != tc.want {
			t.Errorf("sum_array_djnz(%v) = %d, want %d", tc.data, int(a), tc.want)
		} else {
			t.Logf("sum_array_djnz(%v) = %d ✓", tc.data, int(a))
		}
	}
}

func TestSumArrayDJNZPeephole(t *testing.T) {
	// The existing buildSumArray uses the pre-check while-loop structure.
	// The DJNZ peephole should automatically convert it to DJNZ.
	m := &mir2.Module{Name: "sum_array_peep"}
	buildSumArray(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)
	if !strings.Contains(asm, "DJNZ") {
		t.Errorf("expected DJNZ peephole to apply:\n%s", asm)
	}
}

// ── abs_diff_iar ──────────────────────────────────────────────────────────────
//
// IAR-style single-subtraction abs_diff — compare to IAR Cortex-M3 output:
//
//   unsigned abs_diff_iar(unsigned a, unsigned b) {
//       unsigned r = a - b;         // one SUB, sets carry
//       if (a >= b) return r;       // RET NC — carry IS the comparison
//       return -r;                  // NEG; RET
//   }
//
// IAR generates 3 instructions (Cortex-M3):
//   SUBS  R0, R0, R1   ; r = a-b, flags
//   IT CC / RSBCC R0, R0, #0  ; if borrow: R0 = -R0
//   BX LR
//
// Z80 ideal (after CondRetSink + Sub+CmpGe fusion):
//   SUB  B    ; A = a-b, CF=borrow (4T)
//   RET  NC   ; a>=b → return      (5T taken, 11T not)
//   NEG        ; A = -(a-b)        (8T)
//   RET        ; return            (10T)
//
// Fast path: 4+5 = 9T.  Slow path: 4+11+8+10 = 33T.
func buildAbsDiffIAR(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("abs_diff_iar")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)

	// r = a - b  (sets flags; carry = borrow = a < b)
	r := b.Sub(a, bv, mir2.TyU8, mir2.ClassAcc)

	// Use CmpLt and swap then/else so CondRetSink sinks the TRIVIAL ret_r
	// (just Ret(r), no instructions) rather than the NEG path.
	//
	//   BrIf(CmpLt(a,b), then=ret_neg, else=ret_r)
	//   CondRetSink fires on else=ret_r (trivial):
	//     → TermCondRet(cmp, vals=[r], then=ret_neg)
	//     → "if !CmpLt (a>=b): RET NC with A=r; else: NEG; RET"
	//
	// fusionSubCmpInBlock converts CmpLt(a,b) → CmpSubCarry(r,b), so the
	// allocator no longer sees `a` as live at the Cmp.
	// Final Z80: SUB B / RET NC / NEG / RET  (ideal IAR output, 9T fast path).
	cmp := b.Cmp(mir2.CmpLt, a, bv, mir2.ClassFlag, false)
	b.BrIf(cmp, "ret_neg", nil, "ret_r", nil) // else=ret_r is trivial → gets sunk

	b.SwitchToNewBlock("ret_r") // trivial else — CondRetSink will hoist this
	b.Ret(r)

	b.SwitchToNewBlock("ret_neg")
	nr := b.Neg(r, mir2.TyU8, mir2.ClassAcc)
	b.Ret(nr)

	return f
}

// ── abs_diff_iar_u16 ──────────────────────────────────────────────────────────
//
// Same IAR pattern but for u16.  Z80 ideal:
//   AND A         ; clear carry      (4T)
//   SBC HL, DE    ; HL = a-b, CF=borrow  (15T)
//   RET NC        ; a>=b → return    (5T / 11T)
//   XOR A         ; 16-bit NEG HL via SBC A,A trick
//   SUB L         ; (24T total for the neg)
//   LD  L, A
//   SBC A, A
//   SUB H
//   LD  H, A
//   RET
//
// Fast path: 4+15+5 = 24T.  Slow path: 4+15+11+24+10 = 64T.
func buildAbsDiffIARU16(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("abs_diff_iar_u16")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU16, mir2.ClassPointer)
	bv := b.Param("b", mir2.TyU16, mir2.ClassIndex)

	r := b.Sub(a, bv, mir2.TyU16, mir2.ClassPointer)

	// Same CmpLt+swap-branches trick as buildAbsDiffIAR (see above).
	cmp := b.Cmp(mir2.CmpLt, a, bv, mir2.ClassFlag, false)
	b.BrIf(cmp, "ret_neg", nil, "ret_r", nil) // else=ret_r is trivial

	b.SwitchToNewBlock("ret_r")
	b.Ret(r)

	b.SwitchToNewBlock("ret_neg")
	nr := b.Neg(r, mir2.TyU16, mir2.ClassPointer)
	b.Ret(nr)

	return f
}

// ── abs_diff_u16 ───────────────────────────────────────────────────────────────
//
// fun abs_diff_u16(a: u16, b: u16) -> u16 {
//     if a >= b { return a - b }
//     return b - a      // b-a = -(a-b); uses SBC A,A trick for 16-bit NEG
// }
//
// Z80 calling convention: a → HL (ClassPointer), b → DE (ClassIndex).
func buildAbsDiffU16(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("abs_diff_u16")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU16, mir2.ClassPointer)
	bv := b.Param("b", mir2.TyU16, mir2.ClassIndex)

	// if a >= b → a_ge_b
	cmp := b.Cmp(mir2.CmpGe, a, bv, mir2.ClassFlag, false)
	b.BrIf(cmp, "a_ge_b", nil, "b_gt_a", nil)

	// a_ge_b: return a - b  (HL = HL - DE; CondRetSink should fire)
	b.SwitchToNewBlock("a_ge_b")
	diff1 := b.Sub(a, bv, mir2.TyU16, mir2.ClassPointer)
	b.Ret(diff1)

	// b_gt_a: return b - a, i.e. -(a - b). Compiler computes sub(b,a)
	// and the 16-bit NEG path fires via OpNeg + SBC A,A trick.
	b.SwitchToNewBlock("b_gt_a")
	diff2 := b.Sub(bv, a, mir2.TyU16, mir2.ClassPointer)
	b.Ret(diff2)

	return f
}

// bootstrapHL2DE loads HL and DE before calling a two-u16-arg function.
func bootstrapHL2DE(funcName string, argHL, argDE int) string {
	return fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD HL, %d
    LD DE, %d
    CALL %s
    DI
    HALT
`, testLoadAddr, argHL, argDE, funcName)
}

func TestAbsDiffU16Z80(t *testing.T) {
	m := &mir2.Module{Name: "abs_diff_u16"}
	buildAbsDiffU16(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	// The compiler emits Sub(b, a) via EX DE,HL + SBC HL,DE directly (operand swap),
	// which is more efficient than going through OpNeg.  The SBC A,A trick is still
	// exercised when OpNeg u16 is emitted explicitly — see TestNegU16Z80.

	cases := []struct{ a, b, want int }{
		{1000, 300, 700},
		{300, 1000, 700},
		{500, 500, 0},
		{0, 65535, 65535},
		{65535, 0, 65535},
		{1, 2, 1},
	}
	for _, tc := range cases {
		src := bootstrapHL2DE("abs_diff_u16", tc.a, tc.b) + "\n" + asm
		_, hl, err := runZ80(t, src)
		if err != nil {
			t.Errorf("abs_diff_u16(%d,%d): %v", tc.a, tc.b, err)
			continue
		}
		if int(hl) != tc.want {
			t.Errorf("abs_diff_u16(%d,%d) = %d, want %d", tc.a, tc.b, int(hl), tc.want)
		} else {
			t.Logf("abs_diff_u16(%d,%d) = %d ✓", tc.a, tc.b, int(hl))
		}
	}
}

// TestNegU16Z80 exercises OpNeg on a u16 value and verifies the XOR/SUB/SBC A,A
// demo-scene trick is emitted.  Conceptually: neg_u16(x) = -x as unsigned u16.
func TestNegU16Z80(t *testing.T) {
	m := &mir2.Module{Name: "neg_u16"}
	f := m.AddFunc("neg_u16")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU16, mir2.ClassPointer)
	nx := b.Neg(x, mir2.TyU16, mir2.ClassPointer)
	b.Ret(nx)

	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	// Must use the SBC A,A trick, not the old NEG-based 8-bit sequence.
	if !strings.Contains(asm, "SBC A, A") {
		t.Errorf("expected SBC A, A trick for 16-bit NEG:\n%s", asm)
	}

	cases := []struct{ x, want int }{
		{0, 0},
		{1, 65535},
		{256, 65280},
		{1000, 64536},
		{65535, 1},
	}
	for _, tc := range cases {
		src := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD HL, %d
    CALL neg_u16
    DI
    HALT
`, testLoadAddr, tc.x) + asm
		_, hl, err := runZ80(t, src)
		if err != nil {
			t.Errorf("neg_u16(%d): %v", tc.x, err)
			continue
		}
		if int(hl) != tc.want {
			t.Errorf("neg_u16(%d) = %d, want %d", tc.x, int(hl), tc.want)
		} else {
			t.Logf("neg_u16(%d) = %d ✓", tc.x, int(hl))
		}
	}
}

// ── clamp_u16 ─────────────────────────────────────────────────────────────────
//
// fun clamp_u16(val, lo, hi: u16) -> u16 {
//     if val < lo  { return lo  }
//     if hi < val  { return hi  }
//     return val
// }
//
// Z80 calling convention: val→HL, lo→DE, hi→BC (three u16 params).
func buildClampU16(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("clamp_u16")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	val := b.Param("val", mir2.TyU16, mir2.ClassPointer)
	lo := b.Param("lo", mir2.TyU16, mir2.ClassIndex)
	hi := b.Param("hi", mir2.TyU16, mir2.ClassPair)

	cmpLo := b.Cmp(mir2.CmpLt, val, lo, mir2.ClassFlag, false) // val < lo → C
	b.BrIf(cmpLo, "below_lo", nil, "check_hi", nil)

	b.SwitchToNewBlock("check_hi")
	cmpHi := b.Cmp(mir2.CmpLt, hi, val, mir2.ClassFlag, false) // hi < val → C
	b.BrIf(cmpHi, "above_hi", nil, "done", nil)

	b.SwitchToNewBlock("below_lo")
	b.Ret(lo)

	b.SwitchToNewBlock("above_hi")
	hiHL := b.Move(hi, mir2.TyU16, mir2.ClassPointer)
	b.Ret(hiHL)

	b.SwitchToNewBlock("done")
	b.Ret(val)

	return f
}

// bootstrapHLDEBC loads HL, DE, BC then calls funcName; result read from HL.
func bootstrapHLDEBC(funcName string, hlVal, deVal, bcVal int) string {
	return fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD HL, %d
    LD DE, %d
    LD BC, %d
    CALL %s
    DI
    HALT
`, testLoadAddr, hlVal, deVal, bcVal, funcName)
}

func TestClampU16Z80(t *testing.T) {
	m := &mir2.Module{Name: "clamp_u16"}
	buildClampU16(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct{ val, lo, hi, want int }{
		{50, 10, 100, 50},
		{5, 10, 100, 10},
		{200, 10, 100, 100},
		{10, 10, 100, 10},
		{100, 10, 100, 100},
		{0, 0, 65535, 0},
		{65535, 0, 65535, 65535},
		{1000, 500, 2000, 1000},
		{300, 500, 2000, 500},
		{3000, 500, 2000, 2000},
	}
	for _, tc := range cases {
		// Allocator assigns: val→HL, lo→BC, hi→DE; so pass deVal=hi, bcVal=lo.
		src := bootstrapHLDEBC("clamp_u16", tc.val, tc.hi, tc.lo) + "\n" + asm
		_, hl, err := runZ80(t, src)
		if err != nil {
			t.Errorf("clamp_u16(%d,%d,%d): %v", tc.val, tc.lo, tc.hi, err)
			continue
		}
		if int(hl) != tc.want {
			t.Errorf("clamp_u16(%d,%d,%d) = %d, want %d", tc.val, tc.lo, tc.hi, int(hl), tc.want)
		} else {
			t.Logf("clamp_u16(%d,%d,%d) = %d ✓", tc.val, tc.lo, tc.hi, int(hl))
		}
	}
}

// ── mul_u16 ───────────────────────────────────────────────────────────────────
//
// fun mul_u16(a: u16, b: u16) -> u16 { return a * b }
//
// Z80: a → HL (ClassPointer), b → DE (ClassIndex), result → HL.
func buildMulU16(m *mir2.Module, bConst int64, useConst bool) *mir2.Func {
	name := "mul_u16"
	if useConst {
		name = fmt.Sprintf("mul_u16_by_%d", bConst)
	}
	f := m.AddFunc(name)
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU16, mir2.ClassPointer)
	var bv mir2.Reg
	if useConst {
		bv = b.Const(bConst, mir2.TyU16, mir2.ClassIndex)
	} else {
		bv = b.Param("b", mir2.TyU16, mir2.ClassIndex)
	}
	result := b.Mul(a, bv, mir2.TyU16, mir2.ClassPointer)
	b.Ret(result)
	return f
}

func TestMulU16ConstZ80(t *testing.T) {
	cases := []struct {
		factor int64
		tests  []struct{ a, want int }
	}{
		{2, []struct{ a, want int }{{3, 6}, {100, 200}, {1000, 2000}}},
		{3, []struct{ a, want int }{{5, 15}, {100, 300}, {21845, 65535}}},
		{4, []struct{ a, want int }{{10, 40}, {1000, 4000}, {16384, 0}}}, // wrap at 16-bit
		{5, []struct{ a, want int }{{13, 65}, {100, 500}}},
		{6, []struct{ a, want int }{{7, 42}, {100, 600}}},
		{9, []struct{ a, want int }{{7, 63}, {100, 900}}},
		{8, []struct{ a, want int }{{100, 800}, {1000, 8000}}},
		{16, []struct{ a, want int }{{100, 1600}, {4096, 0}}},
		{256, []struct{ a, want int }{{1, 256}, {200, 51200}, {257, 256}}},  // Fix A: LD H,L / LD L,0
		{512, []struct{ a, want int }{{1, 512}, {100, 51200}, {129, 512}}},  // Fix A: byte-swap + ADD HL,HL
	}
	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("x%d", tc.factor), func(t *testing.T) {
			m := &mir2.Module{Name: fmt.Sprintf("mul_u16_by_%d", tc.factor)}
			buildMulU16(m, tc.factor, true)
			asm := compileFunc(t, m)
			t.Log("\n" + asm)
			funcName := fmt.Sprintf("mul_u16_by_%d", tc.factor)
			for _, inner := range tc.tests {
				src := bootstrapHL_DE(funcName, inner.a, 0) + "\n" + asm
				_, hl, err := runZ80(t, src)
				if err != nil {
					t.Errorf("%d*%d: %v", inner.a, tc.factor, err)
					continue
				}
				got := int(hl)
				want := (inner.a * int(tc.factor)) & 0xFFFF
				_ = inner.want // computed dynamically
				if got != want {
					t.Errorf("%d * %d = %d, want %d", inner.a, tc.factor, got, want)
				} else {
					t.Logf("%d * %d = %d ✓", inner.a, tc.factor, got)
				}
			}
		})
	}
}

func TestMulU16Const256Asm(t *testing.T) {
	// Fix A: * 256 must use LD H,L / LD L,0 (byte-swap), not 8× ADD HL,HL
	m := &mir2.Module{Name: "mul_u16_by_256"}
	buildMulU16(m, 256, true)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)
	if !strings.Contains(asm, "LD H, L") || !strings.Contains(asm, "LD L, 0") {
		t.Errorf("expected LD H,L / LD L,0 byte-swap; got:\n%s", asm)
	}
	// Count ADD HL,HL — should be 0 for *256
	count := strings.Count(asm, "ADD HL, HL")
	if count != 0 {
		t.Errorf("expected 0 ADD HL,HL for *256, got %d", count)
	}
}

func TestMulU16VarZ80(t *testing.T) {
	m := &mir2.Module{Name: "mul_u16"}
	buildMulU16(m, 0, false)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct{ a, b, want int }{
		{3, 5, 15},
		{10, 10, 100},
		{255, 255, 65025},
		{256, 256, 0},   // wraps to 0 mod 2^16
		{1000, 7, 7000},
		{0, 12345, 0},
		{1, 65535, 65535},
		{7, 9, 63},
	}
	for _, tc := range cases {
		src := bootstrapHL_DE("mul_u16", tc.a, tc.b) + "\n" + asm
		_, hl, err := runZ80(t, src)
		if err != nil {
			t.Errorf("mul_u16(%d,%d): %v", tc.a, tc.b, err)
			continue
		}
		got := int(hl)
		want := (tc.a * tc.b) & 0xFFFF
		if got != want {
			t.Errorf("mul_u16(%d,%d) = %d, want %d", tc.a, tc.b, got, want)
		} else {
			t.Logf("mul_u16(%d,%d) = %d ✓", tc.a, tc.b, got)
		}
	}
}

// ── abs_diff_u32 ──────────────────────────────────────────────────────────────
//
// fun abs_diff_u32(a: u32, b: u32) -> u32 {
//     if a >= b { return a - b }
//     return b - a
// }
//
// Z80 calling convention (ClassDWord):
//   a → HL+H'L' (lo in main HL, hi in shadow H'L')
//   b → DE+D'E' (lo in main DE, hi in shadow D'E')
//   result → HL+H'L'
func buildAbsDiffU32(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("abs_diff_u32")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU32, Class: mir2.ClassDWord}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU32, mir2.ClassDWord)
	bv := b.Param("b", mir2.TyU32, mir2.ClassDWord)

	cmp := b.Cmp(mir2.CmpGe, a, bv, mir2.ClassFlag, false)
	b.BrIf(cmp, "a_ge_b", nil, "b_gt_a", nil)

	b.SwitchToNewBlock("a_ge_b")
	diff1 := b.Sub(a, bv, mir2.TyU32, mir2.ClassDWord)
	b.Ret(diff1)

	b.SwitchToNewBlock("b_gt_a")
	diff2 := b.Sub(bv, a, mir2.TyU32, mir2.ClassDWord)
	b.Ret(diff2)

	return f
}

// testDWordResultAddr is the memory address where bootstrapDWord2 stores the
// 32-bit result after the function call (4 bytes: lo16 at addr, hi16 at addr+2).
const testDWordResultAddr = uint16(0xFFF0)

// bootstrapDWord2 loads a into HL+H'L' and b into DE+D'E', calls funcName,
// then stores the 32-bit result (HL+H'L') to testDWordResultAddr.
func bootstrapDWord2(funcName string, a, b uint32) string {
	aLo := a & 0xFFFF
	aHi := (a >> 16) & 0xFFFF
	bLo := b & 0xFFFF
	bHi := (b >> 16) & 0xFFFF
	return fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    EXX
    LD HL, %d
    LD DE, %d
    EXX
    LD HL, %d
    LD DE, %d
    CALL %s
    LD (0x%04X), HL
    EXX
    LD (0x%04X), HL
    EXX
    DI
    HALT
`, testLoadAddr, aHi, bHi, aLo, bLo, funcName,
		testDWordResultAddr, testDWordResultAddr+2)
}

// runZ80Mem4 assembles and runs src, then reads 4 bytes from addr as a uint32
// (little-endian: byte[addr]=bits0-7, byte[addr+3]=bits24-31).
func runZ80Mem4(t *testing.T, src string, addr uint16) (uint32, error) {
	t.Helper()
	asm := z80asm.NewAssembler()
	res, asmErr := asm.AssembleString(src)
	if asmErr != nil {
		return 0, fmt.Errorf("assemble: %w", asmErr)
	}
	if len(res.Errors) > 0 {
		var sb strings.Builder
		for _, e := range res.Errors {
			sb.WriteString(e.Error())
			sb.WriteByte('\n')
		}
		return 0, fmt.Errorf("assemble errors:\n%s", sb.String())
	}
	z80 := emulator.NewRemogattoZ80()
	if loadErr := z80.LoadMemory(testLoadAddr, res.Binary); loadErr != nil {
		return 0, fmt.Errorf("load memory: %w", loadErr)
	}
	z80.SetPC(testLoadAddr)
	if runErr := z80.Run(); runErr != nil {
		return 0, fmt.Errorf("run: %w", runErr)
	}
	b0 := uint32(z80.GetMemory(addr))
	b1 := uint32(z80.GetMemory(addr + 1))
	b2 := uint32(z80.GetMemory(addr + 2))
	b3 := uint32(z80.GetMemory(addr + 3))
	return b0 | b1<<8 | b2<<16 | b3<<24, nil
}

func TestAbsDiffU32Z80(t *testing.T) {
	m := &mir2.Module{Name: "abs_diff_u32"}
	buildAbsDiffU32(m)
	asm := compileFunc(t, m)
	t.Log("\n" + asm)

	cases := []struct {
		a, b, want uint32
	}{
		{1000, 300, 700},
		{300, 1000, 700},
		{500, 500, 0},
		{0, 65535, 65535},
		{65535, 0, 65535},
		{0x00010000, 0x00000001, 0x0000FFFF},
		{0x00000001, 0x00010000, 0x0000FFFF},
		{0xFFFFFFFF, 0x00000000, 0xFFFFFFFF},
		{0x80000000, 0x7FFFFFFF, 0x00000001},
		{0x12345678, 0x11111111, 0x01234567},
	}
	for _, tc := range cases {
		src := bootstrapDWord2("abs_diff_u32", tc.a, tc.b) + "\n" + asm
		got, err := runZ80Mem4(t, src, testDWordResultAddr)
		if err != nil {
			t.Errorf("abs_diff_u32(0x%08X, 0x%08X): %v", tc.a, tc.b, err)
			continue
		}
		if got != tc.want {
			t.Errorf("abs_diff_u32(0x%08X, 0x%08X) = 0x%08X, want 0x%08X",
				tc.a, tc.b, got, tc.want)
		} else {
			t.Logf("abs_diff_u32(0x%08X, 0x%08X) = 0x%08X ✓", tc.a, tc.b, got)
		}
	}
}
