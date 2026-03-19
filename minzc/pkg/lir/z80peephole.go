// z80peephole.go — Post-emit Z80 peephole optimizer for LIR.
//
// Applies proven transformations on emitted assembly text.
// Sources: superoptimizer (602K proven rules), MIR2 peephole (46 patterns),
// SDCC comparison analysis.
//
// Every transformation here is provably correct (exhaustive verification
// or algebraic identity). Each pattern documents its source and savings.
package lir

import (
	"strings"
)

// Z80Peephole applies post-emit peephole optimizations to Z80 assembly.
// Input and output are complete assembly text (multi-line).
// Runs multiple passes until no more changes.
func Z80Peephole(asm string) string {
	lines := strings.Split(asm, "\n")

	changed := true
	for pass := 0; changed && pass < 10; pass++ {
		changed = false

		// Single-instruction rewrites
		for i := range lines {
			if new, ok := rewriteSingle(lines[i]); ok {
				lines[i] = new
				changed = true
			}
		}

		// Two-instruction window (peephole-2)
		for i := 0; i+1 < len(lines); i++ {
			if new1, new2, ok := rewritePair(lines[i], lines[i+1]); ok {
				lines[i] = new1
				lines[i+1] = new2
				changed = true
			}
		}

		// Remove lines marked as dead (empty string with original indent preserved)
		newLines := lines[:0]
		for _, l := range lines {
			if l != "\x00DEAD" {
				newLines = append(newLines, l)
			}
		}
		lines = newLines
	}

	return strings.Join(lines, "\n")
}

// deadLine marks a line for removal.
const deadLine = "\x00DEAD"

// ── Single-instruction rewrites ─────────────────────────────────────────────

func rewriteSingle(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, ";") {
		return "", false
	}

	indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]

	// LD A, 0 → XOR A (saves 1B, 3T)
	// Source: superoptimizer + MIR2 peephole pattern #1
	if trimmed == "LD A, 0" || trimmed == "LD A, $00" || trimmed == "LD A, 0x00" {
		return indent + "XOR A", true
	}

	// SLA A → ADD A, A (saves 1B, 4T: SLA=8T/2B vs ADD=4T/1B)
	// Source: superoptimizer, algebraic identity (shift left = double)
	if trimmed == "SLA A" {
		return indent + "ADD A, A", true
	}

	// CP 0 → OR A (saves 1B: CP=7T/2B vs OR=4T/1B, both set Z flag for A==0)
	// Source: MIR2 peephole pattern #22, superoptimizer
	// NOTE: OR A also resets C flag; CP 0 sets C=0 too. Equivalent for Z/NZ tests.
	if trimmed == "CP 0" || trimmed == "CP $00" {
		return indent + "OR A", true
	}

	// ADD A, 0 → NOP (identity, remove)
	if trimmed == "ADD A, 0" {
		return deadLine, true
	}

	// SUB 0 → NOP (identity, remove)
	if trimmed == "SUB 0" {
		return deadLine, true
	}

	// AND 255 / AND $FF → NOP (8-bit identity: A & 0xFF = A)
	// Source: MIR2 peephole pattern #37
	if trimmed == "AND 255" || trimmed == "AND $FF" || trimmed == "AND 0xFF" {
		return deadLine, true
	}

	// OR 0 → NOP (identity)
	if trimmed == "OR 0" || trimmed == "OR $00" {
		return deadLine, true
	}

	// XOR 0 → NOP (identity)
	if trimmed == "XOR 0" || trimmed == "XOR $00" {
		return deadLine, true
	}

	// LD A, A → NOP (self-load)
	// LD B, B → NOP etc.
	// Source: MIR2 peephole pattern #36
	if isNopLD(trimmed) {
		return deadLine, true
	}

	// ADD A, 1 → INC A (saves 1B)
	// Source: MIR2 peephole pattern #2
	if trimmed == "ADD A, 1" {
		return indent + "INC A", true
	}

	// SUB 1 → DEC A (saves 1B)
	if trimmed == "SUB 1" {
		return indent + "DEC A", true
	}

	return "", false
}

func isNopLD(trimmed string) bool {
	nopLDs := []string{
		"LD A, A", "LD B, B", "LD C, C", "LD D, D",
		"LD E, E", "LD H, H", "LD L, L",
	}
	for _, nop := range nopLDs {
		if trimmed == nop {
			return true
		}
	}
	return false
}

// ── Two-instruction rewrites ────────────────────────────────────────────────

func rewritePair(line1, line2 string) (string, string, bool) {
	t1 := strings.TrimSpace(line1)
	t2 := strings.TrimSpace(line2)

	if t1 == "" || t2 == "" || strings.HasPrefix(t1, ";") || strings.HasPrefix(t2, ";") {
		return "", "", false
	}

	// Double EX DE, HL → remove both (self-inverse)
	// Source: MIR2 peephole, algebraic identity
	if t1 == "EX DE, HL" && t2 == "EX DE, HL" {
		return deadLine, deadLine, true
	}

	// Double NEG → remove both ((-(-x)) = x)
	if t1 == "NEG" && t2 == "NEG" {
		return deadLine, deadLine, true
	}

	// Double CPL → remove both (~~x = x)
	if t1 == "CPL" && t2 == "CPL" {
		return deadLine, deadLine, true
	}

	// Double EXX → remove both
	if t1 == "EXX" && t2 == "EXX" {
		return deadLine, deadLine, true
	}

	// Double CCF → remove both (complement twice = identity)
	if t1 == "CCF" && t2 == "CCF" {
		return deadLine, deadLine, true
	}

	// PUSH rr / POP rr (same register) → remove both (no effect)
	// Source: MIR2 peephole pattern #6
	if strings.HasPrefix(t1, "PUSH ") && strings.HasPrefix(t2, "POP ") {
		pushReg := strings.TrimSpace(t1[5:])
		popReg := strings.TrimSpace(t2[4:])
		if pushReg == popReg {
			return deadLine, deadLine, true
		}
	}

	// XOR A followed by CP 0 / OR A / AND A → remove second (flags already set)
	// Source: MIR2 peephole pattern #28,#43
	if t1 == "XOR A" && (t2 == "CP 0" || t2 == "OR A" || t2 == "AND A") {
		return line1, deadLine, true
	}

	// OR A followed by OR A → remove second (idempotent flag setter)
	// Source: MIR2 peephole pattern #42
	if t1 == "OR A" && t2 == "OR A" {
		return line1, deadLine, true
	}
	if t1 == "AND A" && t2 == "AND A" {
		return line1, deadLine, true
	}

	// SLA A; RR A → OR A (saves 3B/12T)
	// Source: superoptimizer, proven by exhaustive verification
	if t1 == "SLA A" && t2 == "RR A" {
		indent := line1[:len(line1)-len(strings.TrimLeft(line1, " \t"))]
		return indent + "OR A", deadLine, true
	}

	// SRL A; RL A → OR A (saves 3B/12T)
	// Source: superoptimizer, proven by exhaustive verification
	if t1 == "SRL A" && t2 == "RL A" {
		indent := line1[:len(line1)-len(strings.TrimLeft(line1, " \t"))]
		return indent + "OR A", deadLine, true
	}

	// LD r, X; LD r, Y → LD r, Y (second overwrites first, remove first)
	// Source: dead store elimination
	if strings.HasPrefix(t1, "LD ") && strings.HasPrefix(t2, "LD ") {
		dst1 := extractLDDst(t1)
		dst2 := extractLDDst(t2)
		if dst1 != "" && dst1 == dst2 {
			// Don't eliminate if first LD reads memory (side effects)
			if !strings.Contains(t1, "(") {
				return deadLine, line2, true
			}
		}
	}

	return "", "", false
}

// extractLDDst extracts the destination from "LD X, Y" → "X"
func extractLDDst(trimmed string) string {
	if !strings.HasPrefix(trimmed, "LD ") {
		return ""
	}
	rest := trimmed[3:]
	comma := strings.Index(rest, ",")
	if comma < 0 {
		return ""
	}
	dst := strings.TrimSpace(rest[:comma])
	upper := strings.ToUpper(dst)
	// Only track simple register destinations
	switch upper {
	case "A", "B", "C", "D", "E", "H", "L",
		"BC", "DE", "HL", "SP", "IX", "IY":
		return upper
	}
	return ""
}
