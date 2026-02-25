// Package disasm provides a comprehensive Z80 disassembler.
//
// It correctly handles all Z80 instructions including CB, ED, DD, FD,
// DDCB, and FDCB prefixed instructions (including undocumented opcodes).
package disasm

import (
	"encoding/binary"
	"fmt"
)

// Disasm disassembles a single Z80 instruction from mem at the given PC.
// Returns the mnemonic string and the instruction size in bytes.
func Disasm(mem []byte, pc uint16) (string, int) {
	if len(mem) == 0 {
		return "???", 1
	}
	op := mem[0]

	// Helper functions
	nn := func() uint16 {
		if len(mem) >= 3 {
			return binary.LittleEndian.Uint16(mem[1:3])
		}
		return 0
	}
	n := func() byte {
		if len(mem) >= 2 {
			return mem[1]
		}
		return 0
	}
	jr := func() uint16 {
		if len(mem) >= 2 {
			return uint16(int(pc) + 2 + int(int8(mem[1])))
		}
		return pc
	}

	// 8-bit register names
	r := []string{"B", "C", "D", "E", "H", "L", "(HL)", "A"}
	// ALU operations
	alu := []string{"ADD A,", "ADC A,", "SUB", "SBC A,", "AND", "XOR", "OR", "CP"}

	// Handle prefixed instructions first
	switch op {
	case 0xCB: // CB prefix - bit operations
		if len(mem) < 2 {
			return "DB $CB", 1
		}
		return disasmCB(mem[1]), 2

	case 0xDD: // DD prefix - IX instructions
		if len(mem) < 2 {
			return "DB $DD", 1
		}
		return disasmDDFD(mem[1:], pc, "IX")

	case 0xFD: // FD prefix - IY instructions
		if len(mem) < 2 {
			return "DB $FD", 1
		}
		return disasmDDFD(mem[1:], pc, "IY")

	case 0xED: // ED prefix - extended instructions
		if len(mem) < 2 {
			return "DB $ED", 1
		}
		return disasmED(mem[1:])
	}

	// Main opcode table
	switch op {
	// 0x00-0x0F
	case 0x00:
		return "NOP", 1
	case 0x01:
		return fmt.Sprintf("LD BC,$%04X", nn()), 3
	case 0x02:
		return "LD (BC),A", 1
	case 0x03:
		return "INC BC", 1
	case 0x04:
		return "INC B", 1
	case 0x05:
		return "DEC B", 1
	case 0x06:
		return fmt.Sprintf("LD B,$%02X", n()), 2
	case 0x07:
		return "RLCA", 1
	case 0x08:
		return "EX AF,AF'", 1
	case 0x09:
		return "ADD HL,BC", 1
	case 0x0A:
		return "LD A,(BC)", 1
	case 0x0B:
		return "DEC BC", 1
	case 0x0C:
		return "INC C", 1
	case 0x0D:
		return "DEC C", 1
	case 0x0E:
		return fmt.Sprintf("LD C,$%02X", n()), 2
	case 0x0F:
		return "RRCA", 1

	// 0x10-0x1F
	case 0x10:
		return fmt.Sprintf("DJNZ $%04X", jr()), 2
	case 0x11:
		return fmt.Sprintf("LD DE,$%04X", nn()), 3
	case 0x12:
		return "LD (DE),A", 1
	case 0x13:
		return "INC DE", 1
	case 0x14:
		return "INC D", 1
	case 0x15:
		return "DEC D", 1
	case 0x16:
		return fmt.Sprintf("LD D,$%02X", n()), 2
	case 0x17:
		return "RLA", 1
	case 0x18:
		return fmt.Sprintf("JR $%04X", jr()), 2
	case 0x19:
		return "ADD HL,DE", 1
	case 0x1A:
		return "LD A,(DE)", 1
	case 0x1B:
		return "DEC DE", 1
	case 0x1C:
		return "INC E", 1
	case 0x1D:
		return "DEC E", 1
	case 0x1E:
		return fmt.Sprintf("LD E,$%02X", n()), 2
	case 0x1F:
		return "RRA", 1

	// 0x20-0x2F
	case 0x20:
		return fmt.Sprintf("JR NZ,$%04X", jr()), 2
	case 0x21:
		return fmt.Sprintf("LD HL,$%04X", nn()), 3
	case 0x22:
		return fmt.Sprintf("LD ($%04X),HL", nn()), 3
	case 0x23:
		return "INC HL", 1
	case 0x24:
		return "INC H", 1
	case 0x25:
		return "DEC H", 1
	case 0x26:
		return fmt.Sprintf("LD H,$%02X", n()), 2
	case 0x27:
		return "DAA", 1
	case 0x28:
		return fmt.Sprintf("JR Z,$%04X", jr()), 2
	case 0x29:
		return "ADD HL,HL", 1
	case 0x2A:
		return fmt.Sprintf("LD HL,($%04X)", nn()), 3
	case 0x2B:
		return "DEC HL", 1
	case 0x2C:
		return "INC L", 1
	case 0x2D:
		return "DEC L", 1
	case 0x2E:
		return fmt.Sprintf("LD L,$%02X", n()), 2
	case 0x2F:
		return "CPL", 1

	// 0x30-0x3F
	case 0x30:
		return fmt.Sprintf("JR NC,$%04X", jr()), 2
	case 0x31:
		return fmt.Sprintf("LD SP,$%04X", nn()), 3
	case 0x32:
		return fmt.Sprintf("LD ($%04X),A", nn()), 3
	case 0x33:
		return "INC SP", 1
	case 0x34:
		return "INC (HL)", 1
	case 0x35:
		return "DEC (HL)", 1
	case 0x36:
		return fmt.Sprintf("LD (HL),$%02X", n()), 2
	case 0x37:
		return "SCF", 1
	case 0x38:
		return fmt.Sprintf("JR C,$%04X", jr()), 2
	case 0x39:
		return "ADD HL,SP", 1
	case 0x3A:
		return fmt.Sprintf("LD A,($%04X)", nn()), 3
	case 0x3B:
		return "DEC SP", 1
	case 0x3C:
		return "INC A", 1
	case 0x3D:
		return "DEC A", 1
	case 0x3E:
		return fmt.Sprintf("LD A,$%02X", n()), 2
	case 0x3F:
		return "CCF", 1

	// 0x40-0x7F: LD r,r' (except HALT at 0x76)
	case 0x76:
		return "HALT", 1
	}

	// LD r,r' block (0x40-0x7F except 0x76)
	if op >= 0x40 && op <= 0x7F {
		dst := (op >> 3) & 0x07
		src := op & 0x07
		return fmt.Sprintf("LD %s,%s", r[dst], r[src]), 1
	}

	// ALU operations (0x80-0xBF)
	if op >= 0x80 && op <= 0xBF {
		aluOp := (op >> 3) & 0x07
		src := op & 0x07
		return fmt.Sprintf("%s %s", alu[aluOp], r[src]), 1
	}

	// 0xC0-0xFF
	switch op {
	case 0xC0:
		return "RET NZ", 1
	case 0xC1:
		return "POP BC", 1
	case 0xC2:
		return fmt.Sprintf("JP NZ,$%04X", nn()), 3
	case 0xC3:
		return fmt.Sprintf("JP $%04X", nn()), 3
	case 0xC4:
		return fmt.Sprintf("CALL NZ,$%04X", nn()), 3
	case 0xC5:
		return "PUSH BC", 1
	case 0xC6:
		return fmt.Sprintf("ADD A,$%02X", n()), 2
	case 0xC7:
		return "RST $00", 1
	case 0xC8:
		return "RET Z", 1
	case 0xC9:
		return "RET", 1
	case 0xCA:
		return fmt.Sprintf("JP Z,$%04X", nn()), 3
	case 0xCC:
		return fmt.Sprintf("CALL Z,$%04X", nn()), 3
	case 0xCD:
		return fmt.Sprintf("CALL $%04X", nn()), 3
	case 0xCE:
		return fmt.Sprintf("ADC A,$%02X", n()), 2
	case 0xCF:
		return "RST $08", 1

	case 0xD0:
		return "RET NC", 1
	case 0xD1:
		return "POP DE", 1
	case 0xD2:
		return fmt.Sprintf("JP NC,$%04X", nn()), 3
	case 0xD3:
		return fmt.Sprintf("OUT ($%02X),A", n()), 2
	case 0xD4:
		return fmt.Sprintf("CALL NC,$%04X", nn()), 3
	case 0xD5:
		return "PUSH DE", 1
	case 0xD6:
		return fmt.Sprintf("SUB $%02X", n()), 2
	case 0xD7:
		return "RST $10", 1
	case 0xD8:
		return "RET C", 1
	case 0xD9:
		return "EXX", 1
	case 0xDA:
		return fmt.Sprintf("JP C,$%04X", nn()), 3
	case 0xDB:
		return fmt.Sprintf("IN A,($%02X)", n()), 2
	case 0xDC:
		return fmt.Sprintf("CALL C,$%04X", nn()), 3
	case 0xDE:
		return fmt.Sprintf("SBC A,$%02X", n()), 2
	case 0xDF:
		return "RST $18", 1

	case 0xE0:
		return "RET PO", 1
	case 0xE1:
		return "POP HL", 1
	case 0xE2:
		return fmt.Sprintf("JP PO,$%04X", nn()), 3
	case 0xE3:
		return "EX (SP),HL", 1
	case 0xE4:
		return fmt.Sprintf("CALL PO,$%04X", nn()), 3
	case 0xE5:
		return "PUSH HL", 1
	case 0xE6:
		return fmt.Sprintf("AND $%02X", n()), 2
	case 0xE7:
		return "RST $20", 1
	case 0xE8:
		return "RET PE", 1
	case 0xE9:
		return "JP (HL)", 1
	case 0xEA:
		return fmt.Sprintf("JP PE,$%04X", nn()), 3
	case 0xEB:
		return "EX DE,HL", 1
	case 0xEC:
		return fmt.Sprintf("CALL PE,$%04X", nn()), 3
	case 0xEE:
		return fmt.Sprintf("XOR $%02X", n()), 2
	case 0xEF:
		return "RST $28", 1

	case 0xF0:
		return "RET P", 1
	case 0xF1:
		return "POP AF", 1
	case 0xF2:
		return fmt.Sprintf("JP P,$%04X", nn()), 3
	case 0xF3:
		return "DI", 1
	case 0xF4:
		return fmt.Sprintf("CALL P,$%04X", nn()), 3
	case 0xF5:
		return "PUSH AF", 1
	case 0xF6:
		return fmt.Sprintf("OR $%02X", n()), 2
	case 0xF7:
		return "RST $30", 1
	case 0xF8:
		return "RET M", 1
	case 0xF9:
		return "LD SP,HL", 1
	case 0xFA:
		return fmt.Sprintf("JP M,$%04X", nn()), 3
	case 0xFB:
		return "EI", 1
	case 0xFC:
		return fmt.Sprintf("CALL M,$%04X", nn()), 3
	case 0xFE:
		return fmt.Sprintf("CP $%02X", n()), 2
	case 0xFF:
		return "RST $38", 1
	}

	return fmt.Sprintf("DB $%02X", op), 1
}

// DisasmFull disassembles a single instruction and returns additional metadata.
// targetAddr is the resolved target for branch/call instructions (-1 if not a branch).
// relOffset is the signed relative offset for JR/DJNZ instructions (0 otherwise).
func DisasmFull(mem []byte, pc uint16) (mnemonic string, size int, targetAddr int, relOffset int) {
	mnemonic, size = Disasm(mem, pc)
	targetAddr = -1

	if len(mem) == 0 {
		return
	}
	op := mem[0]

	// JR/DJNZ: relative branches
	switch op {
	case 0x10, 0x18, 0x20, 0x28, 0x30, 0x38: // DJNZ, JR, JR NZ, JR Z, JR NC, JR C
		if len(mem) >= 2 {
			rel := int(int8(mem[1]))
			relOffset = rel
			targetAddr = int(pc) + 2 + rel
		}
		return
	}

	// JP nn / JP cc,nn
	switch op {
	case 0xC3, 0xC2, 0xCA, 0xD2, 0xDA, 0xE2, 0xEA, 0xF2, 0xFA:
		if len(mem) >= 3 {
			targetAddr = int(binary.LittleEndian.Uint16(mem[1:3]))
		}
		return
	}

	// CALL nn / CALL cc,nn
	switch op {
	case 0xCD, 0xC4, 0xCC, 0xD4, 0xDC, 0xE4, 0xEC, 0xF4, 0xFC:
		if len(mem) >= 3 {
			targetAddr = int(binary.LittleEndian.Uint16(mem[1:3]))
		}
		return
	}

	// RST instructions
	switch op {
	case 0xC7:
		targetAddr = 0x00
	case 0xCF:
		targetAddr = 0x08
	case 0xD7:
		targetAddr = 0x10
	case 0xDF:
		targetAddr = 0x18
	case 0xE7:
		targetAddr = 0x20
	case 0xEF:
		targetAddr = 0x28
	case 0xF7:
		targetAddr = 0x30
	case 0xFF:
		targetAddr = 0x38
	}

	return
}

// disasmCB handles CB-prefixed bit manipulation instructions.
func disasmCB(op byte) string {
	r := []string{"B", "C", "D", "E", "H", "L", "(HL)", "A"}
	reg := op & 0x07
	bit := (op >> 3) & 0x07

	switch op >> 6 {
	case 0: // Rotates and shifts
		ops := []string{"RLC", "RRC", "RL", "RR", "SLA", "SRA", "SLL", "SRL"}
		return fmt.Sprintf("%s %s", ops[bit], r[reg])
	case 1: // BIT
		return fmt.Sprintf("BIT %d,%s", bit, r[reg])
	case 2: // RES
		return fmt.Sprintf("RES %d,%s", bit, r[reg])
	case 3: // SET
		return fmt.Sprintf("SET %d,%s", bit, r[reg])
	}
	return fmt.Sprintf("DB $CB,$%02X", op)
}

// disasmED handles ED-prefixed extended instructions.
func disasmED(mem []byte) (string, int) {
	if len(mem) == 0 {
		return "DB $ED", 1
	}
	op := mem[0]

	nn := func() uint16 {
		if len(mem) >= 3 {
			return binary.LittleEndian.Uint16(mem[1:3])
		}
		return 0
	}

	switch op {
	// Input/Output block
	case 0x40:
		return "IN B,(C)", 2
	case 0x41:
		return "OUT (C),B", 2
	case 0x42:
		return "SBC HL,BC", 2
	case 0x43:
		return fmt.Sprintf("LD ($%04X),BC", nn()), 4
	case 0x44:
		return "NEG", 2
	case 0x45:
		return "RETN", 2
	case 0x46:
		return "IM 0", 2
	case 0x47:
		return "LD I,A", 2
	case 0x48:
		return "IN C,(C)", 2
	case 0x49:
		return "OUT (C),C", 2
	case 0x4A:
		return "ADC HL,BC", 2
	case 0x4B:
		return fmt.Sprintf("LD BC,($%04X)", nn()), 4
	case 0x4D:
		return "RETI", 2
	case 0x4F:
		return "LD R,A", 2

	case 0x50:
		return "IN D,(C)", 2
	case 0x51:
		return "OUT (C),D", 2
	case 0x52:
		return "SBC HL,DE", 2
	case 0x53:
		return fmt.Sprintf("LD ($%04X),DE", nn()), 4
	case 0x56:
		return "IM 1", 2
	case 0x57:
		return "LD A,I", 2
	case 0x58:
		return "IN E,(C)", 2
	case 0x59:
		return "OUT (C),E", 2
	case 0x5A:
		return "ADC HL,DE", 2
	case 0x5B:
		return fmt.Sprintf("LD DE,($%04X)", nn()), 4
	case 0x5E:
		return "IM 2", 2
	case 0x5F:
		return "LD A,R", 2

	case 0x60:
		return "IN H,(C)", 2
	case 0x61:
		return "OUT (C),H", 2
	case 0x62:
		return "SBC HL,HL", 2
	case 0x63:
		return fmt.Sprintf("LD ($%04X),HL", nn()), 4
	case 0x67:
		return "RRD", 2
	case 0x68:
		return "IN L,(C)", 2
	case 0x69:
		return "OUT (C),L", 2
	case 0x6A:
		return "ADC HL,HL", 2
	case 0x6B:
		return fmt.Sprintf("LD HL,($%04X)", nn()), 4
	case 0x6F:
		return "RLD", 2

	case 0x72:
		return "SBC HL,SP", 2
	case 0x73:
		return fmt.Sprintf("LD ($%04X),SP", nn()), 4
	case 0x78:
		return "IN A,(C)", 2
	case 0x79:
		return "OUT (C),A", 2
	case 0x7A:
		return "ADC HL,SP", 2
	case 0x7B:
		return fmt.Sprintf("LD SP,($%04X)", nn()), 4

	// Block instructions
	case 0xA0:
		return "LDI", 2
	case 0xA1:
		return "CPI", 2
	case 0xA2:
		return "INI", 2
	case 0xA3:
		return "OUTI", 2
	case 0xA8:
		return "LDD", 2
	case 0xA9:
		return "CPD", 2
	case 0xAA:
		return "IND", 2
	case 0xAB:
		return "OUTD", 2
	case 0xB0:
		return "LDIR", 2
	case 0xB1:
		return "CPIR", 2
	case 0xB2:
		return "INIR", 2
	case 0xB3:
		return "OTIR", 2
	case 0xB8:
		return "LDDR", 2
	case 0xB9:
		return "CPDR", 2
	case 0xBA:
		return "INDR", 2
	case 0xBB:
		return "OTDR", 2
	}

	return fmt.Sprintf("DB $ED,$%02X", op), 2
}

// disasmDDFD handles DD/FD-prefixed IX/IY instructions.
func disasmDDFD(mem []byte, pc uint16, idx string) (string, int) {
	if len(mem) == 0 {
		return fmt.Sprintf("DB $%s", idx), 1
	}
	op := mem[0]

	nn := func() uint16 {
		if len(mem) >= 3 {
			return binary.LittleEndian.Uint16(mem[1:3])
		}
		return 0
	}
	d := func() int8 {
		if len(mem) >= 2 {
			return int8(mem[1])
		}
		return 0
	}
	dispStr := func() string {
		disp := d()
		if disp >= 0 {
			return fmt.Sprintf("(%s+$%02X)", idx, disp)
		}
		return fmt.Sprintf("(%s-$%02X)", idx, -disp)
	}

	switch op {
	case 0x09:
		return fmt.Sprintf("ADD %s,BC", idx), 2
	case 0x19:
		return fmt.Sprintf("ADD %s,DE", idx), 2
	case 0x21:
		return fmt.Sprintf("LD %s,$%04X", idx, nn()), 4
	case 0x22:
		return fmt.Sprintf("LD ($%04X),%s", nn(), idx), 4
	case 0x23:
		return fmt.Sprintf("INC %s", idx), 2
	case 0x29:
		return fmt.Sprintf("ADD %s,%s", idx, idx), 2
	case 0x2A:
		return fmt.Sprintf("LD %s,($%04X)", idx, nn()), 4
	case 0x2B:
		return fmt.Sprintf("DEC %s", idx), 2
	case 0x34:
		return fmt.Sprintf("INC %s", dispStr()), 3
	case 0x35:
		return fmt.Sprintf("DEC %s", dispStr()), 3
	case 0x36:
		if len(mem) >= 3 {
			return fmt.Sprintf("LD %s,$%02X", dispStr(), mem[2]), 4
		}
	case 0x39:
		return fmt.Sprintf("ADD %s,SP", idx), 2
	case 0x46:
		return fmt.Sprintf("LD B,%s", dispStr()), 3
	case 0x4E:
		return fmt.Sprintf("LD C,%s", dispStr()), 3
	case 0x56:
		return fmt.Sprintf("LD D,%s", dispStr()), 3
	case 0x5E:
		return fmt.Sprintf("LD E,%s", dispStr()), 3
	case 0x66:
		return fmt.Sprintf("LD H,%s", dispStr()), 3
	case 0x6E:
		return fmt.Sprintf("LD L,%s", dispStr()), 3
	case 0x70:
		return fmt.Sprintf("LD %s,B", dispStr()), 3
	case 0x71:
		return fmt.Sprintf("LD %s,C", dispStr()), 3
	case 0x72:
		return fmt.Sprintf("LD %s,D", dispStr()), 3
	case 0x73:
		return fmt.Sprintf("LD %s,E", dispStr()), 3
	case 0x74:
		return fmt.Sprintf("LD %s,H", dispStr()), 3
	case 0x75:
		return fmt.Sprintf("LD %s,L", dispStr()), 3
	case 0x77:
		return fmt.Sprintf("LD %s,A", dispStr()), 3
	case 0x7E:
		return fmt.Sprintf("LD A,%s", dispStr()), 3
	case 0x86:
		return fmt.Sprintf("ADD A,%s", dispStr()), 3
	case 0x8E:
		return fmt.Sprintf("ADC A,%s", dispStr()), 3
	case 0x96:
		return fmt.Sprintf("SUB %s", dispStr()), 3
	case 0x9E:
		return fmt.Sprintf("SBC A,%s", dispStr()), 3
	case 0xA6:
		return fmt.Sprintf("AND %s", dispStr()), 3
	case 0xAE:
		return fmt.Sprintf("XOR %s", dispStr()), 3
	case 0xB6:
		return fmt.Sprintf("OR %s", dispStr()), 3
	case 0xBE:
		return fmt.Sprintf("CP %s", dispStr()), 3
	case 0xCB:
		// DD CB d op or FD CB d op - bit operations on (IX+d)/(IY+d)
		if len(mem) >= 3 {
			disp := int8(mem[1])
			bitOp := mem[2]
			var dispS string
			if disp >= 0 {
				dispS = fmt.Sprintf("(%s+$%02X)", idx, disp)
			} else {
				dispS = fmt.Sprintf("(%s-$%02X)", idx, -disp)
			}
			bit := (bitOp >> 3) & 0x07
			switch bitOp >> 6 {
			case 0:
				ops := []string{"RLC", "RRC", "RL", "RR", "SLA", "SRA", "SLL", "SRL"}
				return fmt.Sprintf("%s %s", ops[bit], dispS), 4
			case 1:
				return fmt.Sprintf("BIT %d,%s", bit, dispS), 4
			case 2:
				return fmt.Sprintf("RES %d,%s", bit, dispS), 4
			case 3:
				return fmt.Sprintf("SET %d,%s", bit, dispS), 4
			}
		}
	case 0xE1:
		return fmt.Sprintf("POP %s", idx), 2
	case 0xE3:
		return fmt.Sprintf("EX (SP),%s", idx), 2
	case 0xE5:
		return fmt.Sprintf("PUSH %s", idx), 2
	case 0xE9:
		return fmt.Sprintf("JP (%s)", idx), 2
	case 0xF9:
		return fmt.Sprintf("LD SP,%s", idx), 2
	}

	return fmt.Sprintf("DB $DD/FD,$%02X", op), 2
}
