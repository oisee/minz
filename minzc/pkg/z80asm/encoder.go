package z80asm

import (
	"fmt"
)

// Virtual instruction encoders for sjasmplus compatibility

// encodeVirtualLD16 encodes LD HL, DE as LD H, D : LD L, E
func encodeVirtualLD16(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	if len(line.Operands) != 2 {
		return nil, fmt.Errorf("LD expects 2 operands")
	}
	
	dst := line.Operands[0]
	src := line.Operands[1]
	
	// Only handle specific register pairs for now
	if dst == "HL" && src == "DE" {
		return []byte{0x62, 0x6B}, nil // LD H, D : LD L, E
	} else if dst == "HL" && src == "BC" {
		return []byte{0x60, 0x69}, nil // LD H, B : LD L, C
	} else if dst == "DE" && src == "HL" {
		return []byte{0x54, 0x5D}, nil // LD D, H : LD E, L
	} else if dst == "BC" && src == "HL" {
		return []byte{0x44, 0x4D}, nil // LD B, H : LD C, L
	}
	
	return nil, fmt.Errorf("unsupported virtual LD %s, %s", dst, src)
}

// encodeVirtualShiftTriple encodes SRL A, A, A as SRL A : SRL A : SRL A
func encodeVirtualShiftTriple(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	if len(line.Operands) != 3 {
		return nil, fmt.Errorf("virtual SRL expects 3 operands")
	}
	
	// All operands should be the same register
	reg := line.Operands[0]
	if line.Operands[1] != reg || line.Operands[2] != reg {
		return nil, fmt.Errorf("virtual SRL operands must be the same register")
	}
	
	// Get register encoding (A=7, B=0, C=1, etc.)
	regCode, ok := getRegisterCode(reg)
	if !ok {
		return nil, fmt.Errorf("invalid register: %s", reg)
	}
	
	// SRL r = CB 38+r (repeat 3 times)
	opcode := 0x38 + regCode
	return []byte{0xCB, opcode, 0xCB, opcode, 0xCB, opcode}, nil
}

// getRegisterCode returns the 3-bit encoding for 8-bit registers
func getRegisterCode(reg string) (byte, bool) {
	switch reg {
	case "B": return 0, true
	case "C": return 1, true  
	case "D": return 2, true
	case "E": return 3, true
	case "H": return 4, true
	case "L": return 5, true
	case "(HL)": return 6, true
	case "A": return 7, true
	default: return 0, false
	}
}

// Standard encoders for common instruction patterns

// encodeNoOperand handles instructions with no operands (NOP, HALT, etc.)
func encodeNoOperand(opcode byte) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		return []byte{opcode}, nil
	}
}

// encodeImplied handles single-byte instructions like RET, EI, DI
func encodeImplied(opcode byte) EncoderFunc {
	return encodeNoOperand(opcode)
}

// encodeCBPrefix handles CB-prefixed instructions
func encodeCBPrefix(opcode byte) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		return []byte{PrefixCB, opcode}, nil
	}
}

// encodeEDPrefix handles ED-prefixed instructions
func encodeEDPrefix(opcode byte) EncoderFunc {
	return func(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
		return []byte{PrefixED, opcode}, nil
	}
}

// encodeReg8ToA handles LD A, r instructions
func encodeReg8ToA(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	if len(line.Operands) != 2 {
		return nil, fmt.Errorf("LD requires 2 operands")
	}
	
	// Parse source register
	srcReg, ok := parseRegister(line.Operands[1])
	if !ok {
		return nil, fmt.Errorf("invalid source register: %s", line.Operands[1])
	}
	
	// Encode based on source register
	var opcode byte
	switch srcReg {
	case RegB:
		opcode = 0x78
	case RegC:
		opcode = 0x79
	case RegD:
		opcode = 0x7A
	case RegE:
		opcode = 0x7B
	case RegH:
		opcode = 0x7C
	case RegL:
		opcode = 0x7D
	case RegA:
		opcode = 0x7F
	default:
		return nil, fmt.Errorf("invalid source register for LD A, r: %v", srcReg)
	}
	
	return []byte{opcode}, nil
}

// encodeLD handles various LD instructions
func encodeLD(a *Assembler, line *Line, def *InstructionDef) ([]byte, error) {
	if len(line.Operands) != 2 {
		return nil, fmt.Errorf("LD requires 2 operands")
	}
	
	dest := line.Operands[0]
	src := line.Operands[1]
	
	// Try to parse as registers
	destReg, destIsReg := parseRegister(dest)
	srcReg, srcIsReg := parseRegister(src)
	
	// Handle register to register moves
	if destIsReg && srcIsReg {
		return encodeLDRegReg(destReg, srcReg)
	}
	
	// Handle immediate loads
	if destIsReg && !srcIsReg {
		return encodeLDRegImm(a, destReg, src)
	}
	
	// Handle indirect addressing
	if isIndirect(dest) || isIndirect(src) {
		return encodeLDIndirect(a, dest, src)
	}
	
	// Handle memory operations
	if !destIsReg && !srcIsReg {
		return encodeLDMemory(a, dest, src)
	}
	
	return nil, fmt.Errorf("unsupported LD operation: %s, %s", dest, src)
}

// encodeLDRegReg handles register to register loads
func encodeLDRegReg(dest, src Register) ([]byte, error) {
	// 8-bit register to register
	if isReg8(dest) && isReg8(src) {
		destCode, err := encodeReg8(dest)
		if err != nil {
			return nil, err
		}
		srcCode, err := encodeReg8(src)
		if err != nil {
			return nil, err
		}
		
		// LD r, r' = 01 ddd sss
		opcode := 0x40 | (destCode << 3) | srcCode
		return []byte{opcode}, nil
	}
	
	// Special cases for 16-bit loads
	if dest == RegSP && src == RegHL {
		return []byte{0xF9}, nil // LD SP, HL
	}
	
	// Handle IX/IY half registers (undocumented)
	if dest == RegIXH || dest == RegIXL || src == RegIXH || src == RegIXL {
		return encodeIXHalfReg(dest, src)
	}
	if dest == RegIYH || dest == RegIYL || src == RegIYH || src == RegIYL {
		return encodeIYHalfReg(dest, src)
	}
	
	return nil, fmt.Errorf("unsupported register combination: %v, %v", dest, src)
}

// encodeLDRegImm handles immediate loads to registers
func encodeLDRegImm(a *Assembler, dest Register, immStr string) ([]byte, error) {
	value, err := a.resolveValue(immStr)
	if err != nil {
		return nil, err
	}
	
	// 8-bit immediate loads
	if isReg8(dest) {
		var opcode byte
		switch dest {
		case RegA:
			opcode = 0x3E
		case RegB:
			opcode = 0x06
		case RegC:
			opcode = 0x0E
		case RegD:
			opcode = 0x16
		case RegE:
			opcode = 0x1E
		case RegH:
			opcode = 0x26
		case RegL:
			opcode = 0x2E
		// Undocumented IX/IY halves
		case RegIXH:
			return []byte{0xDD, 0x26, byte(value)}, nil
		case RegIXL:
			return []byte{0xDD, 0x2E, byte(value)}, nil
		case RegIYH:
			return []byte{0xFD, 0x26, byte(value)}, nil
		case RegIYL:
			return []byte{0xFD, 0x2E, byte(value)}, nil
		default:
			return nil, fmt.Errorf("invalid destination register for immediate: %v", dest)
		}
		return []byte{opcode, byte(value)}, nil
	}
	
	// 16-bit immediate loads
	if isReg16(dest) {
		var opcode byte
		switch dest {
		case RegBC:
			opcode = 0x01
		case RegDE:
			opcode = 0x11
		case RegHL:
			opcode = 0x21
		case RegSP:
			opcode = 0x31
		case RegIX:
			return []byte{0xDD, 0x21, byte(value), byte(value >> 8)}, nil
		case RegIY:
			return []byte{0xFD, 0x21, byte(value), byte(value >> 8)}, nil
		default:
			return nil, fmt.Errorf("invalid 16-bit destination register: %v", dest)
		}
		return []byte{opcode, byte(value), byte(value >> 8)}, nil
	}
	
	return nil, fmt.Errorf("invalid destination register type: %v", dest)
}

// encodeLDIndirect handles indirect addressing modes
func encodeLDIndirect(a *Assembler, dest, src string) ([]byte, error) {
	// LD (HL), r
	if dest == "(HL)" {
		srcReg, ok := parseRegister(src)
		if ok && isReg8(srcReg) {
			srcCode, err := encodeReg8(srcReg)
			if err != nil {
				return nil, err
			}
			opcode := 0x70 | srcCode
			return []byte{opcode}, nil
		}
		// LD (HL), n
		if !ok {
			value, err := a.resolveValue(src)
			if err != nil {
				return nil, err
			}
			return []byte{0x36, byte(value)}, nil
		}
	}
	
	// LD r, (HL)
	if src == "(HL)" {
		destReg, ok := parseRegister(dest)
		if ok && isReg8(destReg) {
			destCode, err := encodeReg8(destReg)
			if err != nil {
				return nil, err
			}
			opcode := 0x46 | (destCode << 3)
			return []byte{opcode}, nil
		}
	}
	
	// LD A, (BC) or LD A, (DE)
	if dest == "A" {
		if src == "(BC)" {
			return []byte{0x0A}, nil
		}
		if src == "(DE)" {
			return []byte{0x1A}, nil
		}
	}
	
	// LD (BC), A or LD (DE), A
	if src == "A" {
		if dest == "(BC)" {
			return []byte{0x02}, nil
		}
		if dest == "(DE)" {
			return []byte{0x12}, nil
		}
	}
	
	// Handle IX/IY offset addressing
	if isIndexedOperand(dest, "IX") || isIndexedOperand(src, "IX") {
		return encodeIXOffset(a, dest, src)
	}
	if isIndexedOperand(dest, "IY") || isIndexedOperand(src, "IY") {
		return encodeIYOffset(a, dest, src)
	}
	
	return nil, fmt.Errorf("unsupported indirect addressing: %s, %s", dest, src)
}

// encodeLDMemory handles direct memory operations
func encodeLDMemory(a *Assembler, dest, src string) ([]byte, error) {
	// Check if dest is memory address
	if isIndirect(dest) {
		addr, err := a.resolveValue(stripIndirect(dest))
		if err != nil {
			return nil, err
		}
		
		// LD (nn), A
		if src == "A" {
			return []byte{0x32, byte(addr), byte(addr >> 8)}, nil
		}
		// LD (nn), HL
		if src == "HL" {
			return []byte{0x22, byte(addr), byte(addr >> 8)}, nil
		}
		// LD (nn), BC/DE/SP (ED prefix)
		srcReg, ok := parseRegister(src)
		if ok {
			switch srcReg {
			case RegBC:
				return []byte{0xED, 0x43, byte(addr), byte(addr >> 8)}, nil
			case RegDE:
				return []byte{0xED, 0x53, byte(addr), byte(addr >> 8)}, nil
			case RegSP:
				return []byte{0xED, 0x73, byte(addr), byte(addr >> 8)}, nil
			case RegIX:
				return []byte{0xDD, 0x22, byte(addr), byte(addr >> 8)}, nil
			case RegIY:
				return []byte{0xFD, 0x22, byte(addr), byte(addr >> 8)}, nil
			}
		}
	}
	
	// Check if src is memory address
	if isIndirect(src) {
		addr, err := a.resolveValue(stripIndirect(src))
		if err != nil {
			return nil, err
		}
		
		// LD A, (nn)
		if dest == "A" {
			return []byte{0x3A, byte(addr), byte(addr >> 8)}, nil
		}
		// LD HL, (nn)
		if dest == "HL" {
			return []byte{0x2A, byte(addr), byte(addr >> 8)}, nil
		}
		// LD BC/DE/SP, (nn) (ED prefix)
		destReg, ok := parseRegister(dest)
		if ok {
			switch destReg {
			case RegBC:
				return []byte{0xED, 0x4B, byte(addr), byte(addr >> 8)}, nil
			case RegDE:
				return []byte{0xED, 0x5B, byte(addr), byte(addr >> 8)}, nil
			case RegSP:
				return []byte{0xED, 0x7B, byte(addr), byte(addr >> 8)}, nil
			case RegIX:
				return []byte{0xDD, 0x2A, byte(addr), byte(addr >> 8)}, nil
			case RegIY:
				return []byte{0xFD, 0x2A, byte(addr), byte(addr >> 8)}, nil
			}
		}
	}
	
	return nil, fmt.Errorf("unsupported memory operation: %s, %s", dest, src)
}

// resolveValue resolves an operand to a numeric value
func (a *Assembler) resolveValue(operand string) (uint16, error) {
	// Try to parse as number first
	if val, err := parseNumber(operand); err == nil {
		return val, nil
	}
	
	// Try to resolve as symbol
	return a.resolveSymbol(operand)
}

// Undocumented instruction encoders

// encodeIXHalfReg handles undocumented IX half-register operations
func encodeIXHalfReg(dest, src Register) ([]byte, error) {
	// Map IX half registers to H/L equivalents
	var destCode, srcCode byte
	var err error
	
	switch dest {
	case RegIXH:
		destCode = 0x04 // H position
	case RegIXL:
		destCode = 0x05 // L position
	default:
		destCode, err = encodeReg8(dest)
		if err != nil {
			return nil, err
		}
	}
	
	switch src {
	case RegIXH:
		srcCode = 0x04 // H position
	case RegIXL:
		srcCode = 0x05 // L position
	default:
		srcCode, err = encodeReg8(src)
		if err != nil {
			return nil, err
		}
	}
	
	// DD prefix + modified opcode
	opcode := 0x40 | (destCode << 3) | srcCode
	return []byte{0xDD, opcode}, nil
}

// encodeIYHalfReg handles undocumented IY half-register operations
func encodeIYHalfReg(dest, src Register) ([]byte, error) {
	// Map IY half registers to H/L equivalents
	var destCode, srcCode byte
	var err error
	
	switch dest {
	case RegIYH:
		destCode = 0x04 // H position
	case RegIYL:
		destCode = 0x05 // L position
	default:
		destCode, err = encodeReg8(dest)
		if err != nil {
			return nil, err
		}
	}
	
	switch src {
	case RegIYH:
		srcCode = 0x04 // H position
	case RegIYL:
		srcCode = 0x05 // L position
	default:
		srcCode, err = encodeReg8(src)
		if err != nil {
			return nil, err
		}
	}
	
	// FD prefix + modified opcode
	opcode := 0x40 | (destCode << 3) | srcCode
	return []byte{0xFD, opcode}, nil
}

// encodeIXOffset handles (IX+d) addressing
func encodeIXOffset(a *Assembler, dest, src string) ([]byte, error) {
	// Extract offset
	var offset int8
	var err error
	
	if isIndexedOperand(dest, "IX") {
		offset, err = getIndexOffset(dest)
		if err != nil {
			return nil, err
		}
		
		// LD (IX+d), r
		srcReg, ok := parseRegister(src)
		if ok && isReg8(srcReg) {
			srcCode, err := encodeReg8(srcReg)
			if err != nil {
				return nil, err
			}
			opcode := 0x70 | srcCode
			return []byte{0xDD, opcode, byte(offset)}, nil
		}
		
		// LD (IX+d), n
		if !ok {
			value, err := a.resolveValue(src)
			if err != nil {
				return nil, err
			}
			return []byte{0xDD, 0x36, byte(offset), byte(value)}, nil
		}
	}
	
	if isIndexedOperand(src, "IX") {
		offset, err = getIndexOffset(src)
		if err != nil {
			return nil, err
		}
		
		// LD r, (IX+d)
		destReg, ok := parseRegister(dest)
		if ok && isReg8(destReg) {
			destCode, err := encodeReg8(destReg)
			if err != nil {
				return nil, err
			}
			opcode := 0x46 | (destCode << 3)
			return []byte{0xDD, opcode, byte(offset)}, nil
		}
	}
	
	return nil, fmt.Errorf("invalid IX offset operation")
}

// encodeIYOffset handles (IY+d) addressing
func encodeIYOffset(a *Assembler, dest, src string) ([]byte, error) {
	// Extract offset
	var offset int8
	var err error
	
	if isIndexedOperand(dest, "IY") {
		offset, err = getIndexOffset(dest)
		if err != nil {
			return nil, err
		}
		
		// LD (IY+d), r
		srcReg, ok := parseRegister(src)
		if ok && isReg8(srcReg) {
			srcCode, err := encodeReg8(srcReg)
			if err != nil {
				return nil, err
			}
			opcode := 0x70 | srcCode
			return []byte{0xFD, opcode, byte(offset)}, nil
		}
		
		// LD (IY+d), n
		if !ok {
			value, err := a.resolveValue(src)
			if err != nil {
				return nil, err
			}
			return []byte{0xFD, 0x36, byte(offset), byte(value)}, nil
		}
	}
	
	if isIndexedOperand(src, "IY") {
		offset, err = getIndexOffset(src)
		if err != nil {
			return nil, err
		}
		
		// LD r, (IY+d)
		destReg, ok := parseRegister(dest)
		if ok && isReg8(destReg) {
			destCode, err := encodeReg8(destReg)
			if err != nil {
				return nil, err
			}
			opcode := 0x46 | (destCode << 3)
			return []byte{0xFD, opcode, byte(offset)}, nil
		}
	}
	
	return nil, fmt.Errorf("invalid IY offset operation")
}