package mir2_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// ── Dual-target compilation helper ──────────────────────────────────────────

func compileDual(t *testing.T, m *mir2.Module) (z80asm, m6502asm string) {
	t.Helper()
	for _, f := range m.Funcs {
		mir2.ReorderBlocks(f)
	}
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}

	// Z80
	z80combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		for r, loc := range ar.Locs {
			z80combined.Locs[r] = loc
		}
	}
	z80asm = mir2.Z80Codegen(m, z80combined)

	// 6502
	m6502combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.M6502CostTable{})
		for r, loc := range ar.Locs {
			m6502combined.Locs[r] = loc
		}
	}
	m6502asm = mir2.M6502Codegen(m, m6502combined)
	return
}

// countInstructions counts non-label, non-comment, non-empty lines.
func countInstructions(asm string) int {
	n := 0
	for _, line := range strings.Split(asm, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") ||
			strings.HasPrefix(trimmed, ".cpu") || strings.HasPrefix(trimmed, "* =") {
			continue
		}
		if strings.HasSuffix(trimmed, ":") {
			continue
		}
		n++
	}
	return n
}

// ── Showcase: add ────────────────────────────────────────────────────────────
//
// fun add(a: u8, b: u8) -> u8 { return a + b }
//
// Optimal hand-written 6502:
//     ; a in A, b in X or zp
//     CLC         ; 2 cyc — clear carry for addition
//     ADC operand ; 3 cyc — add b (from ZP) to A
//     RTS         ; 6 cyc — return (result in A)
//     ; Total: 3 instructions, 11 cycles + JSR overhead

func TestShowcase_Add(t *testing.T) {
	m := &mir2.Module{Name: "add"}
	f := m.AddFunc("add")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassGeneral)
	sum := b.Add(a, bv, mir2.TyU8, mir2.ClassAcc)
	b.Ret(sum)

	z80, m65 := compileDual(t, m)

	t.Log("═══ Z80 ═══")
	t.Log("\n" + z80)
	t.Log("═══ 6502 ═══")
	t.Log("\n" + m65)
	t.Log("═══ 6502 ANNOTATED ═══")
	t.Log(annotate6502(m65))
	t.Logf("═══ Instruction count: Z80=%d  6502=%d ═══",
		countInstructions(z80), countInstructions(m65))
}

// ── Showcase: abs_diff ──────────────────────────────────────────────────────
//
// fun abs_diff(a: u8, b: u8) -> u8 {
//     if a >= b { return a - b }
//     return b - a
// }
//
// Optimal hand-written 6502:
//     ; a in A, b in ZP
//     SEC         ; 2 cyc — set carry for subtraction
//     SBC zp      ; 3 cyc — A = a - b (sets C if a >= b)
//     BCS done    ; 2/3  — if no borrow, result is correct
//     EOR #$FF    ; 2 cyc — bitwise NOT (part of negate)
//     CLC         ; 2 cyc — clear carry
//     ADC #1      ; 2 cyc — +1 to complete two's complement negate
//   done:
//     RTS         ; 6 cyc
//     ; Total: 7 instructions, ~17 cycles average

func TestShowcase_AbsDiff(t *testing.T) {
	m := &mir2.Module{Name: "abs_diff"}
	buildAbsDiff(m)

	z80, m65 := compileDual(t, m)

	t.Log("═══ Z80 ═══")
	t.Log("\n" + z80)
	t.Log("═══ 6502 ═══")
	t.Log("\n" + m65)
	t.Log("═══ 6502 ANNOTATED ═══")
	t.Log(annotate6502(m65))
	t.Logf("═══ Instruction count: Z80=%d  6502=%d ═══",
		countInstructions(z80), countInstructions(m65))
}

// ── Showcase: double ────────────────────────────────────────────────────────
//
// fun double(x: u8) -> u8 { return x + x }
//
// Optimal hand-written 6502:
//     ASL A       ; 2 cyc — arithmetic shift left = multiply by 2
//     RTS         ; 6 cyc
//     ; Total: 2 instructions, 8 cycles
//
// (Note: CLC;ADC A is also correct but 2 extra bytes/cycles vs ASL A.
//  A peephole could catch ADD A,A → ASL A.)

func TestShowcase_Double(t *testing.T) {
	m := &mir2.Module{Name: "double"}
	buildDouble(m)

	z80, m65 := compileDual(t, m)

	t.Log("═══ Z80 ═══")
	t.Log("\n" + z80)
	t.Log("═══ 6502 ═══")
	t.Log("\n" + m65)
	t.Log("═══ 6502 ANNOTATED ═══")
	t.Log(annotate6502(m65))
	t.Logf("═══ Instruction count: Z80=%d  6502=%d ═══",
		countInstructions(z80), countInstructions(m65))

	// Peephole opportunity: CLC;ADC A → ASL A
	if strings.Contains(m65, "CLC") && strings.Contains(m65, "ADC A") {
		t.Log("💡 PEEPHOLE: CLC;ADC A could be reduced to ASL A (saves 1 byte, 2 cycles)")
	}
}

// ── Showcase: GCD ──────────────────────────────────────────────────────────
//
// fun gcd(a: u8, b: u8) -> u8 {
//     while a != b {
//         if a < b { b = b - a }
//         else     { a = a - b }
//     }
//     return a
// }
//
// Optimal hand-written 6502:
//   gcd:
//     STX zp0     ; 3 cyc — save b to ZP (or keep in X if CMP works)
//   .loop:
//     CPX zp0     ; 3 cyc — compare X(a) with zp0(b) — wait, need A for CMP
//     ; Actually: a in A, b in X. CMP doesn't exist for X vs A directly.
//     ; Must use: STX zp / CMP zp, or rearrange.
//     ; Best approach: a in zp0, b in zp1, use A as scratch:
//   .loop:
//     LDA zp0     ; 3 cyc — load a
//     CMP zp1     ; 3 cyc — compare a with b (sets C and Z)
//     BEQ .done   ; 2/3  — a == b → return
//     BCC .a_lt   ; 2/3  — a < b (carry clear)
//     SEC         ; 2    — a > b: a = a - b
//     SBC zp1     ; 3    — A = a - b
//     STA zp0     ; 3    — store new a
//     JMP .loop   ; 3    — back to loop
//   .a_lt:
//     LDA zp1     ; 3    — load b
//     SEC         ; 2
//     SBC zp0     ; 3    — A = b - a
//     STA zp1     ; 3    — store new b
//     JMP .loop   ; 3
//   .done:
//     LDA zp0     ; 3    — return a in A
//     RTS         ; 6
//     ; Total: ~17 instructions, ~25 cycles per iteration

func TestShowcase_GCD(t *testing.T) {
	m := &mir2.Module{Name: "gcd"}
	buildGCD(m)

	z80, m65 := compileDual(t, m)

	t.Log("═══ Z80 ═══")
	t.Log("\n" + z80)
	t.Log("═══ 6502 ═══")
	t.Log("\n" + m65)
	t.Log("═══ 6502 ANNOTATED ═══")
	t.Log(annotate6502(m65))
	t.Logf("═══ Instruction count: Z80=%d  6502=%d ═══",
		countInstructions(z80), countInstructions(m65))
}

// ── Showcase: clamp ─────────────────────────────────────────────────────────
//
// fun clamp(val: u8, lo: u8, hi: u8) -> u8 {
//     if val < lo { return lo }
//     if val > hi { return hi }
//     return val
// }
//
// Optimal hand-written 6502:
//     ; val in A, lo in X, hi in Y (or ZP)
//     STX zp0     ; save lo
//     STY zp1     ; save hi
//     CMP zp0     ; compare val with lo
//     BCS .not_low; if val >= lo, skip
//     LDA zp0     ; val < lo → return lo
//     RTS
//   .not_low:
//     CMP zp1     ; compare val with hi
//     BCC .done   ; if val < hi, val is in range
//     BEQ .done   ; if val == hi, also fine
//     LDA zp1     ; val > hi → return hi
//   .done:
//     RTS
//     ; Total: ~10 instructions

func TestShowcase_Clamp(t *testing.T) {
	m := &mir2.Module{Name: "clamp"}
	buildClamp(m)

	z80, m65 := compileDual(t, m)

	t.Log("═══ Z80 ═══")
	t.Log("\n" + z80)
	t.Log("═══ 6502 ═══")
	t.Log("\n" + m65)
	t.Log("═══ 6502 ANNOTATED ═══")
	t.Log(annotate6502(m65))
	t.Logf("═══ Instruction count: Z80=%d  6502=%d ═══",
		countInstructions(z80), countInstructions(m65))
}

// ── Showcase: quad (call chain) ─────────────────────────────────────────────
//
// fun double(x: u8) -> u8 { return x + x }
// fun quad(x: u8) -> u8 { return double(double(x)) }
//
// Optimal hand-written 6502:
//   double:
//     ASL A       ; 2 cyc — x * 2
//     RTS         ; 6 cyc
//   quad:
//     JSR double  ; 6 cyc — call double(x)
//     JSR double  ; 6 cyc — call double(result)
//     RTS         ; 6 cyc
//     ; Total: 5 instructions, 26 cycles

func TestShowcase_Quad(t *testing.T) {
	m := &mir2.Module{Name: "quad"}
	buildDouble(m)
	buildQuad(m)

	z80, m65 := compileDual(t, m)

	t.Log("═══ Z80 ═══")
	t.Log("\n" + z80)
	t.Log("═══ 6502 ═══")
	t.Log("\n" + m65)
	t.Log("═══ 6502 ANNOTATED ═══")
	t.Log(annotate6502(m65))
	t.Logf("═══ Instruction count: Z80=%d  6502=%d ═══",
		countInstructions(z80), countInstructions(m65))

	// Check that PFCCO eliminates inter-call moves (both calls use A→A)
	// Count adapter moves between JSR calls
	lines := strings.Split(m65, "\n")
	for i, line := range lines {
		if strings.Contains(line, "JSR") && i+1 < len(lines) {
			next := strings.TrimSpace(lines[i+1])
			if strings.HasPrefix(next, "JSR") || next == "RTS" {
				t.Log("✓ PFCCO: no adapter moves between calls (A→A convention match)")
			}
		}
	}
}

// ── Showcase: SMC counter (patchable instruction) ───────────────────────────
//
// fun counter() -> u8 {
//     slot = patch_slot 0            // self-modifying: initial value 0
//     val  = load_patched slot       // reads the current patched value
//     next = val + 1
//     patch slot, next               // overwrites the immediate in-place
//     return val
// }
//
// 6502 SMC is natural — the instruction stream is in writable RAM:
//     counter:
//       LDA #$00     ; ← this $00 byte is patched in-place
//       TAX          ; save current value to return later
//       CLC
//       ADC #$01     ; increment
//       STA counter+1; write new value back into the LDA immediate
//       TXA          ; restore original value to A for return
//       RTS
//     ; Total: 7 instructions. The `STA counter+1` is the self-modify.

func TestShowcase_SMCCounter(t *testing.T) {
	m := &mir2.Module{Name: "smc"}
	f := m.AddFunc("counter")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	slot := bld.PatchSlot(0, mir2.TyU8, mir2.ClassAcc)
	val := bld.LoadPatched(slot, mir2.TyU8, mir2.ClassAcc)
	one := bld.Const(1, mir2.TyU8, mir2.ClassGeneral)
	next := bld.Add(val, one, mir2.TyU8, mir2.ClassAcc)
	bld.Patch(slot, next, mir2.TyU8)
	bld.Ret(val)

	z80, m65 := compileDual(t, m)

	t.Log("═══ Z80 ═══")
	t.Log("\n" + z80)
	t.Log("═══ 6502 ═══")
	t.Log("\n" + m65)
	t.Log("═══ 6502 ANNOTATED ═══")
	t.Log(annotate6502(m65))
	t.Logf("═══ Instruction count: Z80=%d  6502=%d ═══",
		countInstructions(z80), countInstructions(m65))

	// Check SMC support
	if strings.Contains(m65, "patch_slot") || strings.Contains(m65, "TODO") {
		t.Log("⚠ 6502 codegen does not yet lower patch_slot/load_patched/patch — emits TODO comments")
		t.Log("  SMC on 6502 is NATURAL: code runs from RAM, so STA into the instruction stream works directly.")
		t.Log("  Unlike Z80 where you might need LD (addr),A — on 6502 it's just STA label+1.")
	} else {
		t.Log("✓ SMC instructions lowered to 6502 assembly")
	}
}

// ── 6502 instruction annotation ─────────────────────────────────────────────

var m6502annotations = map[string]string{
	// Load/Store
	"LDA": "Load Accumulator — value → A register",
	"LDX": "Load X register — value → X",
	"LDY": "Load Y register — value → Y",
	"STA": "Store Accumulator — A → memory",
	"STX": "Store X — X → memory",
	"STY": "Store Y — Y → memory",

	// Transfer
	"TAX": "Transfer A→X — copy accumulator to X (2 cycles)",
	"TAY": "Transfer A→Y — copy accumulator to Y (2 cycles)",
	"TXA": "Transfer X→A — copy X to accumulator (2 cycles)",
	"TYA": "Transfer Y→A — copy Y to accumulator (2 cycles)",

	// Arithmetic
	"ADC": "Add with Carry — A = A + operand + C flag",
	"SBC": "Subtract with Borrow — A = A - operand - (1-C)",
	"CLC": "Clear Carry flag — must precede ADC for clean addition",
	"SEC": "Set Carry flag — must precede SBC for clean subtraction",

	// Compare
	"CMP": "Compare A — sets Z (equal), C (A≥operand), N (sign of A-operand)",
	"CPX": "Compare X — same as CMP but for X register",
	"CPY": "Compare Y — same as CMP but for Y register",

	// Logic
	"AND": "Bitwise AND — A = A & operand",
	"ORA": "Bitwise OR — A = A | operand",
	"EOR": "Exclusive OR — A = A ^ operand (also used for bitwise NOT via EOR #$FF)",

	// Shift
	"ASL": "Arithmetic Shift Left — bit7→C, shift left, 0→bit0 (= multiply by 2)",
	"LSR": "Logical Shift Right — bit0→C, shift right, 0→bit7 (= unsigned divide by 2)",
	"ROL": "Rotate Left through Carry — C→bit0, bit7→C",
	"ROR": "Rotate Right through Carry — C→bit7, bit0→C",

	// Inc/Dec
	"INX": "Increment X — X = X + 1 (2 cycles)",
	"INY": "Increment Y — Y = Y + 1 (2 cycles)",
	"DEX": "Decrement X — X = X - 1 (2 cycles, sets Z for loop)",
	"DEY": "Decrement Y — Y = Y - 1 (2 cycles, sets Z for loop)",
	"INC": "Increment memory — [addr] = [addr] + 1 (5 cycles ZP, 6 abs)",
	"DEC": "Decrement memory — [addr] = [addr] - 1 (5 cycles ZP, 6 abs)",

	// Branch
	"BEQ": "Branch if Equal (Z=1) — relative jump if last result was zero",
	"BNE": "Branch if Not Equal (Z=0) — relative jump if last result was non-zero",
	"BCC": "Branch if Carry Clear (C=0) — used for 'less than' (unsigned)",
	"BCS": "Branch if Carry Set (C=1) — used for 'greater or equal' (unsigned)",
	"BPL": "Branch if Plus (N=0) — positive result",
	"BMI": "Branch if Minus (N=1) — negative result",
	"BVC": "Branch if Overflow Clear (V=0)",
	"BVS": "Branch if Overflow Set (V=1)",

	// Jump/Return
	"JMP": "Jump — unconditional jump to absolute address (3 cycles)",
	"JSR": "Jump to Subroutine — push PC+2, jump to address (6 cycles)",
	"RTS": "Return from Subroutine — pull PC, jump to PC+1 (6 cycles)",

	// Stack
	"PHA": "Push A — push accumulator to stack (3 cycles)",
	"PLA": "Pull A — pull top of stack to accumulator (4 cycles)",
	"PHP": "Push Processor Status — push flags to stack",
	"PLP": "Pull Processor Status — pull flags from stack",

	// Misc
	"NOP": "No Operation — 2 cycles, 1 byte",
	"BRK": "Software Interrupt — push PC+2 and P, jump to IRQ vector",
}

func annotate6502(asm string) string {
	var sb strings.Builder
	for _, line := range strings.Split(asm, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, ";") {
			sb.WriteString(line + "\n")
			continue
		}
		// Find the mnemonic (first word after leading spaces)
		mnemonic := ""
		for _, word := range strings.Fields(trimmed) {
			if !strings.HasSuffix(word, ":") {
				mnemonic = word
				break
			}
		}
		if ann, ok := m6502annotations[mnemonic]; ok {
			sb.WriteString(fmt.Sprintf("%-30s ; %s\n", line, ann))
		} else {
			sb.WriteString(line + "\n")
		}
	}
	return sb.String()
}
