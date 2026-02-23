package analysis

import (
	"github.com/minz/minzc/pkg/disasm"
)

// Cycles returns T-state counts for the instruction at mem.
// Returns (taken, notTaken) — for unconditional instructions both are equal.
// For conditional branches, taken is when branch is taken, notTaken when not.
func Cycles(mem []byte) (taken int, notTaken int) {
	if len(mem) == 0 {
		return 0, 0
	}

	op := mem[0]

	// Check for prefix bytes first
	switch op {
	case 0xCB:
		if len(mem) < 2 {
			return 4, 4
		}
		return cyclesCB(mem[1]), cyclesCB(mem[1])

	case 0xED:
		if len(mem) < 2 {
			return 4, 4
		}
		t := cyclesED(mem[1])
		return t, t

	case 0xDD, 0xFD:
		if len(mem) < 2 {
			return 4, 4
		}
		if mem[1] == 0xCB {
			// DDCB/FDCB — all 23T except BIT which is 20T
			if len(mem) >= 4 {
				bitOp := mem[3]
				if bitOp>>6 == 1 { // BIT
					return 20, 20
				}
			}
			return 23, 23
		}
		t := cyclesDDFD(mem[1])
		return t, t
	}

	return cyclesMain(op)
}

// cyclesMain returns (taken, notTaken) T-states for main opcodes.
func cyclesMain(op byte) (int, int) {
	switch op {
	// NOP
	case 0x00:
		return 4, 4

	// LD rr,nn
	case 0x01, 0x11, 0x21, 0x31:
		return 10, 10

	// LD (BC/DE),A and LD A,(BC/DE)
	case 0x02, 0x0A, 0x12, 0x1A:
		return 7, 7

	// INC/DEC rr (16-bit)
	case 0x03, 0x0B, 0x13, 0x1B, 0x23, 0x2B, 0x33, 0x3B:
		return 6, 6

	// INC/DEC r (8-bit)
	case 0x04, 0x05, 0x0C, 0x0D, 0x14, 0x15, 0x1C, 0x1D, 0x24, 0x25, 0x2C, 0x2D, 0x3C, 0x3D:
		return 4, 4

	// INC/DEC (HL)
	case 0x34, 0x35:
		return 11, 11

	// LD r,n (8-bit immediate)
	case 0x06, 0x0E, 0x16, 0x1E, 0x26, 0x2E, 0x3E:
		return 7, 7
	case 0x36: // LD (HL),n
		return 10, 10

	// RLCA, RRCA, RLA, RRA
	case 0x07, 0x0F, 0x17, 0x1F:
		return 4, 4

	// EX AF,AF'
	case 0x08:
		return 4, 4

	// ADD HL,rr
	case 0x09, 0x19, 0x29, 0x39:
		return 11, 11

	// DJNZ
	case 0x10:
		return 13, 8 // taken/not-taken

	// JR e
	case 0x18:
		return 12, 12

	// JR cc,e
	case 0x20, 0x28, 0x30, 0x38:
		return 12, 7

	// LD (nn),HL / LD HL,(nn)
	case 0x22, 0x2A:
		return 16, 16

	// DAA, CPL, SCF, CCF
	case 0x27, 0x2F, 0x37, 0x3F:
		return 4, 4

	// LD (nn),A / LD A,(nn)
	case 0x32, 0x3A:
		return 13, 13

	// HALT
	case 0x76:
		return 4, 4

	// LD r,r' (0x40-0x7F except 0x76)
	default:
		if op >= 0x40 && op <= 0x7F {
			src := op & 0x07
			dst := (op >> 3) & 0x07
			if src == 6 || dst == 6 { // (HL) involved
				return 7, 7
			}
			return 4, 4
		}

		// ALU r (0x80-0xBF)
		if op >= 0x80 && op <= 0xBF {
			src := op & 0x07
			if src == 6 { // (HL)
				return 7, 7
			}
			return 4, 4
		}
	}

	// 0xC0-0xFF
	switch op {
	// RET cc
	case 0xC0, 0xC8, 0xD0, 0xD8, 0xE0, 0xE8, 0xF0, 0xF8:
		return 11, 5

	// POP rr
	case 0xC1, 0xD1, 0xE1, 0xF1:
		return 10, 10

	// JP cc,nn
	case 0xC2, 0xCA, 0xD2, 0xDA, 0xE2, 0xEA, 0xF2, 0xFA:
		return 10, 10

	// JP nn
	case 0xC3:
		return 10, 10

	// CALL cc,nn
	case 0xC4, 0xCC, 0xD4, 0xDC, 0xE4, 0xEC, 0xF4, 0xFC:
		return 17, 10

	// PUSH rr
	case 0xC5, 0xD5, 0xE5, 0xF5:
		return 11, 11

	// ALU A,n (immediate)
	case 0xC6, 0xCE, 0xD6, 0xDE, 0xE6, 0xEE, 0xF6, 0xFE:
		return 7, 7

	// RST
	case 0xC7, 0xCF, 0xD7, 0xDF, 0xE7, 0xEF, 0xF7, 0xFF:
		return 11, 11

	// RET
	case 0xC9:
		return 10, 10

	// CALL nn
	case 0xCD:
		return 17, 17

	// OUT (n),A / IN A,(n)
	case 0xD3, 0xDB:
		return 11, 11

	// EXX
	case 0xD9:
		return 4, 4

	// EX (SP),HL
	case 0xE3:
		return 19, 19

	// JP (HL)
	case 0xE9:
		return 4, 4

	// EX DE,HL
	case 0xEB:
		return 4, 4

	// DI, EI
	case 0xF3, 0xFB:
		return 4, 4

	// LD SP,HL
	case 0xF9:
		return 6, 6
	}

	return 4, 4 // Default for unknown
}

// cyclesCB returns T-states for CB-prefixed instructions.
func cyclesCB(op byte) int {
	reg := op & 0x07
	group := op >> 6

	if reg == 6 { // (HL) operations
		if group == 1 { // BIT b,(HL)
			return 12
		}
		return 15 // RLC/RRC/RL/RR/SLA/SRA/SLL/SRL/RES/SET (HL)
	}
	if group == 1 { // BIT b,r
		return 8
	}
	return 8 // All register rotate/shift/RES/SET
}

// cyclesED returns T-states for ED-prefixed instructions.
func cyclesED(op byte) int {
	switch op {
	// IN r,(C) / OUT (C),r
	case 0x40, 0x48, 0x50, 0x58, 0x60, 0x68, 0x78:
		return 12 // IN
	case 0x41, 0x49, 0x51, 0x59, 0x61, 0x69, 0x79:
		return 12 // OUT

	// SBC HL,rr / ADC HL,rr
	case 0x42, 0x52, 0x62, 0x72: // SBC HL,rr
		return 15
	case 0x4A, 0x5A, 0x6A, 0x7A: // ADC HL,rr
		return 15

	// LD (nn),rr / LD rr,(nn) (ED-prefixed)
	case 0x43, 0x53, 0x63, 0x73: // LD (nn),rr
		return 20
	case 0x4B, 0x5B, 0x6B, 0x7B: // LD rr,(nn)
		return 20

	// NEG
	case 0x44:
		return 8

	// RETN / RETI
	case 0x45:
		return 14
	case 0x4D:
		return 14

	// IM 0/1/2
	case 0x46, 0x56, 0x5E:
		return 8

	// LD I,A / LD R,A / LD A,I / LD A,R
	case 0x47, 0x4F, 0x57, 0x5F:
		return 9

	// RRD / RLD
	case 0x67, 0x6F:
		return 18

	// Block instructions (non-repeating)
	case 0xA0, 0xA8: // LDI, LDD
		return 16
	case 0xA1, 0xA9: // CPI, CPD
		return 16
	case 0xA2, 0xAA: // INI, IND
		return 16
	case 0xA3, 0xAB: // OUTI, OUTD
		return 16

	// Block instructions (repeating)
	case 0xB0, 0xB8: // LDIR, LDDR
		return 21 // 21 when repeating, 16 when done (we report max)
	case 0xB1, 0xB9: // CPIR, CPDR
		return 21
	case 0xB2, 0xBA: // INIR, INDR
		return 21
	case 0xB3, 0xBB: // OTIR, OTDR
		return 21
	}

	return 8 // Default for undefined ED ops (acts as NOP NOP)
}

// cyclesDDFD returns T-states for DD/FD-prefixed instructions.
func cyclesDDFD(op byte) int {
	switch op {
	// ADD IX/IY,rr
	case 0x09, 0x19, 0x29, 0x39:
		return 15

	// LD IX/IY,nn
	case 0x21:
		return 14

	// LD (nn),IX/IY
	case 0x22:
		return 20

	// INC IX/IY
	case 0x23:
		return 10

	// LD IX/IY,(nn)
	case 0x2A:
		return 20

	// DEC IX/IY
	case 0x2B:
		return 10

	// INC/DEC (IX/IY+d)
	case 0x34, 0x35:
		return 23

	// LD (IX/IY+d),n
	case 0x36:
		return 19

	// LD r,(IX/IY+d)
	case 0x46, 0x4E, 0x56, 0x5E, 0x66, 0x6E, 0x7E:
		return 19

	// LD (IX/IY+d),r
	case 0x70, 0x71, 0x72, 0x73, 0x74, 0x75, 0x77:
		return 19

	// ALU (IX/IY+d)
	case 0x86, 0x8E, 0x96, 0x9E, 0xA6, 0xAE, 0xB6, 0xBE:
		return 19

	// POP IX/IY
	case 0xE1:
		return 14

	// EX (SP),IX/IY
	case 0xE3:
		return 23

	// PUSH IX/IY
	case 0xE5:
		return 15

	// JP (IX/IY)
	case 0xE9:
		return 8

	// LD SP,IX/IY
	case 0xF9:
		return 10
	}

	return 8 // Default for undefined DD/FD ops
}

// BasicBlockCycles sums T-states for a basic block starting at addr.
// Returns (totalTaken, totalNotTaken) — they differ only if the last
// instruction is conditional.
func (a *Analysis) BasicBlockCycles(addr uint16) (int, int) {
	totalTaken := 0
	totalNotTaken := 0

	for {
		if !a.InRange(addr) || a.ByteMap[addr] != ByteCodeStart {
			break
		}

		mem := a.ReadBytes(addr, 4)
		if len(mem) == 0 {
			break
		}

		_, size := disasm.Disasm(mem, addr)
		taken, notTaken := Cycles(mem)

		// For all but the last instruction in the block, use 'taken' for both
		// We'll adjust at the end if the last is conditional
		totalTaken += taken
		totalNotTaken += notTaken

		next := uint16(int(addr) + size)
		op := mem[0]

		// Check if this is a block-ending instruction
		switch classifyInstruction(op, mem, size) {
		case instrUnconditionalJump, instrReturn, instrHalt, instrIndirectJump:
			return totalTaken, totalNotTaken
		case instrConditionalJump, instrConditionalReturn, instrConditionalCall:
			return totalTaken, totalNotTaken
		}

		// Check if the next address is still code in the same block
		if next > addr && a.ByteMap[next] == ByteCodeStart {
			addr = next
		} else {
			break
		}
	}

	return totalTaken, totalNotTaken
}
