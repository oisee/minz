package mir2

import "strings"

// z80TStates returns the T-state cost of a single Z80 instruction line as
// emitted by z80codegen.  Lines that are labels, blank, or comments return
// (0, 0).
//
// Returns (primary, secondary) where:
//   - primary  = T-states for the normal / taken path
//   - secondary = T-states for the NOT-taken path (only non-zero for
//     conditional branches: JRS cc, RET cc, DJNZ)
//
// Sources: Zilog Z80 CPU User Manual UM0080, Table 2 (instruction timings).
func z80TStates(line string) (primary, secondary int) {
	// Only instruction lines have a 4-space indent.
	if !strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "    ;") {
		return 0, 0
	}
	s := strings.TrimSpace(line)
	if s == "" {
		return 0, 0
	}

	mnem, rest, _ := strings.Cut(s, " ")
	mnem = strings.ToUpper(mnem)
	rest = strings.TrimSpace(rest)

	switch mnem {
	case "NOP", "EXX", "CPL", "DAA", "SCF", "CCF", "HALT", "RLA", "RRA", "RLCA", "RRCA":
		return 4, 0
	case "EX":
		// EX DE,HL = 4T; EX (SP),HL = 19T; EX AF,AF' = 4T
		if strings.Contains(rest, "(SP)") {
			return 19, 0
		}
		return 4, 0

	case "LD":
		return tsLD(rest)

	case "ADD":
		return tsADD(rest)
	case "ADC":
		return tsADCSBC(rest)
	case "SBC":
		return tsADCSBC(rest)

	case "SUB", "AND", "OR", "XOR", "CP":
		// All: r=4T, n=7T, (HL)=7T
		return tsALU1(rest)

	case "NEG":
		return 8, 0

	case "INC", "DEC":
		return tsINCDEC(rest)

	// CB-prefix bit ops on registers: 8T; on (HL): 15T; on (IX+d): 23T
	case "SRL", "SLA", "SRA", "SLL", "RR", "RL", "RRC", "RLC",
		"BIT", "SET", "RES":
		if strings.Contains(rest, "(IX") || strings.Contains(rest, "(IY") {
			return 23, 0
		}
		if strings.Contains(rest, "(HL)") {
			return 15, 0
		}
		return 8, 0

	case "PUSH":
		if rest == "IX" || rest == "IY" {
			return 15, 0
		}
		return 11, 0
	case "POP":
		if rest == "IX" || rest == "IY" {
			return 14, 0
		}
		return 10, 0

	case "JP":
		// JP nn = 10T; JP cc,nn = 10T (always, Z80 doesn't short-circuit)
		return 10, 0
	case "JR", "JRS":
		// JR e = 12T; JR cc,e = 12T/7T
		cc, _, found := strings.Cut(rest, ",")
		if found && isCC(strings.TrimSpace(cc)) {
			return 12, 7
		}
		return 12, 0

	case "DJNZ":
		return 13, 8 // 13T loop (B≠0), 8T exit (B=0)

	case "CALL":
		// CALL nn = 17T; CALL cc,nn = 17T/10T (we only emit unconditional CALLs)
		return 17, 0
	case "RET":
		if rest == "" {
			return 10, 0
		}
		return 11, 5 // RET cc: 11T taken, 5T not-taken

	case "RST":
		return 11, 0

	case "OUT":
		// OUT (C),r = 12T; OUT (n),A = 11T
		if strings.HasPrefix(rest, "(C)") {
			return 12, 0
		}
		return 11, 0
	case "IN":
		// IN r,(C) = 12T; IN A,(n) = 11T
		if strings.Contains(rest, "(C)") {
			return 12, 0
		}
		return 11, 0

	case "LDIR", "LDDR", "CPIR", "CPDR":
		// Variable; use per-iteration cost
		return 21, 16

	case "DI", "EI":
		return 4, 0
	}
	return 0, 0
}

// isCC reports whether s is a Z80 condition code.
func isCC(s string) bool {
	switch s {
	case "NZ", "Z", "NC", "C", "PO", "PE", "P", "M",
		"CGT", "CLE": // synthetic two-jump conditions used in BrIf codegen
		return true
	}
	return false
}

// isR8 reports whether s names an 8-bit register (A B C D E H L).
func isR8(s string) bool {
	switch s {
	case "A", "B", "C", "D", "E", "H", "L", "I", "R":
		return true
	}
	return false
}

// isR16 reports whether s names a 16-bit register pair.
func isR16(s string) bool {
	switch s {
	case "BC", "DE", "HL", "SP", "IX", "IY", "AF":
		return true
	}
	return false
}

// isImm reports whether s looks like an immediate value: numeric literal, hex,
// negative number, or a symbol name (used as a 16-bit address in LD rr, sym).
func isImm(s string) bool {
	if len(s) == 0 {
		return false
	}
	c := s[0]
	// Numeric literals: decimal, hex (0x...), negative
	if (c >= '0' && c <= '9') || c == '-' {
		return true
	}
	// Symbol names used as address immediates: start with a letter or underscore,
	// not enclosed in parens (those are indirect), not a known register.
	if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '_' {
		return !isR8(s) && !isR16(s) && !isCC(s)
	}
	return false
}

// isIndirect reports whether s is an (…) indirect operand.
func isIndirect(s string) bool {
	return strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")")
}

// indBase returns the contents of (X) → X.
func indBase(s string) string { return s[1 : len(s)-1] }

// tsLD handles the operands of LD instructions.
func tsLD(ops string) (int, int) {
	dst, src, _ := strings.Cut(ops, ",")
	dst = strings.TrimSpace(dst)
	src = strings.TrimSpace(src)

	// LD r, (IX+d) / LD r, (IY+d) → 19T
	if isR8(dst) && isIndirect(src) {
		b := indBase(src)
		if strings.HasPrefix(b, "IX") || strings.HasPrefix(b, "IY") {
			return 19, 0
		}
		if b == "HL" { // LD r, (HL)
			return 7, 0
		}
		if b == "BC" || b == "DE" { // LD A, (BC/DE)
			return 7, 0
		}
		return 13, 0 // LD A, (nn)
	}

	// LD (IX+d), r / LD (IY+d), r → 19T
	if isIndirect(dst) && isR8(src) {
		b := indBase(dst)
		if strings.HasPrefix(b, "IX") || strings.HasPrefix(b, "IY") {
			return 19, 0
		}
		if b == "HL" {
			return 7, 0
		}
		if b == "BC" || b == "DE" {
			return 7, 0
		}
		return 13, 0 // LD (nn), A
	}

	// LD (HL), n → 10T
	if dst == "(HL)" && isImm(src) {
		return 10, 0
	}

	// LD r, r → 4T
	if isR8(dst) && isR8(src) {
		return 4, 0
	}

	// LD r, n → 7T
	if isR8(dst) && isImm(src) {
		return 7, 0
	}

	// LD SP, HL → 6T
	if dst == "SP" && src == "HL" {
		return 6, 0
	}

	// LD (nn), HL / LD HL, (nn) → 16T
	if isIndirect(dst) && src == "HL" {
		return 16, 0
	}
	if dst == "HL" && isIndirect(src) {
		return 16, 0
	}

	// LD IX, nn / LD IY, nn → 14T
	if (dst == "IX" || dst == "IY") && isImm(src) {
		return 14, 0
	}

	// LD rr, nn → 10T (BC/DE/HL/SP)
	if isR16(dst) && isImm(src) {
		return 10, 0
	}

	// LD rr, (nn) or LD (nn), rr → 20T
	if isR16(dst) && isIndirect(src) {
		return 20, 0
	}
	if isIndirect(dst) && isR16(src) {
		return 20, 0
	}

	return 4, 0 // fallback: assume register-to-register
}

// tsADD handles ADD A,r / ADD A,n / ADD HL,rr operands.
func tsADD(ops string) (int, int) {
	dst, src, _ := strings.Cut(ops, ",")
	dst = strings.TrimSpace(dst)
	src = strings.TrimSpace(src)
	if dst == "HL" {
		return 11, 0
	}
	// ADD A, r or ADD A, n
	if isImm(src) || src == "(HL)" {
		return 7, 0
	}
	return 4, 0
}

// tsADCSBC handles ADC/SBC with A or HL as destination.
func tsADCSBC(ops string) (int, int) {
	dst, src, _ := strings.Cut(ops, ",")
	dst = strings.TrimSpace(dst)
	src = strings.TrimSpace(src)
	if dst == "HL" {
		return 15, 0
	}
	if isImm(src) || src == "(HL)" {
		return 7, 0
	}
	return 4, 0
}

// tsALU1 handles single-operand 8-bit ALU ops (SUB/AND/OR/XOR/CP).
func tsALU1(ops string) (int, int) {
	s := strings.TrimSpace(ops)
	if isImm(s) || s == "(HL)" {
		return 7, 0
	}
	return 4, 0
}

// tsINCDEC handles INC/DEC operands.
func tsINCDEC(ops string) (int, int) {
	s := strings.TrimSpace(ops)
	if isR16(s) {
		return 6, 0
	}
	if s == "(HL)" {
		return 11, 0
	}
	return 4, 0 // r8
}
