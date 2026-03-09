package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// TestE2EFibonacciZ80 is the full pipeline test:
//
//	buildFib → ReorderBlocks → Verify → ComputeLiveness → Allocate
//	→ Z80Codegen → MZA assemble → MZE emulate → check HL result
//
// The fibonacci function is called with n in A and returns fib(n) in HL.
func TestE2EFibonacciZ80(t *testing.T) {
	// Build and optimise the MIR2 module.
	m := &mir2.Module{Name: "fib"}
	buildFib(m)

	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify after ReorderBlocks: %v", err)
	}

	// Allocate registers.
	var fibAsm string
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		fibAsm = mir2.Z80Codegen(m, ar)
	}

	t.Logf("generated assembly:\n%s", fibAsm)

	// Test cases: (n, expected fib(n)).
	cases := [][2]int{{0, 0}, {1, 1}, {2, 1}, {3, 2}, {5, 5}, {8, 21}, {10, 55}}

	for _, tc := range cases {
		n, want := tc[0], tc[1]
		got, tStates, err := runFibZ80(t, fibAsm, n)
		if err != nil {
			t.Errorf("fib(%d): emulator error: %v", n, err)
			continue
		}
		if got != want {
			t.Errorf("fib(%d) = %d, want %d", n, got, want)
		} else {
			t.Logf("fib(%d) = %d  (%d T-states)", n, got, tStates)
		}
	}
}

// runFibZ80 assembles and runs fibonacci(n) through the Z80 emulator.
// Returns the HL result, the T-state count, and any error.
func runFibZ80(t *testing.T, fibAsm string, n int) (result int, tStates int, err error) {
	t.Helper()

	// Build the full assembly: bootstrap + fibonacci function body.
	// The bootstrap loads n into A, calls fibonacci, then DI+HALT to stop.
	const loadAddr = 0x8000
	bootstrap := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD A, %d
    CALL fibonacci
    DI
    HALT
`, loadAddr, n)

	src := bootstrap + "\n" + fibAsm

	// Assemble.
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

	// Load into emulator and run.
	z80 := emulator.NewRemogattoZ80()
	if loadErr := z80.LoadMemory(uint16(loadAddr), res.Binary); loadErr != nil {
		return 0, 0, fmt.Errorf("load memory: %w", loadErr)
	}
	z80.SetPC(uint16(loadAddr))

	if runErr := z80.Run(); runErr != nil {
		return 0, 0, fmt.Errorf("run: %w", runErr)
	}

	regs := z80.GetRegisters()
	return int(regs.HL), z80.GetCycles(), nil
}

// ── Flag-return ABI E2E test ──────────────────────────────────────────────────

// TestE2EFlagReturn verifies the flag-return ABI:
//
//	isLess(a, b: u8) -> bool  [ClassFlag, CmpUlt — C flag]
//	clampByte(x, lo, hi: u8) -> u8  (uses isLess internally)
//
// isLess returns via C flag (carry = a < b = true).
// The caller branches directly with JP C / JP NC — no AND A or CP 0.
//
// Verification: clampByte(x, 10, 200) should clamp x into [10..200].
func TestE2EFlagReturn(t *testing.T) {
	m := &mir2.Module{Name: "flag_ret_test"}

	// ── isLess(a: u8 in A, b: u8 in C) -> bool in C flag (CmpUlt) ────────────
	//
	// MIR2:
	//   entry: %cmp = cmp.ult %a, %b : bool [ClassFlag]
	//          ret %cmp
	//
	// Expected Z80:
	//   isLess:
	//     CP C       ; A - C → sets C if A < C
	//     RET        ; return C flag directly — no LD A, 0/1 !
	isLess := m.AddFunc("isLess")
	{
		pa := isLess.AllocReg() // param a in A
		pb := isLess.AllocReg() // param b in C
		isLess.Contract.Params = []mir2.Param{
			{Name: "a", Reg: pa, Ty: mir2.TyU8, Class: mir2.ClassAcc},
			{Name: "b", Reg: pb, Ty: mir2.TyU8, Class: mir2.ClassCounter},
		}
		isLess.Contract.Returns = []mir2.Return{
			{Ty: mir2.TyBool, Class: mir2.ClassFlag, FlagCond: mir2.CmpUlt},
		}

		entry := isLess.NewBlock("entry")
		cmp := isLess.AllocReg()
		entry.Insts = append(entry.Insts, &mir2.Inst{
			Op: mir2.OpCmp, Dst: cmp, Src: [2]mir2.Reg{pa, pb},
			Ty: mir2.TyBool, Cls: mir2.ClassFlag, Cond: mir2.CmpUlt,
		})
		entry.Term = &mir2.TermRet{Vals: []mir2.Reg{cmp}}
	}

	// ── clampByte(x, lo, hi: u8) -> u8 ───────────────────────────────────────
	//
	// Equivalent to: if x < lo { return lo } else if hi < x { return hi } else { return x }
	//
	// Calls isLess twice.  The caller uses JP C directly after each CALL
	// (no AND A needed because isLess returns ClassFlag, CmpUlt → C flag).
	//
	// MIR2 structure:
	//   entry: %lt_lo = call isLess(%x, %lo)  [ClassFlag]
	//          brif %lt_lo, clamp_lo, check_hi
	//   clamp_lo: ret %lo
	//   check_hi: %gt_hi = call isLess(%hi, %x)  [ClassFlag]
	//             brif %gt_hi, clamp_hi, in_range
	//   clamp_hi: ret %hi
	//   in_range: ret %x
	clamp := m.AddFunc("clampByte")
	{
		px := clamp.AllocReg()
		plo := clamp.AllocReg()
		phi := clamp.AllocReg()
		clamp.Contract.Params = []mir2.Param{
			{Name: "x", Reg: px, Ty: mir2.TyU8, Class: mir2.ClassAcc},
			{Name: "lo", Reg: plo, Ty: mir2.TyU8, Class: mir2.ClassCounter},
			{Name: "hi", Reg: phi, Ty: mir2.TyU8, Class: mir2.ClassGeneral}, // C/D/E
		}
		clamp.Contract.Returns = []mir2.Return{
			{Ty: mir2.TyU8, Class: mir2.ClassAcc},
		}

		entry := clamp.NewBlock("entry")
		clampLo := clamp.NewBlock("clamp_lo")
		checkHi := clamp.NewBlock("check_hi")
		clampHi := clamp.NewBlock("clamp_hi")
		inRange := clamp.NewBlock("in_range")

		// entry: %lt_lo = call isLess(%x, %lo)
		ltLo := clamp.AllocReg()
		entry.Insts = append(entry.Insts, &mir2.Inst{
			Op: mir2.OpCall, Dst: ltLo, Sym: "isLess",
			Args: []mir2.Reg{px, plo}, Ty: mir2.TyBool, Cls: mir2.ClassFlag,
		})
		entry.Term = &mir2.TermBrIf{Cond: ltLo, Then: clampLo.Label, Else: checkHi.Label}

		clampLo.Term = &mir2.TermRet{Vals: []mir2.Reg{plo}}

		// check_hi: %gt_hi = call isLess(%hi, %x)
		gtHi := clamp.AllocReg()
		checkHi.Insts = append(checkHi.Insts, &mir2.Inst{
			Op: mir2.OpCall, Dst: gtHi, Sym: "isLess",
			Args: []mir2.Reg{phi, px}, Ty: mir2.TyBool, Cls: mir2.ClassFlag,
		})
		checkHi.Term = &mir2.TermBrIf{Cond: gtHi, Then: clampHi.Label, Else: inRange.Label}

		clampHi.Term = &mir2.TermRet{Vals: []mir2.Reg{phi}}

		inRange.Term = &mir2.TermRet{Vals: []mir2.Reg{px}}
	}

	// ── Compile ───────────────────────────────────────────────────────────────
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}

	var fullAsm string
	ar := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		fAr := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		for r, loc := range fAr.Locs {
			ar.Locs[r] = loc
		}
	}
	fullAsm = mir2.Z80Codegen(m, ar)
	t.Logf("flag-return assembly:\n%s", fullAsm)

	// ── Verify: isLess uses C flag (no AND A / CP 0 in body) ─────────────────
	isLessLines := extractFunc(fullAsm, "isLess")
	t.Logf("isLess body:\n%s", isLessLines)

	// The isLess body must NOT contain LD A, 0 / LD A, 1 (flag materialisation).
	if strings.Contains(isLessLines, "LD A, 0") || strings.Contains(isLessLines, "LD A, 1") {
		t.Errorf("isLess body materialises bool to A — flag-return ABI not applied:\n%s", isLessLines)
	}
	// Must contain CP (the comparison instruction).
	if !strings.Contains(isLessLines, "CP") {
		t.Errorf("isLess body missing CP instruction:\n%s", isLessLines)
	}

	// ── Verify: clampByte caller uses JP C directly (no AND A after CALL) ────
	clampLines := extractFunc(fullAsm, "clampByte")
	t.Logf("clampByte body:\n%s", clampLines)

	// The clamp body must contain JP C or JP NC (direct flag branch after CALL
	// isLess — either the taken or not-taken edge uses the carry flag directly).
	if !strings.Contains(clampLines, "JP C") && !strings.Contains(clampLines, "JP NC") {
		t.Errorf("clampByte missing JP C/JP NC — caller not using flag-return ABI:\n%s", clampLines)
	}
	// Must NOT contain AND A between CALL and JP (that would defeat the purpose).
	if strings.Contains(clampLines, "AND A") {
		t.Errorf("clampByte contains AND A — redundant flag check emitted:\n%s", clampLines)
	}

	// ── Functional correctness via emulator ───────────────────────────────────
	cases := []struct{ x, lo, hi, want int }{
		{5, 10, 200, 10},   // below range: clamp to lo
		{10, 10, 200, 10},  // at lo: return lo (or x, same value)
		{100, 10, 200, 100}, // in range: return x
		{200, 10, 200, 200}, // at hi: return hi (or x, same value)
		{250, 10, 200, 200}, // above range: clamp to hi
	}
	for _, c := range cases {
		got, tStates, err := runClampZ80(t, fullAsm, c.x, c.lo, c.hi)
		if err != nil {
			t.Errorf("clampByte(%d,%d,%d): %v", c.x, c.lo, c.hi, err)
			continue
		}
		if got != c.want {
			t.Errorf("clampByte(%d,%d,%d) = %d, want %d", c.x, c.lo, c.hi, got, c.want)
		} else {
			t.Logf("clampByte(%d,%d,%d) = %d (%dT)", c.x, c.lo, c.hi, got, tStates)
		}
	}
}

// extractFunc returns the assembly lines belonging to the named function,
// from its label up to (not including) the next top-level label.
func extractFunc(asm, name string) string {
	lines := strings.Split(asm, "\n")
	var out []string
	inFunc := false
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		// Top-level label (not indented, ends with :, no leading dot)
		if len(trimmed) > 0 && trimmed[len(trimmed)-1] == ':' && !strings.HasPrefix(trimmed, ".") {
			funcName := trimmed[:len(trimmed)-1]
			if funcName == name {
				inFunc = true
			} else if inFunc {
				break // hit the next function label
			}
		}
		if inFunc {
			out = append(out, l)
		}
	}
	return strings.Join(out, "\n")
}

// runClampZ80 runs clampByte(x, lo, hi) and returns A (result), T-states, error.
func runClampZ80(t *testing.T, asm string, x, lo, hi int) (int, int, error) {
	t.Helper()
	const loadAddr = 0x8000
	bootstrap := fmt.Sprintf(`
    ORG 0x%04X
    LD SP, 0xFF00
    LD A, %d
    LD B, %d
    LD C, %d
    CALL clampByte
    DI
    HALT
`, loadAddr, x, lo, hi)

	src := bootstrap + "\n" + asm

	assembler := z80asm.NewAssembler()
	res, err := assembler.AssembleString(src)
	if err != nil {
		return 0, 0, fmt.Errorf("assemble: %w", err)
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
	if err := z80.LoadMemory(uint16(loadAddr), res.Binary); err != nil {
		return 0, 0, fmt.Errorf("load: %w", err)
	}
	z80.SetPC(uint16(loadAddr))
	if err := z80.Run(); err != nil {
		return 0, 0, fmt.Errorf("run: %w", err)
	}
	regs := z80.GetRegisters()
	return int(regs.A), z80.GetCycles(), nil
}
