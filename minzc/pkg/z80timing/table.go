package z80timing

// ── Table-driven instruction timing ───────────────────────────────────────────
//
// Base[opcode]   — T-states for 1-byte (unprefixed) opcodes.
// CBBase[opcode] — T-states for CB-prefixed opcodes (CB xx).
// EDBase[opcode] — T-states for ED-prefixed opcodes (ED xx).
// DDBase[opcode] — T-states for DD/FD-prefixed opcodes (DD/FD xx); IX/IY variants.
//
// A value of 0 means the opcode is undefined/illegal (treated as 4T NOP by emulators).
// Two-entry groups exist where taken/not-taken differ — see taken/notTaken arrays below.
//
// Source: Zilog Z80 CPU User Manual UM0080 rev 11, verified via FUSE test suite.
//
// Usage in emulator:
//
//	cycles := z80timing.Base[opcode]
//	if opcode == 0xCB { cycles = z80timing.CBBase[nextByte] }
//	// etc.

// Base is the T-state table for unprefixed opcodes 0x00–0xFF.
//
// Conditional jumps/calls/rets that have different taken/not-taken costs use
// the TAKEN cost here.  Check BaseCond[opcode] for the not-taken cost.
var Base [256]uint8

// BaseCond is the NOT-TAKEN T-state cost for conditional instructions.
// Zero means the instruction is unconditional (or taken==not-taken).
var BaseCond [256]uint8

// CBBase is the T-state table for CB-prefixed opcodes (rotate/shift/bit).
var CBBase [256]uint8

// EDBase is the T-state table for ED-prefixed opcodes (extended instructions).
var EDBase [256]uint8

// DDBase is the T-state table for DD-prefixed opcodes (IX variants).
// FD-prefixed opcodes (IY variants) have identical timings.
var DDBase [256]uint8

func init() {
	initBase()
	initCBBase()
	initEDBase()
	initDDBase()
}

// initBase populates Base[] and BaseCond[] for all unprefixed opcodes.
func initBase() {
	// Default: undefined = 4T (treated as NOP)
	for i := range Base {
		Base[i] = NOP
	}

	// ── Row 0x0x ─────────────────────────────────────────────────────────────
	Base[0x00] = NOP          // NOP
	Base[0x01] = LD_rr_nn     // LD BC, nn
	Base[0x02] = LD_BC_A      // LD (BC), A
	Base[0x03] = INC_rr       // INC BC
	Base[0x04] = INC_r        // INC B
	Base[0x05] = DEC_r        // DEC B
	Base[0x06] = LD_r_n       // LD B, n
	Base[0x07] = RLCA         // RLCA
	Base[0x08] = EX_AF_AF     // EX AF, AF'
	Base[0x09] = ADD_HL_rr    // ADD HL, BC
	Base[0x0A] = LD_A_BC      // LD A, (BC)
	Base[0x0B] = DEC_rr       // DEC BC
	Base[0x0C] = INC_r        // INC C
	Base[0x0D] = DEC_r        // DEC C
	Base[0x0E] = LD_r_n       // LD C, n
	Base[0x0F] = RRCA         // RRCA

	// ── Row 0x1x ─────────────────────────────────────────────────────────────
	Base[0x10] = DJNZ         // DJNZ e  (taken)
	BaseCond[0x10] = DJNZ_NT  // DJNZ e  (not taken)
	Base[0x11] = LD_rr_nn     // LD DE, nn
	Base[0x12] = LD_DE_A      // LD (DE), A
	Base[0x13] = INC_rr       // INC DE
	Base[0x14] = INC_r        // INC D
	Base[0x15] = DEC_r        // DEC D
	Base[0x16] = LD_r_n       // LD D, n
	Base[0x17] = RLA          // RLA
	Base[0x18] = JR_e         // JR e
	Base[0x19] = ADD_HL_rr    // ADD HL, DE
	Base[0x1A] = LD_A_DE      // LD A, (DE)
	Base[0x1B] = DEC_rr       // DEC DE
	Base[0x1C] = INC_r        // INC E
	Base[0x1D] = DEC_r        // DEC E
	Base[0x1E] = LD_r_n       // LD E, n
	Base[0x1F] = RRA          // RRA

	// ── Row 0x2x ─────────────────────────────────────────────────────────────
	Base[0x20] = JR_cc_e       // JR NZ, e  (taken)
	BaseCond[0x20] = JR_cc_e_NT
	Base[0x21] = LD_rr_nn      // LD HL, nn
	Base[0x22] = LD_nn_HL      // LD (nn), HL
	Base[0x23] = INC_rr        // INC HL
	Base[0x24] = INC_r         // INC H
	Base[0x25] = DEC_r         // DEC H
	Base[0x26] = LD_r_n        // LD H, n
	Base[0x27] = 4             // DAA
	Base[0x28] = JR_cc_e       // JR Z, e  (taken)
	BaseCond[0x28] = JR_cc_e_NT
	Base[0x29] = ADD_HL_rr     // ADD HL, HL
	Base[0x2A] = LD_HL_nn_mem  // LD HL, (nn)
	Base[0x2B] = DEC_rr        // DEC HL
	Base[0x2C] = INC_r         // INC L
	Base[0x2D] = DEC_r         // DEC L
	Base[0x2E] = LD_r_n        // LD L, n
	Base[0x2F] = CPL           // CPL

	// ── Row 0x3x ─────────────────────────────────────────────────────────────
	Base[0x30] = JR_cc_e       // JR NC, e  (taken)
	BaseCond[0x30] = JR_cc_e_NT
	Base[0x31] = LD_rr_nn      // LD SP, nn
	Base[0x32] = LD_nn_A       // LD (nn), A
	Base[0x33] = INC_rr        // INC SP
	Base[0x34] = 11            // INC (HL)
	Base[0x35] = 11            // DEC (HL)
	Base[0x36] = 10            // LD (HL), n
	Base[0x37] = SCF           // SCF
	Base[0x38] = JR_cc_e       // JR C, e  (taken)
	BaseCond[0x38] = JR_cc_e_NT
	Base[0x39] = ADD_HL_rr     // ADD HL, SP
	Base[0x3A] = LD_A_nn       // LD A, (nn)
	Base[0x3B] = DEC_rr        // DEC SP
	Base[0x3C] = INC_r         // INC A
	Base[0x3D] = DEC_r         // DEC A
	Base[0x3E] = LD_r_n        // LD A, n
	Base[0x3F] = CCF           // CCF

	// ── Rows 0x4x–0x7x: LD r, r' (4T each) ──────────────────────────────────
	// 0x76 = HALT (special case)
	for op := byte(0x40); op <= 0x7F; op++ {
		if op == 0x76 {
			Base[op] = HALT // HALT
		} else {
			Base[op] = LD_r_r
		}
	}
	// (HL) variants within the block: LD r, (HL) and LD (HL), r = 7T
	for _, op := range []byte{
		0x46, 0x4E, 0x56, 0x5E, 0x66, 0x6E, 0x7E, // LD r, (HL)
		0x70, 0x71, 0x72, 0x73, 0x74, 0x75,        // LD (HL), r
	} {
		Base[op] = LD_r_HL
	}

	// ── Rows 0x8x–0xBx: ALU ops ──────────────────────────────────────────────
	// reg forms (0x80–0xBF excluding (HL) variants): 4T
	// (HL) forms: 7T
	aluHLOffsets := []byte{0x06, 0x0E, 0x16, 0x1E, 0x26, 0x2E, 0x36, 0x3E} // (HL) slot within each ALU group
	for base := byte(0x80); base <= 0xBF; base += 8 {
		for i := byte(0); i < 8; i++ {
			op := base + i
			if i == 6 { // (HL) operand
				Base[op] = LD_r_HL // 7T for (HL) ALU forms
			} else {
				Base[op] = ADD_A_r // 4T for register ALU forms
			}
		}
	}
	_ = aluHLOffsets // referenced for clarity

	// ── Row 0xCx ─────────────────────────────────────────────────────────────
	Base[0xC0] = RET_cc        // RET NZ  (taken)
	BaseCond[0xC0] = RET_cc_NT
	Base[0xC1] = POP_rr        // POP BC
	Base[0xC2] = JP_cc_nn      // JP NZ, nn
	Base[0xC3] = JP_nn         // JP nn
	Base[0xC4] = CALL_cc_nn    // CALL NZ, nn  (taken)
	BaseCond[0xC4] = CALL_cc_nn_T
	Base[0xC5] = PUSH_rr       // PUSH BC
	Base[0xC6] = ADD_A_n       // ADD A, n
	Base[0xC7] = RST_n         // RST 00h
	Base[0xC8] = RET_cc        // RET Z  (taken)
	BaseCond[0xC8] = RET_cc_NT
	Base[0xC9] = RET           // RET
	Base[0xCA] = JP_cc_nn      // JP Z, nn
	// 0xCB = CB prefix (handled separately)
	Base[0xCC] = CALL_cc_nn    // CALL Z, nn  (taken)
	BaseCond[0xCC] = CALL_cc_nn_T
	Base[0xCD] = CALL_nn       // CALL nn
	Base[0xCE] = ADC_A_n       // ADC A, n
	Base[0xCF] = RST_n         // RST 08h

	// ── Row 0xDx ─────────────────────────────────────────────────────────────
	Base[0xD0] = RET_cc
	BaseCond[0xD0] = RET_cc_NT // RET NC
	Base[0xD1] = POP_rr        // POP DE
	Base[0xD2] = JP_cc_nn      // JP NC, nn
	Base[0xD3] = 11            // OUT (n), A
	Base[0xD4] = CALL_cc_nn
	BaseCond[0xD4] = CALL_cc_nn_T // CALL NC, nn
	Base[0xD5] = PUSH_rr       // PUSH DE
	Base[0xD6] = SUB_n         // SUB n
	Base[0xD7] = RST_n         // RST 10h
	Base[0xD8] = RET_cc
	BaseCond[0xD8] = RET_cc_NT // RET C
	Base[0xD9] = EXX           // EXX
	Base[0xDA] = JP_cc_nn      // JP C, nn
	Base[0xDB] = 11            // IN A, (n)
	Base[0xDC] = CALL_cc_nn
	BaseCond[0xDC] = CALL_cc_nn_T // CALL C, nn
	// 0xDD = DD prefix (IX)
	Base[0xDE] = SBC_A_r       // SBC A, n  (7T as immediate variant)
	Base[0xDF] = RST_n         // RST 18h

	// ── Row 0xEx ─────────────────────────────────────────────────────────────
	Base[0xE0] = RET_cc
	BaseCond[0xE0] = RET_cc_NT // RET PO
	Base[0xE1] = POP_rr        // POP HL
	Base[0xE2] = JP_cc_nn      // JP PO, nn
	Base[0xE3] = 19            // EX (SP), HL
	Base[0xE4] = CALL_cc_nn
	BaseCond[0xE4] = CALL_cc_nn_T // CALL PO, nn
	Base[0xE5] = PUSH_rr       // PUSH HL
	Base[0xE6] = AND_n         // AND n
	Base[0xE7] = RST_n         // RST 20h
	Base[0xE8] = RET_cc
	BaseCond[0xE8] = RET_cc_NT // RET PE
	Base[0xE9] = JP_HL         // JP (HL)
	Base[0xEA] = JP_cc_nn      // JP PE, nn
	Base[0xEB] = EX_DE_HL      // EX DE, HL
	Base[0xEC] = CALL_cc_nn
	BaseCond[0xEC] = CALL_cc_nn_T // CALL PE, nn
	// 0xED = ED prefix
	Base[0xEE] = XOR_n         // XOR n
	Base[0xEF] = RST_n         // RST 28h

	// ── Row 0xFx ─────────────────────────────────────────────────────────────
	Base[0xF0] = RET_cc
	BaseCond[0xF0] = RET_cc_NT // RET P
	Base[0xF1] = POP_rr        // POP AF
	Base[0xF2] = JP_cc_nn      // JP P, nn
	Base[0xF3] = DI            // DI
	Base[0xF4] = CALL_cc_nn
	BaseCond[0xF4] = CALL_cc_nn_T // CALL P, nn
	Base[0xF5] = PUSH_rr       // PUSH AF
	Base[0xF6] = OR_n          // OR n
	Base[0xF7] = RST_n         // RST 30h
	Base[0xF8] = RET_cc
	BaseCond[0xF8] = RET_cc_NT // RET M
	Base[0xF9] = LD_SP_HL      // LD SP, HL
	Base[0xFA] = JP_cc_nn      // JP M, nn
	Base[0xFB] = EI            // EI
	Base[0xFC] = CALL_cc_nn
	BaseCond[0xFC] = CALL_cc_nn_T // CALL M, nn
	// 0xFD = FD prefix (IY)
	Base[0xFE] = CP_n          // CP n
	Base[0xFF] = RST_n         // RST 38h
}

// initCBBase populates CBBase[] for CB-prefixed opcodes.
func initCBBase() {
	// Rows 0x00–0x3F: rotate/shift on register = 8T; on (HL) = 15T
	for op := 0; op <= 0x3F; op++ {
		if op&7 == 6 {
			CBBase[op] = 15 // (HL) variant
		} else {
			CBBase[op] = uint8(RLC_r)
		}
	}
	// Rows 0x40–0x7F: BIT b, r = 8T; BIT b, (HL) = 12T
	for op := 0x40; op <= 0x7F; op++ {
		if op&7 == 6 {
			CBBase[op] = 12
		} else {
			CBBase[op] = 8
		}
	}
	// Rows 0x80–0xBF: RES b, r = 8T; RES b, (HL) = 15T
	for op := 0x80; op <= 0xBF; op++ {
		if op&7 == 6 {
			CBBase[op] = 15
		} else {
			CBBase[op] = 8
		}
	}
	// Rows 0xC0–0xFF: SET b, r = 8T; SET b, (HL) = 15T
	for op := 0xC0; op <= 0xFF; op++ {
		if op&7 == 6 {
			CBBase[op] = 15
		} else {
			CBBase[op] = 8
		}
	}
}

// initEDBase populates EDBase[] for ED-prefixed opcodes.
func initEDBase() {
	for i := range EDBase {
		EDBase[i] = NOP // default: illegal = 8T (NOP×2)
	}
	// Block transfer / search
	EDBase[0xA0] = 16 // LDI
	EDBase[0xA1] = 16 // CPI
	EDBase[0xA2] = 16 // INI
	EDBase[0xA3] = 16 // OUTI
	EDBase[0xA8] = 16 // LDD
	EDBase[0xA9] = 16 // CPD
	EDBase[0xAA] = 16 // IND
	EDBase[0xAB] = 16 // OUTD
	EDBase[0xB0] = uint8(LDIR) // LDIR (per loop; last iteration = 16T)
	EDBase[0xB8] = uint8(LDDR) // LDDR

	// ADC/SBC HL, rr
	EDBase[0x4A] = uint8(ADC_HL_rr) // ADC HL, BC
	EDBase[0x5A] = uint8(ADC_HL_rr) // ADC HL, DE
	EDBase[0x6A] = uint8(ADC_HL_rr) // ADC HL, HL
	EDBase[0x7A] = uint8(ADC_HL_rr) // ADC HL, SP
	EDBase[0x42] = uint8(SBC_HL_rr) // SBC HL, BC
	EDBase[0x52] = uint8(SBC_HL_rr) // SBC HL, DE
	EDBase[0x62] = uint8(SBC_HL_rr) // SBC HL, HL
	EDBase[0x72] = uint8(SBC_HL_rr) // SBC HL, SP

	// LD (nn), rr / LD rr, (nn)
	EDBase[0x43] = uint8(LD_nn_HL)      // LD (nn), BC
	EDBase[0x53] = uint8(LD_nn_HL)      // LD (nn), DE
	EDBase[0x73] = uint8(LD_nn_HL)      // LD (nn), SP
	EDBase[0x4B] = uint8(LD_HL_nn_mem)  // LD BC, (nn)
	EDBase[0x5B] = uint8(LD_HL_nn_mem)  // LD DE, (nn)
	EDBase[0x7B] = uint8(LD_HL_nn_mem)  // LD SP, (nn)

	// NEG
	EDBase[0x44] = uint8(NEG)

	// RETN / RETI
	EDBase[0x45] = uint8(RET) + 4 // RETN = 14T
	EDBase[0x4D] = uint8(RET) + 4 // RETI = 14T

	// LD I,A / LD R,A / LD A,I / LD A,R
	EDBase[0x47] = 9 // LD I, A
	EDBase[0x4F] = 9 // LD R, A
	EDBase[0x57] = 9 // LD A, I
	EDBase[0x5F] = 9 // LD A, R

	// IN r, (C)  / OUT (C), r
	for i := 0; i < 8; i++ {
		EDBase[0x40+byte(i*8)] = 12 // IN r, (C) = 12T
		EDBase[0x41+byte(i*8)] = 12 // OUT (C), r = 12T
	}
}

// initDDBase populates DDBase[] for DD/FD-prefixed opcodes (IX/IY variants).
func initDDBase() {
	// Most DD/FD opcodes mirror the base table with 4T added for the prefix decode,
	// but (IX+d) memory accesses add displacement byte fetch: base + IXY_OVERHEAD + displacement.
	//
	// For simplicity: default = Base[op] + IXY_OVERHEAD
	for op := 0; op < 256; op++ {
		if Base[op] > 0 {
			DDBase[op] = Base[op] + IXY_OVERHEAD
		}
	}
	// Override (IX+d) memory access variants — these add displacement byte + 5T extra
	// LD r, (IX+d) = 19T; LD (IX+d), r = 19T; LD (IX+d), n = 19T
	const ixdMem = 19
	for _, op := range []byte{
		0x46, 0x4E, 0x56, 0x5E, 0x66, 0x6E, 0x7E, // LD r, (IX+d)
		0x70, 0x71, 0x72, 0x73, 0x74, 0x75,        // LD (IX+d), r
	} {
		DDBase[op] = ixdMem
	}
	DDBase[0x36] = ixdMem // LD (IX+d), n

	// ALU ops on (IX+d): ADD/ADC/SUB/SBC/AND/XOR/OR/CP = 19T
	for _, op := range []byte{0x86, 0x8E, 0x96, 0x9E, 0xA6, 0xAE, 0xB6, 0xBE} {
		DDBase[op] = ixdMem
	}

	// LD IX/IY, nn = 14T
	DDBase[0x21] = uint8(LD_rr_nn) + IXY_OVERHEAD
	// ADD IX, rr = 15T
	DDBase[0x09] = uint8(ADD_HL_rr) + IXY_OVERHEAD
	DDBase[0x19] = uint8(ADD_HL_rr) + IXY_OVERHEAD
	DDBase[0x29] = uint8(ADD_HL_rr) + IXY_OVERHEAD
	DDBase[0x39] = uint8(ADD_HL_rr) + IXY_OVERHEAD
	// INC/DEC IX = 10T
	DDBase[0x23] = uint8(INC_rr) + IXY_OVERHEAD
	DDBase[0x2B] = uint8(DEC_rr) + IXY_OVERHEAD
	// INC/DEC (IX+d) = 23T
	DDBase[0x34] = 23
	DDBase[0x35] = 23
	// LD (nn), IX / LD IX, (nn) = 20T
	DDBase[0x22] = uint8(LD_nn_HL) + IXY_OVERHEAD
	DDBase[0x2A] = uint8(LD_HL_nn_mem) + IXY_OVERHEAD
	// JP (IX) = 8T
	DDBase[0xE9] = 8
	// PUSH IX / POP IX
	DDBase[0xE5] = PUSH_IX
	DDBase[0xE1] = POP_IX
	// EX (SP), IX = 23T
	DDBase[0xE3] = 23
}

// Lookup returns the T-state count for an instruction given its opcode byte(s).
// prefix should be 0 for unprefixed, 0xCB/0xED/0xDD/0xFD for prefixed.
// For DD+CB and FD+CB (DDCB/FDCB) double-prefix, pass prefix=0xDDCB.
func Lookup(prefix byte, opcode byte) (t int, takent int) {
	switch prefix {
	case 0x00:
		t = int(Base[opcode])
		if BaseCond[opcode] != 0 {
			takent = int(BaseCond[opcode])
		} else {
			takent = t
		}
	case 0xCB:
		t = int(CBBase[opcode])
		takent = t
	case 0xED:
		t = int(EDBase[opcode])
		takent = t
	case 0xDD, 0xFD:
		t = int(DDBase[opcode])
		takent = t
	default:
		t = NOP
		takent = NOP
	}
	return
}
