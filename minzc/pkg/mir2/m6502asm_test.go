package mir2_test

// Minimal 6502 assembler for test harness — assembles the subset of
// instructions emitted by M6502Codegen.  NOT a general-purpose assembler.

import (
	"fmt"
	"strconv"
	"strings"
)

// asm6502 assembles 6502 source text into a binary at the given origin.
// Returns the binary and a map of label → address.
// Supports only the instructions emitted by M6502Codegen.
func asm6502(src string, origin uint16) ([]byte, map[string]uint16, error) {
	a := &tinyAsm6502{
		origin: origin,
		labels: make(map[string]uint16),
	}
	return a.assemble(src)
}

type tinyAsm6502 struct {
	origin  uint16
	labels  map[string]uint16
	code    []byte
	patches []asmPatch // forward references
}

type asmPatch struct {
	offset int    // byte offset in code
	label  string // target label
	rel    bool   // true = relative branch, false = absolute
}

func (a *tinyAsm6502) pc() uint16 { return a.origin + uint16(len(a.code)) }

func (a *tinyAsm6502) emit(b ...byte) { a.code = append(a.code, b...) }

func (a *tinyAsm6502) assemble(src string) ([]byte, map[string]uint16, error) {
	lines := strings.Split(src, "\n")

	// Pass 1: collect labels, emit code with placeholder addresses
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, ";") {
			continue
		}
		// Directives
		if strings.HasPrefix(line, ".cpu") || strings.HasPrefix(line, "* =") {
			continue
		}
		// Label
		if strings.HasSuffix(line, ":") {
			label := strings.TrimSuffix(line, ":")
			a.labels[label] = a.pc()
			continue
		}
		if err := a.emitInst(line); err != nil {
			return nil, nil, fmt.Errorf("line %q: %w", line, err)
		}
	}

	// Pass 2: resolve forward references
	for _, p := range a.patches {
		addr, ok := a.labels[p.label]
		if !ok {
			return nil, nil, fmt.Errorf("undefined label %q", p.label)
		}
		if p.rel {
			// Relative branch: offset from instruction AFTER the branch (pc+2)
			from := a.origin + uint16(p.offset) + 2
			diff := int(addr) - int(from)
			if diff < -128 || diff > 127 {
				return nil, nil, fmt.Errorf("branch to %q out of range (%d)", p.label, diff)
			}
			a.code[p.offset+1] = byte(diff)
		} else {
			// Absolute address (little-endian)
			a.code[p.offset+1] = byte(addr)
			a.code[p.offset+2] = byte(addr >> 8)
		}
	}

	return a.code, a.labels, nil
}

func (a *tinyAsm6502) emitInst(line string) error {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil
	}
	mnem := strings.ToUpper(fields[0])
	operand := ""
	if len(fields) > 1 {
		operand = strings.Join(fields[1:], " ")
	}

	switch mnem {
	// ── Implied ──────────────────────────────────────────────────────────────
	case "BRK":
		a.emit(0x00)
	case "NOP":
		a.emit(0xEA)
	case "RTS":
		a.emit(0x60)
	case "CLC":
		a.emit(0x18)
	case "SEC":
		a.emit(0x38)
	case "TAX":
		a.emit(0xAA)
	case "TAY":
		a.emit(0xA8)
	case "TXA":
		a.emit(0x8A)
	case "TYA":
		a.emit(0x98)
	case "INX":
		a.emit(0xE8)
	case "INY":
		a.emit(0xC8)
	case "DEX":
		a.emit(0xCA)
	case "DEY":
		a.emit(0x88)
	case "PHA":
		a.emit(0x48)
	case "PLA":
		a.emit(0x68)

	// ── Accumulator ──────────────────────────────────────────────────────────
	case "ASL":
		if operand == "A" || operand == "" {
			a.emit(0x0A)
		} else {
			return a.emitZpOrAbs(0x06, 0x0E, operand)
		}
	case "LSR":
		if operand == "A" || operand == "" {
			a.emit(0x4A)
		} else {
			return a.emitZpOrAbs(0x46, 0x4E, operand)
		}

	// ── LDA ──────────────────────────────────────────────────────────────────
	case "LDA":
		return a.emitLoadStore(0xA9, 0xA5, 0xAD, operand)
	case "LDX":
		return a.emitLoadStore(0xA2, 0xA6, 0xAE, operand)
	case "LDY":
		return a.emitLoadStore(0xA0, 0xA4, 0xAC, operand)

	// ── STA ──────────────────────────────────────────────────────────────────
	case "STA":
		return a.emitStore(0x85, 0x8D, operand)
	case "STX":
		return a.emitStore(0x86, 0x8E, operand)
	case "STY":
		return a.emitStore(0x84, 0x8C, operand)

	// ── Arithmetic ───────────────────────────────────────────────────────────
	case "ADC":
		return a.emitALU(0x69, 0x65, 0x6D, operand)
	case "SBC":
		return a.emitALU(0xE9, 0xE5, 0xED, operand)
	case "CMP":
		return a.emitALU(0xC9, 0xC5, 0xCD, operand)
	case "CPX":
		return a.emitLoadStore(0xE0, 0xE4, 0xEC, operand)
	case "CPY":
		return a.emitLoadStore(0xC0, 0xC4, 0xCC, operand)
	case "AND":
		return a.emitALU(0x29, 0x25, 0x2D, operand)
	case "ORA":
		return a.emitALU(0x09, 0x05, 0x0D, operand)
	case "EOR":
		return a.emitALU(0x49, 0x45, 0x4D, operand)

	// ── Branches ─────────────────────────────────────────────────────────────
	case "BEQ":
		return a.emitBranch(0xF0, operand)
	case "BNE":
		return a.emitBranch(0xD0, operand)
	case "BCC":
		return a.emitBranch(0x90, operand)
	case "BCS":
		return a.emitBranch(0xB0, operand)
	case "BPL":
		return a.emitBranch(0x10, operand)
	case "BMI":
		return a.emitBranch(0x30, operand)

	// ── JMP / JSR ────────────────────────────────────────────────────────────
	case "JMP":
		return a.emitAbsRef(0x4C, operand)
	case "JSR":
		return a.emitAbsRef(0x20, operand)

	default:
		return fmt.Errorf("unknown mnemonic %q", mnem)
	}
	return nil
}

// emitLoadStore handles #imm, zp, abs for LDA/LDX/LDY/CPX/CPY
func (a *tinyAsm6502) emitLoadStore(immOp, zpOp, absOp byte, operand string) error {
	if strings.HasPrefix(operand, "#") {
		val, err := a.parseImm(operand)
		if err != nil {
			return err
		}
		a.emit(immOp, val)
		return nil
	}
	return a.emitZpOrAbs(zpOp, absOp, operand)
}

// emitALU handles #imm, zp, abs for ADC/SBC/CMP/AND/ORA/EOR
func (a *tinyAsm6502) emitALU(immOp, zpOp, absOp byte, operand string) error {
	if strings.HasPrefix(operand, "#") {
		val, err := a.parseImm(operand)
		if err != nil {
			return err
		}
		a.emit(immOp, val)
		return nil
	}
	// Register name as operand (from codegen: "ADC X") — this is not valid
	// 6502 but our codegen emits it when operand is in a register.
	// For now, treat as ZP address with the register name.
	return a.emitZpOrAbs(zpOp, absOp, operand)
}

// emitStore handles zp, abs (no immediate mode for stores)
func (a *tinyAsm6502) emitStore(zpOp, absOp byte, operand string) error {
	return a.emitZpOrAbs(zpOp, absOp, operand)
}

// emitZpOrAbs emits a zero-page or absolute instruction based on address.
func (a *tinyAsm6502) emitZpOrAbs(zpOp, absOp byte, operand string) error {
	addr, err := a.parseAddr(operand)
	if err != nil {
		return err
	}
	if addr < 0x100 {
		a.emit(zpOp, byte(addr))
	} else {
		a.emit(absOp, byte(addr), byte(addr>>8))
	}
	return nil
}

// emitBranch emits a relative branch instruction.
func (a *tinyAsm6502) emitBranch(opcode byte, operand string) error {
	label := strings.TrimPrefix(operand, ".")
	if addr, ok := a.labels[operand]; ok {
		from := a.pc() + 2
		diff := int(addr) - int(from)
		if diff < -128 || diff > 127 {
			return fmt.Errorf("branch out of range")
		}
		a.emit(opcode, byte(diff))
	} else {
		// Forward reference
		a.patches = append(a.patches, asmPatch{offset: len(a.code), label: operand, rel: true})
		a.emit(opcode, 0x00) // placeholder
	}
	_ = label
	return nil
}

// emitAbsRef emits a 3-byte instruction with an absolute address (JMP/JSR).
func (a *tinyAsm6502) emitAbsRef(opcode byte, operand string) error {
	// Try as numeric address first
	if addr, err := a.parseAddr(operand); err == nil {
		a.emit(opcode, byte(addr), byte(addr>>8))
		return nil
	}
	// Label reference
	if addr, ok := a.labels[operand]; ok {
		a.emit(opcode, byte(addr), byte(addr>>8))
	} else {
		a.patches = append(a.patches, asmPatch{offset: len(a.code), label: operand, rel: false})
		a.emit(opcode, 0x00, 0x00) // placeholder
	}
	return nil
}

func (a *tinyAsm6502) parseImm(s string) (byte, error) {
	s = strings.TrimPrefix(s, "#")
	return a.parseByte(s)
}

func (a *tinyAsm6502) parseByte(s string) (byte, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "$") {
		v, err := strconv.ParseUint(s[1:], 16, 8)
		return byte(v), err
	}
	v, err := strconv.ParseUint(s, 10, 8)
	return byte(v), err
}

func (a *tinyAsm6502) parseAddr(s string) (uint16, error) {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(s, "$") {
		v, err := strconv.ParseUint(s[1:], 16, 16)
		return uint16(v), err
	}
	// Try as decimal
	v, err := strconv.ParseUint(s, 10, 16)
	if err == nil {
		return uint16(v), nil
	}
	// Could be a ZP variable name from codegen (zp2, zp3, etc.)
	if strings.HasPrefix(s, "zp") {
		numStr := strings.TrimPrefix(s, "zp")
		n, err := strconv.ParseUint(numStr, 10, 8)
		if err == nil {
			return uint16(n), nil
		}
	}
	return 0, fmt.Errorf("cannot parse address %q", s)
}
