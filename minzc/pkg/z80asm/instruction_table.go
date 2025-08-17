package z80asm

import (
	"fmt"
	"strings"
)

// OpType defines the type of operand for table-driven encoding
type OpType int

const (
	OpTypeNone      OpType = iota
	OpTypeReg8             // A, B, C, D, E, H, L
	OpTypeReg16            // BC, DE, HL, SP, IX, IY, AF
	OpTypeImm8             // n (8-bit immediate)
	OpTypeImm16            // nn (16-bit immediate)
	OpTypeIndReg           // (HL), (BC), (DE), (SP)
	OpTypeIndImm           // (nn) - memory address
	OpTypeIndIdx           // (IX+d), (IY+d)
	OpTypeRelative         // e (relative jump)
	OpTypeCondition        // NZ, Z, NC, C, PO, PE, P, M
	OpTypeBit              // 0-7 (bit number)
)

// InstructionPattern defines a pattern for matching and encoding instructions
type InstructionPattern struct {
	Mnemonic     string
	Operands     []OperandPattern
	Encoding     []byte
	EncodingFunc func(*Assembler, *InstructionPattern, []interface{}) ([]byte, error)
	Size         int // Size in bytes (0 = calculate from encoding)
	Cycles       int // Clock cycles
}

// OperandPattern defines the pattern for a single operand
type OperandPattern struct {
	Type       OpType
	Constraint string // Specific value required (e.g., "A", "HL", etc.)
}

// Global instruction table - will be populated with all patterns
var instructionTable []InstructionPattern

// Initialize instruction table with all Z80 instructions
func init() {
	instructionTable = append(instructionTable, ldInstructions...)
	instructionTable = append(instructionTable, jpInstructions...)
	instructionTable = append(instructionTable, arithmeticInstructions...)
	instructionTable = append(instructionTable, stackInstructions...)
	instructionTable = append(instructionTable, bitInstructions...)
	instructionTable = append(instructionTable, miscInstructions...)
}

// LD instruction patterns - comprehensive coverage
var ldInstructions = []InstructionPattern{
	// ========== 8-bit register to register ==========
	// LD A, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x7F}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x78}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x79}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x7A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x7B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x7C}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x7D}},
	
	// LD B, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x47}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x40}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x41}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x42}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x43}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x44}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x45}},
	
	// LD C, r  
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x4F}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x48}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x49}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x4A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x4B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x4C}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x4D}},
	
	// LD D, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x57}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x50}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x51}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x52}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x53}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x54}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x55}},
	
	// LD E, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x5F}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x58}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x59}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x5A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x5B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x5C}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x5D}},
	
	// LD H, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x67}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x60}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x61}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x62}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x63}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x64}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x65}},
	
	// LD L, r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x6F}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x68}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x69}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x6A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x6B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x6C}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x6D}},
	
	// ========== 8-bit immediate loads ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x3E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x06}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x0E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x16}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x1E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x26}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x2E}},
	
	// ========== (HL) operations ==========
	// LD r, (HL)
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x7E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x46}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x4E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x56}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x5E}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x66}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x6E}},
	
	// LD (HL), r
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x77}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x70}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x71}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x72}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x73}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x74}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x75}},
	
	// LD (HL), n
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}, {OpTypeImm8, ""}}, EncodingFunc: encodeLD8Imm, Encoding: []byte{0x36}},
	
	// ========== (BC), (DE) operations ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(BC)"}}, Encoding: []byte{0x0A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(DE)"}}, Encoding: []byte{0x1A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(BC)"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x02}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndReg, "(DE)"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x12}},
	
	// ========== Direct memory operations ==========
	// LD A, (nn)
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0x3A}},
	// LD (nn), A
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg8, "A"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0x32}},
	
	// ========== 16-bit immediate loads ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "BC"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0x01}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "DE"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0x11}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0x21}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "SP"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0x31}},
	
	// IX/IY immediate loads
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "IX"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0xDD, 0x21}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "IY"}, {OpTypeImm16, ""}}, EncodingFunc: encodeLD16Imm, Encoding: []byte{0xFD, 0x21}},
	
	// ========== 16-bit memory operations ==========
	// LD HL, (nn)
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "HL"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0x2A}},
	// LD (nn), HL
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "HL"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0x22}},
	
	// IX/IY memory operations
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "IX"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xDD, 0x2A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "IX"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xDD, 0x22}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "IY"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xFD, 0x2A}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "IY"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xFD, 0x22}},
	
	// ED-prefixed 16-bit memory operations
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "BC"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x4B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "DE"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x5B}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "SP"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x7B}},
	
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "BC"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x43}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "DE"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x53}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg16, "SP"}}, EncodingFunc: encodeLDMemDirect, Encoding: []byte{0xED, 0x73}},
	
	// ========== Special 16-bit transfers ==========
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "SP"}, {OpTypeReg16, "HL"}}, Encoding: []byte{0xF9}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "SP"}, {OpTypeReg16, "IX"}}, Encoding: []byte{0xDD, 0xF9}},
	{Mnemonic: "LD", Operands: []OperandPattern{{OpTypeReg16, "SP"}, {OpTypeReg16, "IY"}}, Encoding: []byte{0xFD, 0xF9}},
}

// Encoding functions for LD instructions
func encodeLD8Imm(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)
	
	// Find immediate value
	for _, v := range values {
		if val, ok := v.(uint8); ok {
			result = append(result, val)
			return result, nil
		}
		if val, ok := v.(uint16); ok && val <= 0xFF {
			result = append(result, byte(val))
			return result, nil
		}
	}
	
	return nil, fmt.Errorf("no immediate value found")
}

func encodeLD16Imm(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)
	
	// Find immediate value
	for _, v := range values {
		if val, ok := v.(uint16); ok {
			// Little-endian encoding
			result = append(result, byte(val), byte(val>>8))
			return result, nil
		}
	}
	
	return nil, fmt.Errorf("no immediate value found")
}

func encodeLDMemDirect(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)
	
	// Find address value
	for _, v := range values {
		if addr, ok := v.(uint16); ok {
			// Little-endian encoding
			result = append(result, byte(addr), byte(addr>>8))
			return result, nil
		}
	}
	
	return nil, fmt.Errorf("no address value found")
}

// JP/JR instruction patterns
var jpInstructions = []InstructionPattern{
	// Unconditional jumps
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xC3}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0xE9}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeIndReg, "(IX)"}}, Encoding: []byte{0xDD, 0xE9}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeIndReg, "(IY)"}}, Encoding: []byte{0xFD, 0xE9}},
	
	// Conditional jumps
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "NZ"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xC2}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "Z"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xCA}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "NC"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xD2}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "C"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xDA}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "PO"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xE2}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "PE"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xEA}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "P"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xF2}},
	{Mnemonic: "JP", Operands: []OperandPattern{{OpTypeCondition, "M"}, {OpTypeImm16, ""}}, EncodingFunc: encodeJPImm, Encoding: []byte{0xFA}},
	
	// Relative jumps
	{Mnemonic: "JR", Operands: []OperandPattern{{OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x18}},
	{Mnemonic: "JR", Operands: []OperandPattern{{OpTypeCondition, "NZ"}, {OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x20}},
	{Mnemonic: "JR", Operands: []OperandPattern{{OpTypeCondition, "Z"}, {OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x28}},
	{Mnemonic: "JR", Operands: []OperandPattern{{OpTypeCondition, "NC"}, {OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x30}},
	{Mnemonic: "JR", Operands: []OperandPattern{{OpTypeCondition, "C"}, {OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x38}},
	
	// DJNZ
	{Mnemonic: "DJNZ", Operands: []OperandPattern{{OpTypeRelative, ""}}, EncodingFunc: encodeJRRel, Encoding: []byte{0x10}},
	
	// CALL instructions
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xCD}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "NZ"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xC4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "Z"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xCC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "NC"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xD4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "C"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xDC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "PO"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xE4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "PE"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xEC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "P"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xF4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "M"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xFC}},
	
	// RET instructions
	{Mnemonic: "RET", Operands: []OperandPattern{}, Encoding: []byte{0xC9}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "NZ"}}, Encoding: []byte{0xC0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "Z"}}, Encoding: []byte{0xC8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "NC"}}, Encoding: []byte{0xD0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "C"}}, Encoding: []byte{0xD8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "PO"}}, Encoding: []byte{0xE0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "PE"}}, Encoding: []byte{0xE8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "P"}}, Encoding: []byte{0xF0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "M"}}, Encoding: []byte{0xF8}},
	{Mnemonic: "RETI", Operands: []OperandPattern{}, Encoding: []byte{0xED, 0x4D}},
	{Mnemonic: "RETN", Operands: []OperandPattern{}, Encoding: []byte{0xED, 0x45}},
	
	// RST instructions - use custom encoding function to handle any number base
	{Mnemonic: "RST", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeRST},
	
	// I/O instructions
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndImm, ""}}, EncodingFunc: encodeIN},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x78}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "B"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x40}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "C"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x48}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "D"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x50}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "E"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x58}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "H"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x60}},
	{Mnemonic: "IN", Operands: []OperandPattern{{OpTypeReg8, "L"}, {OpTypeIndReg, "(C)"}}, Encoding: []byte{0xED, 0x68}},
	
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndImm, ""}, {OpTypeReg8, "A"}}, EncodingFunc: encodeOUT},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "A"}}, Encoding: []byte{0xED, 0x79}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "B"}}, Encoding: []byte{0xED, 0x41}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "C"}}, Encoding: []byte{0xED, 0x49}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "D"}}, Encoding: []byte{0xED, 0x51}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "E"}}, Encoding: []byte{0xED, 0x59}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "H"}}, Encoding: []byte{0xED, 0x61}},
	{Mnemonic: "OUT", Operands: []OperandPattern{{OpTypeIndReg, "(C)"}, {OpTypeReg8, "L"}}, Encoding: []byte{0xED, 0x69}},
}

func encodeRST(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	// RST n format - n must be 0, 8, 16, 24, 32, 40, 48, or 56
	// These map to opcodes C7, CF, D7, DF, E7, EF, F7, FF
	
	// Find the vector value
	var vector uint16
	found := false
	for _, v := range values {
		switch val := v.(type) {
		case uint8:
			vector = uint16(val)
			found = true
		case uint16:
			vector = val
			found = true
		}
	}
	
	if !found {
		return nil, fmt.Errorf("RST requires a vector operand")
	}
	
	// Map vector to opcode
	validVectors := map[uint16]byte{
		0x00: 0xC7, 0x08: 0xCF, 0x10: 0xD7, 0x18: 0xDF,
		0x20: 0xE7, 0x28: 0xEF, 0x30: 0xF7, 0x38: 0xFF,
	}
	
	if opcode, ok := validVectors[vector]; ok {
		return []byte{opcode}, nil
	}
	
	return nil, fmt.Errorf("invalid RST vector: %d ($%02X) - valid vectors are 0, 8, 16 ($10), 24 ($18), 32 ($20), 40 ($28), 48 ($30), 56 ($38)", vector, vector)
}

func encodeIN(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	// IN A, (n) format
	result := []byte{0xDB} // IN opcode
	
	// Find port number
	for _, v := range values {
		switch val := v.(type) {
		case uint8:
			result = append(result, val)
			return result, nil
		case uint16:
			if val > 255 {
				return nil, fmt.Errorf("port number out of range: %d", val)
			}
			result = append(result, byte(val))
			return result, nil
		}
	}
	
	return nil, fmt.Errorf("no port number found")
}

func encodeOUT(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	// OUT (n), A format
	result := []byte{0xD3} // OUT opcode
	
	// Find port number
	for _, v := range values {
		switch val := v.(type) {
		case uint8:
			result = append(result, val)
			return result, nil
		case uint16:
			if val > 255 {
				return nil, fmt.Errorf("port number out of range: %d", val)
			}
			result = append(result, byte(val))
			return result, nil
		}
	}
	
	return nil, fmt.Errorf("no port number found")
}

func encodeJPImm(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)
	
	// Find address value
	for _, v := range values {
		if addr, ok := v.(uint16); ok {
			result = append(result, byte(addr), byte(addr>>8))
			return result, nil
		}
	}
	
	return nil, fmt.Errorf("no address value found")
}

func encodeJRRel(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)
	
	// Find relative offset
	for _, v := range values {
		if offset, ok := v.(int8); ok {
			result = append(result, byte(offset))
			return result, nil
		}
		// Also accept calculated offset from label
		if addr, ok := v.(uint16); ok {
			// Calculate relative offset from current position
			offset := int(addr) - int(a.currentAddr) - 2
			if offset < -128 || offset > 127 {
				return nil, fmt.Errorf("relative jump out of range: %d", offset)
			}
			result = append(result, byte(offset))
			return result, nil
		}
	}
	
	return nil, fmt.Errorf("no relative offset found")
}

// Arithmetic instruction patterns
var arithmeticInstructions = []InstructionPattern{
	// ADD A, r
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "A"}}, Encoding: []byte{0x87}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "B"}}, Encoding: []byte{0x80}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "C"}}, Encoding: []byte{0x81}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "D"}}, Encoding: []byte{0x82}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "E"}}, Encoding: []byte{0x83}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "H"}}, Encoding: []byte{0x84}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeReg8, "L"}}, Encoding: []byte{0x85}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x86}},
	{Mnemonic: "ADD", Operands: []OperandPattern{{OpTypeReg8, "A"}, {OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xC6}},
	
	// SUB r
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0x97}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0x90}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0x91}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0x92}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0x93}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0x94}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0x95}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x96}},
	{Mnemonic: "SUB", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xD6}},
	
	// INC/DEC
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0x3C}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0x04}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0x0C}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0x14}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0x1C}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0x24}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0x2C}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x34}},
	
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0x3D}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0x05}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0x0D}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0x15}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0x1D}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0x25}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0x2D}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0x35}},
	
	// 16-bit INC/DEC
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg16, "BC"}}, Encoding: []byte{0x03}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg16, "DE"}}, Encoding: []byte{0x13}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg16, "HL"}}, Encoding: []byte{0x23}},
	{Mnemonic: "INC", Operands: []OperandPattern{{OpTypeReg16, "SP"}}, Encoding: []byte{0x33}},
	
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg16, "BC"}}, Encoding: []byte{0x0B}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg16, "DE"}}, Encoding: []byte{0x1B}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg16, "HL"}}, Encoding: []byte{0x2B}},
	{Mnemonic: "DEC", Operands: []OperandPattern{{OpTypeReg16, "SP"}}, Encoding: []byte{0x3B}},
}

func encodeArithImm(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)
	
	// Find immediate value
	for _, v := range values {
		if val, ok := v.(uint8); ok {
			result = append(result, val)
			return result, nil
		}
		if val, ok := v.(uint16); ok && val <= 0xFF {
			result = append(result, byte(val))
			return result, nil
		}
	}
	
	return nil, fmt.Errorf("no immediate value found")
}

// Stack instruction patterns
var stackInstructions = []InstructionPattern{
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "AF"}}, Encoding: []byte{0xF5}},
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "BC"}}, Encoding: []byte{0xC5}},
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "DE"}}, Encoding: []byte{0xD5}},
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "HL"}}, Encoding: []byte{0xE5}},
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "IX"}}, Encoding: []byte{0xDD, 0xE5}},
	{Mnemonic: "PUSH", Operands: []OperandPattern{{OpTypeReg16, "IY"}}, Encoding: []byte{0xFD, 0xE5}},
	
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "AF"}}, Encoding: []byte{0xF1}},
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "BC"}}, Encoding: []byte{0xC1}},
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "DE"}}, Encoding: []byte{0xD1}},
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "HL"}}, Encoding: []byte{0xE1}},
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "IX"}}, Encoding: []byte{0xDD, 0xE1}},
	{Mnemonic: "POP", Operands: []OperandPattern{{OpTypeReg16, "IY"}}, Encoding: []byte{0xFD, 0xE1}},
}

// Bit operation patterns
var bitInstructions = []InstructionPattern{
	// AND
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0xA7}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0xA0}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0xA1}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0xA2}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0xA3}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0xA4}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0xA5}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0xA6}},
	{Mnemonic: "AND", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xE6}},
	
	// OR
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0xB7}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0xB0}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0xB1}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0xB2}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0xB3}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0xB4}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0xB5}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0xB6}},
	{Mnemonic: "OR", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xF6}},
	
	// XOR
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0xAF}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0xA8}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0xA9}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0xAA}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0xAB}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0xAC}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0xAD}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0xAE}},
	{Mnemonic: "XOR", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xEE}},
	
	// CP
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "A"}}, Encoding: []byte{0xBF}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "B"}}, Encoding: []byte{0xB8}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "C"}}, Encoding: []byte{0xB9}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "D"}}, Encoding: []byte{0xBA}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "E"}}, Encoding: []byte{0xBB}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "H"}}, Encoding: []byte{0xBC}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeReg8, "L"}}, Encoding: []byte{0xBD}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeIndReg, "(HL)"}}, Encoding: []byte{0xBE}},
	{Mnemonic: "CP", Operands: []OperandPattern{{OpTypeImm8, ""}}, EncodingFunc: encodeArithImm, Encoding: []byte{0xFE}},
}

// Miscellaneous instruction patterns
var miscInstructions = []InstructionPattern{
	// Basic control
	{Mnemonic: "NOP", Operands: []OperandPattern{}, Encoding: []byte{0x00}},
	{Mnemonic: "HALT", Operands: []OperandPattern{}, Encoding: []byte{0x76}},
	{Mnemonic: "DI", Operands: []OperandPattern{}, Encoding: []byte{0xF3}},
	{Mnemonic: "EI", Operands: []OperandPattern{}, Encoding: []byte{0xFB}},
	
	// Returns
	{Mnemonic: "RET", Operands: []OperandPattern{}, Encoding: []byte{0xC9}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "NZ"}}, Encoding: []byte{0xC0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "Z"}}, Encoding: []byte{0xC8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "NC"}}, Encoding: []byte{0xD0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "C"}}, Encoding: []byte{0xD8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "PO"}}, Encoding: []byte{0xE0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "PE"}}, Encoding: []byte{0xE8}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "P"}}, Encoding: []byte{0xF0}},
	{Mnemonic: "RET", Operands: []OperandPattern{{OpTypeCondition, "M"}}, Encoding: []byte{0xF8}},
	
	// Calls
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xCD}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "NZ"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xC4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "Z"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xCC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "NC"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xD4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "C"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xDC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "PO"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xE4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "PE"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xEC}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "P"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xF4}},
	{Mnemonic: "CALL", Operands: []OperandPattern{{OpTypeCondition, "M"}, {OpTypeImm16, ""}}, EncodingFunc: encodeCALL, Encoding: []byte{0xFC}},
	
	// Exchanges
	{Mnemonic: "EX", Operands: []OperandPattern{{OpTypeReg16, "DE"}, {OpTypeReg16, "HL"}}, Encoding: []byte{0xEB}},
	{Mnemonic: "EX", Operands: []OperandPattern{{OpTypeReg16, "AF"}, {OpTypeReg16, "AF'"}}, Encoding: []byte{0x08}},
	{Mnemonic: "EX", Operands: []OperandPattern{{OpTypeIndReg, "(SP)"}, {OpTypeReg16, "HL"}}, Encoding: []byte{0xE3}},
	{Mnemonic: "EX", Operands: []OperandPattern{{OpTypeIndReg, "(SP)"}, {OpTypeReg16, "IX"}}, Encoding: []byte{0xDD, 0xE3}},
	{Mnemonic: "EX", Operands: []OperandPattern{{OpTypeIndReg, "(SP)"}, {OpTypeReg16, "IY"}}, Encoding: []byte{0xFD, 0xE3}},
	{Mnemonic: "EXX", Operands: []OperandPattern{}, Encoding: []byte{0xD9}},
	
	// Rotates
	{Mnemonic: "RLCA", Operands: []OperandPattern{}, Encoding: []byte{0x07}},
	{Mnemonic: "RLA", Operands: []OperandPattern{}, Encoding: []byte{0x17}},
	{Mnemonic: "RRCA", Operands: []OperandPattern{}, Encoding: []byte{0x0F}},
	{Mnemonic: "RRA", Operands: []OperandPattern{}, Encoding: []byte{0x1F}},
	
	// Misc
	{Mnemonic: "DAA", Operands: []OperandPattern{}, Encoding: []byte{0x27}},
	{Mnemonic: "CPL", Operands: []OperandPattern{}, Encoding: []byte{0x2F}},
	{Mnemonic: "NEG", Operands: []OperandPattern{}, Encoding: []byte{0xED, 0x44}},
	{Mnemonic: "CCF", Operands: []OperandPattern{}, Encoding: []byte{0x3F}},
	{Mnemonic: "SCF", Operands: []OperandPattern{}, Encoding: []byte{0x37}},
}

func encodeCALL(a *Assembler, pattern *InstructionPattern, values []interface{}) ([]byte, error) {
	result := make([]byte, len(pattern.Encoding))
	copy(result, pattern.Encoding)
	
	// Find address value
	for _, v := range values {
		if addr, ok := v.(uint16); ok {
			result = append(result, byte(addr), byte(addr>>8))
			return result, nil
		}
	}
	
	return nil, fmt.Errorf("no address value found")
}

// GetInstructionPatterns returns all patterns for a given mnemonic
func GetInstructionPatterns(mnemonic string) []InstructionPattern {
	mnemonic = strings.ToUpper(mnemonic)
	var patterns []InstructionPattern
	
	for _, pattern := range instructionTable {
		if pattern.Mnemonic == mnemonic {
			patterns = append(patterns, pattern)
		}
	}
	
	return patterns
}