package z80asm

// Comprehensive Z80 opcode encoding test, modeled after sjasmplus op_*.asm test suite.
// Verifies every documented Z80 instruction encodes to the correct bytes.

import (
	"bytes"
	"fmt"
	"testing"
)

// assembleOne assembles a single instruction and returns its bytes.
func assembleOne(t *testing.T, instruction string) []byte {
	t.Helper()
	source := fmt.Sprintf("ORG $8000\n%s", instruction)
	asm := NewAssembler()
	result, err := asm.AssembleString(source)
	if err != nil {
		t.Fatalf("Failed to assemble %q: %v", instruction, err)
	}
	return result.Binary
}

// --- Unprefixed opcodes 0x00-0x3F (sjasmplus op_00_3F.asm) ---

func TestOpcodes_00_3F(t *testing.T) {
	tests := []struct {
		inst     string
		expected []byte
	}{
		{"NOP", []byte{0x00}},
		{"LD BC, $1234", []byte{0x01, 0x34, 0x12}},
		{"LD (BC), A", []byte{0x02}},
		{"INC BC", []byte{0x03}},
		{"INC B", []byte{0x04}},
		{"DEC B", []byte{0x05}},
		{"LD B, $44", []byte{0x06, 0x44}},
		{"RLCA", []byte{0x07}},
		{"EX AF, AF'", []byte{0x08}},
		{"ADD HL, BC", []byte{0x09}},
		{"LD A, (BC)", []byte{0x0A}},
		{"DEC BC", []byte{0x0B}},
		{"INC C", []byte{0x0C}},
		{"DEC C", []byte{0x0D}},
		{"LD C, $44", []byte{0x0E, 0x44}},
		{"RRCA", []byte{0x0F}},
		// DJNZ: target = current_addr + 2 + offset. ORG $8000, DJNZ $8010 => offset = $0E
		{"DJNZ $800E", []byte{0x10, 0x0C}},
		{"LD DE, $1234", []byte{0x11, 0x34, 0x12}},
		{"LD (DE), A", []byte{0x12}},
		{"INC DE", []byte{0x13}},
		{"INC D", []byte{0x14}},
		{"DEC D", []byte{0x15}},
		{"LD D, $44", []byte{0x16, 0x44}},
		{"RLA", []byte{0x17}},
		// JR: similar offset calculation
		{"JR $800E", []byte{0x18, 0x0C}},
		{"ADD HL, DE", []byte{0x19}},
		{"LD A, (DE)", []byte{0x1A}},
		{"DEC DE", []byte{0x1B}},
		{"INC E", []byte{0x1C}},
		{"DEC E", []byte{0x1D}},
		{"LD E, $44", []byte{0x1E, 0x44}},
		{"RRA", []byte{0x1F}},
		{"JR NZ, $800E", []byte{0x20, 0x0C}},
		{"LD HL, $1234", []byte{0x21, 0x34, 0x12}},
		{"LD ($1234), HL", []byte{0x22, 0x34, 0x12}},
		{"INC HL", []byte{0x23}},
		{"INC H", []byte{0x24}},
		{"DEC H", []byte{0x25}},
		{"LD H, $44", []byte{0x26, 0x44}},
		{"DAA", []byte{0x27}},
		{"JR Z, $800E", []byte{0x28, 0x0C}},
		{"ADD HL, HL", []byte{0x29}},
		{"LD HL, ($1234)", []byte{0x2A, 0x34, 0x12}},
		{"DEC HL", []byte{0x2B}},
		{"INC L", []byte{0x2C}},
		{"DEC L", []byte{0x2D}},
		{"LD L, $44", []byte{0x2E, 0x44}},
		{"CPL", []byte{0x2F}},
		{"JR NC, $800E", []byte{0x30, 0x0C}},
		{"LD SP, $1234", []byte{0x31, 0x34, 0x12}},
		{"LD ($1234), A", []byte{0x32, 0x34, 0x12}},
		{"INC SP", []byte{0x33}},
		{"INC (HL)", []byte{0x34}},
		{"DEC (HL)", []byte{0x35}},
		{"LD (HL), $44", []byte{0x36, 0x44}},
		{"SCF", []byte{0x37}},
		{"JR C, $800E", []byte{0x38, 0x0C}},
		{"ADD HL, SP", []byte{0x39}},
		{"LD A, ($1234)", []byte{0x3A, 0x34, 0x12}},
		{"DEC SP", []byte{0x3B}},
		{"INC A", []byte{0x3C}},
		{"DEC A", []byte{0x3D}},
		{"LD A, $44", []byte{0x3E, 0x44}},
		{"CCF", []byte{0x3F}},
	}

	for _, tt := range tests {
		t.Run(tt.inst, func(t *testing.T) {
			got := assembleOne(t, tt.inst)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("%s: got %X, want %X", tt.inst, got, tt.expected)
			}
		})
	}
}

// --- LD r,r' opcodes 0x40-0x7F (sjasmplus op_40_7F.asm) ---

func TestOpcodes_40_7F_LD_and_HALT(t *testing.T) {
	regs := []string{"B", "C", "D", "E", "H", "L", "(HL)", "A"}
	baseOpcode := byte(0x40)

	for dstIdx, dst := range regs {
		for srcIdx, src := range regs {
			opcode := baseOpcode + byte(dstIdx*8) + byte(srcIdx)

			var inst string
			if dst == "(HL)" && src == "(HL)" {
				inst = "HALT"
			} else {
				inst = fmt.Sprintf("LD %s, %s", dst, src)
			}

			t.Run(inst, func(t *testing.T) {
				got := assembleOne(t, inst)
				expected := []byte{opcode}
				if !bytes.Equal(got, expected) {
					t.Errorf("%s: got %X, want %X", inst, got, expected)
				}
			})
		}
	}
}

// --- ALU opcodes 0x80-0xBF (sjasmplus op_80_BF.asm) ---

func TestOpcodes_80_BF_ALU(t *testing.T) {
	// ALU ops with register operands
	aluOps := []struct {
		mnemonic string
		base     byte
	}{
		{"ADD A,", 0x80},
		{"ADC A,", 0x88},
		{"SUB", 0x90},
		{"SBC A,", 0x98},
		{"AND", 0xA0},
		{"XOR", 0xA8},
		{"OR", 0xB0},
		{"CP", 0xB8},
	}
	regs := []string{"B", "C", "D", "E", "H", "L", "(HL)", "A"}

	for _, op := range aluOps {
		for regIdx, reg := range regs {
			opcode := op.base + byte(regIdx)
			var inst string
			if op.mnemonic[len(op.mnemonic)-1] == ',' {
				inst = fmt.Sprintf("%s %s", op.mnemonic, reg)
			} else {
				inst = fmt.Sprintf("%s %s", op.mnemonic, reg)
			}

			t.Run(inst, func(t *testing.T) {
				got := assembleOne(t, inst)
				expected := []byte{opcode}
				if !bytes.Equal(got, expected) {
					t.Errorf("%s: got %X, want %X", inst, got, expected)
				}
			})
		}
	}
}

// --- ALU immediate opcodes (C6, CE, D6, DE, E6, EE, F6, FE) ---

func TestOpcodes_ALU_Immediate(t *testing.T) {
	tests := []struct {
		inst     string
		expected []byte
	}{
		{"ADD A, $44", []byte{0xC6, 0x44}},
		{"ADC A, $44", []byte{0xCE, 0x44}},
		{"SUB $44", []byte{0xD6, 0x44}},
		{"SBC A, $44", []byte{0xDE, 0x44}},
		{"AND $44", []byte{0xE6, 0x44}},
		{"XOR $44", []byte{0xEE, 0x44}},
		{"OR $44", []byte{0xF6, 0x44}},
		{"CP $44", []byte{0xFE, 0x44}},
	}

	for _, tt := range tests {
		t.Run(tt.inst, func(t *testing.T) {
			got := assembleOne(t, tt.inst)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("%s: got %X, want %X", tt.inst, got, tt.expected)
			}
		})
	}
}

// --- Upper opcodes 0xC0-0xFF (sjasmplus op_C0_FF.asm) ---

func TestOpcodes_C0_FF(t *testing.T) {
	tests := []struct {
		inst     string
		expected []byte
	}{
		{"RET NZ", []byte{0xC0}},
		{"POP BC", []byte{0xC1}},
		{"JP NZ, $1234", []byte{0xC2, 0x34, 0x12}},
		{"JP $1234", []byte{0xC3, 0x34, 0x12}},
		{"CALL NZ, $1234", []byte{0xC4, 0x34, 0x12}},
		{"PUSH BC", []byte{0xC5}},
		// C6: ADD A, imm - tested above
		{"RST $00", []byte{0xC7}},
		{"RET Z", []byte{0xC8}},
		{"RET", []byte{0xC9}},
		{"JP Z, $1234", []byte{0xCA, 0x34, 0x12}},
		// CB: BIT prefix - tested separately
		{"CALL Z, $1234", []byte{0xCC, 0x34, 0x12}},
		{"CALL $1234", []byte{0xCD, 0x34, 0x12}},
		// CE: ADC A, imm - tested above
		{"RST $08", []byte{0xCF}},
		{"RET NC", []byte{0xD0}},
		{"POP DE", []byte{0xD1}},
		{"JP NC, $1234", []byte{0xD2, 0x34, 0x12}},
		{"OUT ($44), A", []byte{0xD3, 0x44}},
		{"CALL NC, $1234", []byte{0xD4, 0x34, 0x12}},
		{"PUSH DE", []byte{0xD5}},
		// D6: SUB imm - tested above
		{"RST $10", []byte{0xD7}},
		{"RET C", []byte{0xD8}},
		{"EXX", []byte{0xD9}},
		{"JP C, $1234", []byte{0xDA, 0x34, 0x12}},
		{"IN A, ($44)", []byte{0xDB, 0x44}},
		{"CALL C, $1234", []byte{0xDC, 0x34, 0x12}},
		// DD: IX prefix - tested separately
		// DE: SBC A, imm - tested above
		{"RST $18", []byte{0xDF}},
		{"RET PO", []byte{0xE0}},
		{"POP HL", []byte{0xE1}},
		{"JP PO, $1234", []byte{0xE2, 0x34, 0x12}},
		{"EX (SP), HL", []byte{0xE3}},
		{"CALL PO, $1234", []byte{0xE4, 0x34, 0x12}},
		{"PUSH HL", []byte{0xE5}},
		// E6: AND imm - tested above
		{"RST $20", []byte{0xE7}},
		{"RET PE", []byte{0xE8}},
		{"JP (HL)", []byte{0xE9}},
		{"JP PE, $1234", []byte{0xEA, 0x34, 0x12}},
		{"EX DE, HL", []byte{0xEB}},
		{"CALL PE, $1234", []byte{0xEC, 0x34, 0x12}},
		// ED: Extended prefix - tested separately
		// EE: XOR imm - tested above
		{"RST $28", []byte{0xEF}},
		{"RET P", []byte{0xF0}},
		{"POP AF", []byte{0xF1}},
		{"JP P, $1234", []byte{0xF2, 0x34, 0x12}},
		{"DI", []byte{0xF3}},
		{"CALL P, $1234", []byte{0xF4, 0x34, 0x12}},
		{"PUSH AF", []byte{0xF5}},
		// F6: OR imm - tested above
		{"RST $30", []byte{0xF7}},
		{"RET M", []byte{0xF8}},
		{"LD SP, HL", []byte{0xF9}},
		{"JP M, $1234", []byte{0xFA, 0x34, 0x12}},
		{"EI", []byte{0xFB}},
		{"CALL M, $1234", []byte{0xFC, 0x34, 0x12}},
		// FD: IY prefix - tested separately
		// FE: CP imm - tested above
		{"RST $38", []byte{0xFF}},
	}

	for _, tt := range tests {
		t.Run(tt.inst, func(t *testing.T) {
			got := assembleOne(t, tt.inst)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("%s: got %X, want %X", tt.inst, got, tt.expected)
			}
		})
	}
}

// --- ED-prefix instructions (sjasmplus op_ED.asm) ---

func TestOpcodes_ED(t *testing.T) {
	tests := []struct {
		inst     string
		expected []byte
	}{
		{"IN B, (C)", []byte{0xED, 0x40}},
		{"OUT (C), B", []byte{0xED, 0x41}},
		{"SBC HL, BC", []byte{0xED, 0x42}},
		{"LD ($1234), BC", []byte{0xED, 0x43, 0x34, 0x12}},
		{"NEG", []byte{0xED, 0x44}},
		{"RETN", []byte{0xED, 0x45}},
		{"IM 0", []byte{0xED, 0x46}},
		{"LD I, A", []byte{0xED, 0x47}},
		{"IN C, (C)", []byte{0xED, 0x48}},
		{"OUT (C), C", []byte{0xED, 0x49}},
		{"ADC HL, BC", []byte{0xED, 0x4A}},
		{"LD BC, ($1234)", []byte{0xED, 0x4B, 0x34, 0x12}},
		{"RETI", []byte{0xED, 0x4D}},
		{"LD R, A", []byte{0xED, 0x4F}},
		{"IN D, (C)", []byte{0xED, 0x50}},
		{"OUT (C), D", []byte{0xED, 0x51}},
		{"SBC HL, DE", []byte{0xED, 0x52}},
		{"LD ($1234), DE", []byte{0xED, 0x53, 0x34, 0x12}},
		{"IM 1", []byte{0xED, 0x56}},
		{"LD A, I", []byte{0xED, 0x57}},
		{"IN E, (C)", []byte{0xED, 0x58}},
		{"OUT (C), E", []byte{0xED, 0x59}},
		{"ADC HL, DE", []byte{0xED, 0x5A}},
		{"LD DE, ($1234)", []byte{0xED, 0x5B, 0x34, 0x12}},
		{"IM 2", []byte{0xED, 0x5E}},
		{"LD A, R", []byte{0xED, 0x5F}},
		{"IN H, (C)", []byte{0xED, 0x60}},
		{"OUT (C), H", []byte{0xED, 0x61}},
		{"SBC HL, HL", []byte{0xED, 0x62}},
		{"RRD", []byte{0xED, 0x67}},
		{"IN L, (C)", []byte{0xED, 0x68}},
		{"OUT (C), L", []byte{0xED, 0x69}},
		{"ADC HL, HL", []byte{0xED, 0x6A}},
		{"RLD", []byte{0xED, 0x6F}},
		{"SBC HL, SP", []byte{0xED, 0x72}},
		{"LD ($1234), SP", []byte{0xED, 0x73, 0x34, 0x12}},
		{"IN A, (C)", []byte{0xED, 0x78}},
		{"OUT (C), A", []byte{0xED, 0x79}},
		{"ADC HL, SP", []byte{0xED, 0x7A}},
		{"LD SP, ($1234)", []byte{0xED, 0x7B, 0x34, 0x12}},
		// Block transfer/search
		{"LDI", []byte{0xED, 0xA0}},
		{"CPI", []byte{0xED, 0xA1}},
		{"INI", []byte{0xED, 0xA2}},
		{"OUTI", []byte{0xED, 0xA3}},
		{"LDD", []byte{0xED, 0xA8}},
		{"CPD", []byte{0xED, 0xA9}},
		{"IND", []byte{0xED, 0xAA}},
		{"OUTD", []byte{0xED, 0xAB}},
		{"LDIR", []byte{0xED, 0xB0}},
		{"CPIR", []byte{0xED, 0xB1}},
		{"INIR", []byte{0xED, 0xB2}},
		{"OTIR", []byte{0xED, 0xB3}},
		{"LDDR", []byte{0xED, 0xB8}},
		{"CPDR", []byte{0xED, 0xB9}},
		{"INDR", []byte{0xED, 0xBA}},
		{"OTDR", []byte{0xED, 0xBB}},
	}

	for _, tt := range tests {
		t.Run(tt.inst, func(t *testing.T) {
			got := assembleOne(t, tt.inst)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("%s: got %X, want %X", tt.inst, got, tt.expected)
			}
		})
	}
}

// --- CB-prefix: shift/rotate instructions 0xCB00-0xCB3F ---

func TestOpcodes_CB_ShiftRotate(t *testing.T) {
	ops := []struct {
		mnemonic string
		base     byte
	}{
		{"RLC", 0x00},
		{"RRC", 0x08},
		{"RL", 0x10},
		{"RR", 0x18},
		{"SLA", 0x20},
		{"SRA", 0x28},
		// SLI/SLL at 0x30 tested separately (undocumented)
		{"SRL", 0x38},
	}
	regs := []string{"B", "C", "D", "E", "H", "L", "(HL)", "A"}

	for _, op := range ops {
		for regIdx, reg := range regs {
			opcode := op.base + byte(regIdx)
			inst := fmt.Sprintf("%s %s", op.mnemonic, reg)

			t.Run(inst, func(t *testing.T) {
				got := assembleOne(t, inst)
				expected := []byte{0xCB, opcode}
				if !bytes.Equal(got, expected) {
					t.Errorf("%s: got %X, want %X", inst, got, expected)
				}
			})
		}
	}
}

// --- CB-prefix: BIT/RES/SET instructions 0xCB40-0xCBFF ---

func TestOpcodes_CB_BitResSet(t *testing.T) {
	ops := []struct {
		mnemonic string
		base     byte
	}{
		{"BIT", 0x40},
		{"RES", 0x80},
		{"SET", 0xC0},
	}
	regs := []string{"B", "C", "D", "E", "H", "L", "(HL)", "A"}

	for _, op := range ops {
		for bit := 0; bit < 8; bit++ {
			for regIdx, reg := range regs {
				opcode := op.base + byte(bit*8) + byte(regIdx)
				inst := fmt.Sprintf("%s %d, %s", op.mnemonic, bit, reg)

				t.Run(inst, func(t *testing.T) {
					got := assembleOne(t, inst)
					expected := []byte{0xCB, opcode}
					if !bytes.Equal(got, expected) {
						t.Errorf("%s: got %X, want %X", inst, got, expected)
					}
				})
			}
		}
	}
}

// --- DD-prefix: IX instructions (sjasmplus op_IX_DD.asm) ---

func TestOpcodes_DD_IX(t *testing.T) {
	tests := []struct {
		inst     string
		expected []byte
	}{
		// 16-bit arithmetic
		{"ADD IX, BC", []byte{0xDD, 0x09}},
		{"ADD IX, DE", []byte{0xDD, 0x19}},
		{"ADD IX, IX", []byte{0xDD, 0x29}},
		{"ADD IX, SP", []byte{0xDD, 0x39}},
		// LD IX, nn
		{"LD IX, $1234", []byte{0xDD, 0x21, 0x34, 0x12}},
		// LD (nn), IX
		{"LD ($1234), IX", []byte{0xDD, 0x22, 0x34, 0x12}},
		// INC/DEC IX
		{"INC IX", []byte{0xDD, 0x23}},
		{"DEC IX", []byte{0xDD, 0x2B}},
		// LD IX, (nn)
		{"LD IX, ($1234)", []byte{0xDD, 0x2A, 0x34, 0x12}},
		// Indexed addressing: LD r, (IX+d)
		{"LD B, (IX+17)", []byte{0xDD, 0x46, 0x11}},
		{"LD C, (IX+17)", []byte{0xDD, 0x4E, 0x11}},
		{"LD D, (IX+17)", []byte{0xDD, 0x56, 0x11}},
		{"LD E, (IX+17)", []byte{0xDD, 0x5E, 0x11}},
		{"LD H, (IX+17)", []byte{0xDD, 0x66, 0x11}},
		{"LD L, (IX+17)", []byte{0xDD, 0x6E, 0x11}},
		{"LD A, (IX+17)", []byte{0xDD, 0x7E, 0x11}},
		// LD (IX+d), r
		{"LD (IX+17), B", []byte{0xDD, 0x70, 0x11}},
		{"LD (IX+17), C", []byte{0xDD, 0x71, 0x11}},
		{"LD (IX+17), D", []byte{0xDD, 0x72, 0x11}},
		{"LD (IX+17), E", []byte{0xDD, 0x73, 0x11}},
		{"LD (IX+17), H", []byte{0xDD, 0x74, 0x11}},
		{"LD (IX+17), L", []byte{0xDD, 0x75, 0x11}},
		{"LD (IX+17), A", []byte{0xDD, 0x77, 0x11}},
		// LD (IX+d), n
		{"LD (IX+17), 0", []byte{0xDD, 0x36, 0x11, 0x00}},
		{"LD (IX+17), $55", []byte{0xDD, 0x36, 0x11, 0x55}},
		// INC/DEC (IX+d)
		{"INC (IX+17)", []byte{0xDD, 0x34, 0x11}},
		{"DEC (IX+17)", []byte{0xDD, 0x35, 0x11}},
		// ALU A, (IX+d)
		{"ADD A, (IX+17)", []byte{0xDD, 0x86, 0x11}},
		{"ADC A, (IX+17)", []byte{0xDD, 0x8E, 0x11}},
		{"SUB (IX+17)", []byte{0xDD, 0x96, 0x11}},
		{"SBC A, (IX+17)", []byte{0xDD, 0x9E, 0x11}},
		{"AND (IX+17)", []byte{0xDD, 0xA6, 0x11}},
		{"XOR (IX+17)", []byte{0xDD, 0xAE, 0x11}},
		{"OR (IX+17)", []byte{0xDD, 0xB6, 0x11}},
		{"CP (IX+17)", []byte{0xDD, 0xBE, 0x11}},
		// Stack/misc
		{"POP IX", []byte{0xDD, 0xE1}},
		{"EX (SP), IX", []byte{0xDD, 0xE3}},
		{"PUSH IX", []byte{0xDD, 0xE5}},
		{"JP (IX)", []byte{0xDD, 0xE9}},
		{"LD SP, IX", []byte{0xDD, 0xF9}},
		// Negative displacement
		{"LD A, (IX-1)", []byte{0xDD, 0x7E, 0xFF}},
		{"LD A, (IX+0)", []byte{0xDD, 0x7E, 0x00}},
		{"LD A, (IX+127)", []byte{0xDD, 0x7E, 0x7F}},
	}

	for _, tt := range tests {
		t.Run(tt.inst, func(t *testing.T) {
			got := assembleOne(t, tt.inst)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("%s: got %X, want %X", tt.inst, got, tt.expected)
			}
		})
	}
}

// --- FD-prefix: IY instructions (sjasmplus op_IY_FD.asm) ---

func TestOpcodes_FD_IY(t *testing.T) {
	tests := []struct {
		inst     string
		expected []byte
	}{
		// 16-bit arithmetic
		{"ADD IY, BC", []byte{0xFD, 0x09}},
		{"ADD IY, DE", []byte{0xFD, 0x19}},
		{"ADD IY, IY", []byte{0xFD, 0x29}},
		{"ADD IY, SP", []byte{0xFD, 0x39}},
		// LD IY, nn
		{"LD IY, $1234", []byte{0xFD, 0x21, 0x34, 0x12}},
		// LD (nn), IY
		{"LD ($1234), IY", []byte{0xFD, 0x22, 0x34, 0x12}},
		// INC/DEC IY
		{"INC IY", []byte{0xFD, 0x23}},
		{"DEC IY", []byte{0xFD, 0x2B}},
		// LD IY, (nn)
		{"LD IY, ($1234)", []byte{0xFD, 0x2A, 0x34, 0x12}},
		// Indexed addressing: LD r, (IY+d)
		{"LD B, (IY+17)", []byte{0xFD, 0x46, 0x11}},
		{"LD C, (IY+17)", []byte{0xFD, 0x4E, 0x11}},
		{"LD D, (IY+17)", []byte{0xFD, 0x56, 0x11}},
		{"LD E, (IY+17)", []byte{0xFD, 0x5E, 0x11}},
		{"LD H, (IY+17)", []byte{0xFD, 0x66, 0x11}},
		{"LD L, (IY+17)", []byte{0xFD, 0x6E, 0x11}},
		{"LD A, (IY+17)", []byte{0xFD, 0x7E, 0x11}},
		// LD (IY+d), r
		{"LD (IY+17), B", []byte{0xFD, 0x70, 0x11}},
		{"LD (IY+17), C", []byte{0xFD, 0x71, 0x11}},
		{"LD (IY+17), D", []byte{0xFD, 0x72, 0x11}},
		{"LD (IY+17), E", []byte{0xFD, 0x73, 0x11}},
		{"LD (IY+17), H", []byte{0xFD, 0x74, 0x11}},
		{"LD (IY+17), L", []byte{0xFD, 0x75, 0x11}},
		{"LD (IY+17), A", []byte{0xFD, 0x77, 0x11}},
		// LD (IY+d), n
		{"LD (IY+17), 0", []byte{0xFD, 0x36, 0x11, 0x00}},
		// INC/DEC (IY+d)
		{"INC (IY+17)", []byte{0xFD, 0x34, 0x11}},
		{"DEC (IY+17)", []byte{0xFD, 0x35, 0x11}},
		// ALU A, (IY+d)
		{"ADD A, (IY+17)", []byte{0xFD, 0x86, 0x11}},
		{"ADC A, (IY+17)", []byte{0xFD, 0x8E, 0x11}},
		{"SUB (IY+17)", []byte{0xFD, 0x96, 0x11}},
		{"SBC A, (IY+17)", []byte{0xFD, 0x9E, 0x11}},
		{"AND (IY+17)", []byte{0xFD, 0xA6, 0x11}},
		{"XOR (IY+17)", []byte{0xFD, 0xAE, 0x11}},
		{"OR (IY+17)", []byte{0xFD, 0xB6, 0x11}},
		{"CP (IY+17)", []byte{0xFD, 0xBE, 0x11}},
		// Stack/misc
		{"POP IY", []byte{0xFD, 0xE1}},
		{"EX (SP), IY", []byte{0xFD, 0xE3}},
		{"PUSH IY", []byte{0xFD, 0xE5}},
		{"JP (IY)", []byte{0xFD, 0xE9}},
		{"LD SP, IY", []byte{0xFD, 0xF9}},
		// Negative displacement
		{"LD A, (IY-1)", []byte{0xFD, 0x7E, 0xFF}},
	}

	for _, tt := range tests {
		t.Run(tt.inst, func(t *testing.T) {
			got := assembleOne(t, tt.inst)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("%s: got %X, want %X", tt.inst, got, tt.expected)
			}
		})
	}
}

// --- DDCB/FDCB: Indexed bit instructions ---

func TestOpcodes_DDCB_IX_ShiftRotate(t *testing.T) {
	ops := []struct {
		mnemonic string
		base     byte
	}{
		{"RLC", 0x06},
		{"RRC", 0x0E},
		{"RL", 0x16},
		{"RR", 0x1E},
		{"SLA", 0x26},
		{"SRA", 0x2E},
		// SLI/SLL at 0x36 tested separately
		{"SRL", 0x3E},
	}

	for _, op := range ops {
		inst := fmt.Sprintf("%s (IX+17)", op.mnemonic)
		t.Run(inst, func(t *testing.T) {
			got := assembleOne(t, inst)
			expected := []byte{0xDD, 0xCB, 0x11, op.base}
			if !bytes.Equal(got, expected) {
				t.Errorf("%s: got %X, want %X", inst, got, expected)
			}
		})
	}
}

func TestOpcodes_DDCB_IX_BitResSet(t *testing.T) {
	ops := []struct {
		mnemonic string
		base     byte
	}{
		{"BIT", 0x46},
		{"RES", 0x86},
		{"SET", 0xC6},
	}

	for _, op := range ops {
		for bit := 0; bit < 8; bit++ {
			opcode := op.base + byte(bit*8)
			inst := fmt.Sprintf("%s %d, (IX+17)", op.mnemonic, bit)

			t.Run(inst, func(t *testing.T) {
				got := assembleOne(t, inst)
				expected := []byte{0xDD, 0xCB, 0x11, opcode}
				if !bytes.Equal(got, expected) {
					t.Errorf("%s: got %X, want %X", inst, got, expected)
				}
			})
		}
	}
}

func TestOpcodes_FDCB_IY_ShiftRotate(t *testing.T) {
	ops := []struct {
		mnemonic string
		base     byte
	}{
		{"RLC", 0x06},
		{"RRC", 0x0E},
		{"RL", 0x16},
		{"RR", 0x1E},
		{"SLA", 0x26},
		{"SRA", 0x2E},
		{"SRL", 0x3E},
	}

	for _, op := range ops {
		inst := fmt.Sprintf("%s (IY+17)", op.mnemonic)
		t.Run(inst, func(t *testing.T) {
			got := assembleOne(t, inst)
			expected := []byte{0xFD, 0xCB, 0x11, op.base}
			if !bytes.Equal(got, expected) {
				t.Errorf("%s: got %X, want %X", inst, got, expected)
			}
		})
	}
}

func TestOpcodes_FDCB_IY_BitResSet(t *testing.T) {
	ops := []struct {
		mnemonic string
		base     byte
	}{
		{"BIT", 0x46},
		{"RES", 0x86},
		{"SET", 0xC6},
	}

	for _, op := range ops {
		for bit := 0; bit < 8; bit++ {
			opcode := op.base + byte(bit*8)
			inst := fmt.Sprintf("%s %d, (IY+17)", op.mnemonic, bit)

			t.Run(inst, func(t *testing.T) {
				got := assembleOne(t, inst)
				expected := []byte{0xFD, 0xCB, 0x11, opcode}
				if !bytes.Equal(got, expected) {
					t.Errorf("%s: got %X, want %X", inst, got, expected)
				}
			})
		}
	}
}

// --- Expression evaluation tests ---

func TestExpressionOperators(t *testing.T) {
	tests := []struct {
		inst     string
		expected []byte
	}{
		// Basic arithmetic
		{"LD A, 2+3", []byte{0x3E, 0x05}},
		{"LD A, 10-3", []byte{0x3E, 0x07}},
		{"LD A, 4*5", []byte{0x3E, 0x14}},
		{"LD A, 20/4", []byte{0x3E, 0x05}},
		// Shift operators
		{"LD A, 1<<4", []byte{0x3E, 0x10}},
		{"LD A, $80>>3", []byte{0x3E, 0x10}},
		// Bitwise operators
		{"LD A, $FF&$0F", []byte{0x3E, 0x0F}},
		{"LD A, $F0|$0F", []byte{0x3E, 0xFF}},
		// Modulo
		{"LD A, 17%5", []byte{0x3E, 0x02}},
		// Hex formats
		{"LD A, #FF", []byte{0x3E, 0xFF}},
		{"LD A, 0xFF", []byte{0x3E, 0xFF}},
		{"LD A, $FF", []byte{0x3E, 0xFF}},
		// Binary format
		{"LD A, %11110000", []byte{0x3E, 0xF0}},
		{"LD A, 0b10101010", []byte{0x3E, 0xAA}},
		// Character literal
		{"LD A, 'A'", []byte{0x3E, 0x41}},
	}

	for _, tt := range tests {
		t.Run(tt.inst, func(t *testing.T) {
			got := assembleOne(t, tt.inst)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("%s: got %X, want %X", tt.inst, got, tt.expected)
			}
		})
	}
}

// --- Directive tests ---

func TestDirectives(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected []byte
	}{
		{
			name:     "DB positive values",
			source:   "ORG $8000\nDB 0, 1, 127, 255",
			expected: []byte{0x00, 0x01, 0x7F, 0xFF},
		},
		{
			name:     "DB negative values",
			source:   "ORG $8000\nDB -1, -128",
			expected: []byte{0xFF, 0x80},
		},
		{
			name:     "DW little-endian",
			source:   "ORG $8000\nDW $1234, $5678",
			expected: []byte{0x34, 0x12, 0x78, 0x56},
		},
		{
			name:     "DS fill",
			source:   "ORG $8000\nDS 3, $AA",
			expected: []byte{0xAA, 0xAA, 0xAA},
		},
		{
			name:     "DB string",
			source:   "ORG $8000\nDB \"AB\"",
			expected: []byte{0x41, 0x42},
		},
		{
			name:     "EQU constants",
			source:   "ORG $8000\nVAL EQU 42\nLD A, VAL",
			expected: []byte{0x3E, 0x2A},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asm := NewAssembler()
			result, err := asm.AssembleString(tt.source)
			if err != nil {
				t.Fatalf("Assembly failed: %v", err)
			}
			if !bytes.Equal(result.Binary, tt.expected) {
				t.Errorf("Binary mismatch:\ngot:  %X\nwant: %X", result.Binary, tt.expected)
			}
		})
	}
}

// --- Label resolution tests ---

func TestLabelResolution(t *testing.T) {
	tests := []struct {
		name     string
		source   string
		expected []byte
	}{
		{
			name: "forward reference JP",
			source: `
				ORG $8000
				JP target
				NOP
			target:
				RET
			`,
			expected: []byte{0xC3, 0x04, 0x80, 0x00, 0xC9},
		},
		{
			name: "backward reference JR",
			source: `
				ORG $8000
			loop:
				NOP
				JR loop
			`,
			expected: []byte{0x00, 0x18, 0xFD},
		},
		{
			name: "current address $",
			source: `
				ORG $8000
				JP $
			`,
			expected: []byte{0xC3, 0x00, 0x80},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			asm := NewAssembler()
			result, err := asm.AssembleString(tt.source)
			if err != nil {
				t.Fatalf("Assembly failed: %v", err)
			}
			if !bytes.Equal(result.Binary, tt.expected) {
				t.Errorf("Binary mismatch:\ngot:  %X\nwant: %X", result.Binary, tt.expected)
			}
		})
	}
}

// --- Case insensitivity test ---

func TestCaseInsensitivity(t *testing.T) {
	tests := []struct {
		inst     string
		expected []byte
	}{
		{"nop", []byte{0x00}},
		{"Nop", []byte{0x00}},
		{"ld a, b", []byte{0x78}},
		{"LD a, B", []byte{0x78}},
		{"ld hl, $1234", []byte{0x21, 0x34, 0x12}},
		{"add a, (hl)", []byte{0x86}},
		{"push bc", []byte{0xC5}},
	}

	for _, tt := range tests {
		t.Run(tt.inst, func(t *testing.T) {
			got := assembleOne(t, tt.inst)
			if !bytes.Equal(got, tt.expected) {
				t.Errorf("%s: got %X, want %X", tt.inst, got, tt.expected)
			}
		})
	}
}
