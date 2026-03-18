// z80.go — Real Z80 machine descriptor for LIR.
//
// This is the production Z80 target, as opposed to CISC (simplified Z80-like).
// It includes all Z80 registers, DD/FD prefix constraints, and realistic costs.
//
// Register file:
//
//	A(8,acc) F(1,flag) B(8) C(8) D(8) E(8) H(8) L(8)
//	BC(16,pair) DE(16,pair) HL(16,pointer) SP(16,stack)
//	IX(16,index) IY(16,index)
//	IXH(8) IXL(8) IYH(8) IYL(8) — undocumented half-index regs
package lir

// Z80 is the production Z80 machine descriptor.
var Z80 = &MachineDesc{
	Name:     "z80",
	WordSize: 8,
	Locs: []Loc{
		// 8-bit registers
		{Name: "A", Width: 8, Kind: LocAcc},   // 0
		{Name: "B", Width: 8, Kind: LocReg},    // 1
		{Name: "C", Width: 8, Kind: LocReg},    // 2
		{Name: "D", Width: 8, Kind: LocReg},    // 3
		{Name: "E", Width: 8, Kind: LocReg},    // 4
		{Name: "H", Width: 8, Kind: LocReg},    // 5
		{Name: "L", Width: 8, Kind: LocReg},    // 6
		// 16-bit register pairs
		{Name: "BC", Width: 16, Kind: LocPair},  // 7
		{Name: "DE", Width: 16, Kind: LocPair},  // 8
		{Name: "HL", Width: 16, Kind: LocIndex}, // 9
		{Name: "SP", Width: 16, Kind: LocIndex}, // 10
		// Index registers (DD/FD prefixed)
		{Name: "IX", Width: 16, Kind: LocIndex}, // 11
		{Name: "IY", Width: 16, Kind: LocIndex}, // 12
		// Flags
		{Name: "F", Width: 1, Kind: LocFlag},    // 13
		// Index register halves (undocumented, DD/FD prefix, survive calls)
		{Name: "IXH", Width: 8, Kind: LocIndex}, // 14
		{Name: "IXL", Width: 8, Kind: LocIndex}, // 15
		{Name: "IYH", Width: 8, Kind: LocIndex}, // 16
		{Name: "IYL", Width: 8, Kind: LocIndex}, // 17
		// Spill slots (memory-backed)
		{Name: "spill0", Width: 16, Kind: LocMem}, // 18
		{Name: "spill1", Width: 16, Kind: LocMem}, // 19
		{Name: "spill2", Width: 16, Kind: LocMem}, // 20
		{Name: "spill3", Width: 16, Kind: LocMem}, // 21
	},
}

func init() {
	Z80.Patterns = generateZ80Patterns(Z80)
	Z80.Rules = generateZ80Rules(Z80)
}

func generateZ80Patterns(m *MachineDesc) []Pattern {
	a := m.LocSetByNames("A")
	gpr8 := m.LocSetByNames("A", "B", "C", "D", "E", "H", "L")
	b := m.LocSetByNames("B")
	hl := m.LocSetByNames("HL")
	de := m.LocSetByNames("DE")
	bc := m.LocSetByNames("BC")
	pairs := m.LocSetByNames("BC", "DE", "HL")
	ix := m.LocSetByNames("IX")
	iy := m.LocSetByNames("IY")
	flags := m.LocSetByNames("F")
	// Index register halves — survive calls, DD/FD prefix
	ixh := m.LocSetByNames("IXH")
	ixl := m.LocSetByNames("IXL")
	iyh := m.LocSetByNames("IYH")
	iyl := m.LocSetByNames("IYL")
	ixHalves := ixh.Or(ixl).Or(iyh).Or(iyl)
	// GPR without H/L (compatible with DD/FD prefix operations)
	gprNoHL := m.LocSetByNames("A", "B", "C", "D", "E")
	spill := m.LocSetByNames("spill0", "spill1", "spill2", "spill3")

	return []Pattern{
		// ── Constants ─────────────────────────────────────────────────
		{Name: "ld_r_n", MIROp: OpConst, Width: 8, DstLocs: gpr8,
			Template: "LD {dst}, {imm}", Cost: 7, Bytes: 2, Flags: PatImmediate},
		{Name: "ld_rr_nn", MIROp: OpConst, Width: 16, DstLocs: pairs.Or(ix).Or(iy),
			Template: "LD {dst}, {imm}", Cost: 10, Bytes: 3, Flags: PatImmediate},

		// ── 8-bit moves ──────────────────────────────────────────────
		{Name: "ld_r_r", MIROp: OpMove, Width: 8, DstLocs: gpr8, SrcLocs: [2]LocSet{gpr8},
			Template: "LD {dst}, {src0}", Cost: 4, Bytes: 1},

		// ── 8-bit IX/IY half moves (DD/FD prefixed, call-safe) ──────
		// LD IXH, r — save GPR to IX half (r cannot be H or L: DD prefix conflict)
		{Name: "ld_ixh_r", MIROp: OpMove, Width: 8, DstLocs: ixHalves, SrcLocs: [2]LocSet{gprNoHL},
			Template: "LD {dst}, {src0}", Cost: 8, Bytes: 2},
		// LD r, IXH — restore GPR from IX half (r cannot be H or L)
		{Name: "ld_r_ixh", MIROp: OpMove, Width: 8, DstLocs: gprNoHL, SrcLocs: [2]LocSet{ixHalves},
			Template: "LD {dst}, {src0}", Cost: 8, Bytes: 2},

		// ── 8-bit → 16-bit widening (zero-extend) ───────────────────
		// LD L, r; LD H, 0 — zero-extend 8-bit to HL
		{Name: "zext_hl_a", MIROp: OpMove, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{a},
			Template: "LD L, A\n    LD H, 0", Cost: 11, Bytes: 3},
		{Name: "zext_hl_r", MIROp: OpMove, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{gprNoHL},
			Template: "LD L, {src0}\n    LD H, 0", Cost: 11, Bytes: 3},
		// LD C, r; LD B, 0 — zero-extend to BC
		{Name: "zext_bc_r", MIROp: OpMove, Width: 16, DstLocs: bc, SrcLocs: [2]LocSet{gpr8},
			Template: "LD C, {src0}\n    LD B, 0", Cost: 11, Bytes: 3},
		// LD E, r; LD D, 0 — zero-extend to DE
		{Name: "zext_de_r", MIROp: OpMove, Width: 16, DstLocs: de, SrcLocs: [2]LocSet{gpr8},
			Template: "LD E, {src0}\n    LD D, 0", Cost: 11, Bytes: 3},

		// ── 16-bit moves ─────────────────────────────────────────────
		{Name: "push_pop", MIROp: OpMove, Width: 16, DstLocs: pairs, SrcLocs: [2]LocSet{pairs},
			Template: "PUSH {src0}\n    POP {dst}", Cost: 21, Bytes: 2},
		{Name: "ex_de_hl", MIROp: OpMove, Width: 16, DstLocs: de, SrcLocs: [2]LocSet{hl},
			Template: "EX DE, HL", Cost: 4, Bytes: 1},

		// ── 8-bit ALU (A = accumulator destination) ──────────────────
		{Name: "add_a_r", MIROp: OpAdd, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, gpr8},
			Template: "ADD A, {src1}", Cost: 4, Bytes: 1, Clobbers: flags, Flags: PatCommutative},
		{Name: "sub_a_r", MIROp: OpSub, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, gpr8},
			Template: "SUB {src1}", Cost: 4, Bytes: 1, Clobbers: flags},
		{Name: "and_a_r", MIROp: OpAnd, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, gpr8},
			Template: "AND {src1}", Cost: 4, Bytes: 1, Clobbers: flags, Flags: PatCommutative},
		{Name: "or_a_r", MIROp: OpOr, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, gpr8},
			Template: "OR {src1}", Cost: 4, Bytes: 1, Clobbers: flags, Flags: PatCommutative},
		{Name: "xor_a_r", MIROp: OpXor, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, gpr8},
			Template: "XOR {src1}", Cost: 4, Bytes: 1, Clobbers: flags, Flags: PatCommutative},
		{Name: "cp_r", MIROp: OpCmp, Width: 8, DstLocs: flags, SrcLocs: [2]LocSet{a, gpr8},
			Template: "CP {src1}", Cost: 4, Bytes: 1, Clobbers: flags},

		// ── 8-bit INC/DEC (any GPR, no carry) ────────────────────────
		{Name: "inc_r", MIROp: OpAdd, Width: 8, DstLocs: gpr8, SrcLocs: [2]LocSet{gpr8, gpr8},
			Template: "INC {dst}", Cost: 4, Bytes: 1},
		{Name: "dec_r", MIROp: OpSub, Width: 8, DstLocs: gpr8, SrcLocs: [2]LocSet{gpr8, gpr8},
			Template: "DEC {dst}", Cost: 4, Bytes: 1},

		// ── 16-bit ALU (HL destination) ──────────────────────────────
		{Name: "add_hl_rr", MIROp: OpAdd, Width: 16, DstLocs: hl,
			SrcLocs: [2]LocSet{hl, bc.Or(de).Or(hl)},
			Template: "ADD HL, {src1}", Cost: 11, Bytes: 1, Clobbers: flags},
		{Name: "sbc_hl_rr", MIROp: OpSub, Width: 16, DstLocs: hl,
			SrcLocs: [2]LocSet{hl, bc.Or(de).Or(hl)},
			Template: "OR A\n    SBC HL, {src1}", Cost: 19, Bytes: 3, Clobbers: flags},
		{Name: "inc_rr", MIROp: OpAdd, Width: 16, DstLocs: pairs,
			SrcLocs: [2]LocSet{pairs, pairs},
			Template: "INC {dst}", Cost: 6, Bytes: 1},

		// ── Shifts — 8-bit ───────────────────────────────────────────
		{Name: "sla_a", MIROp: OpShl, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, gpr8},
			Template: "SLA A", Cost: 8, Bytes: 2, Clobbers: flags},
		{Name: "srl_a", MIROp: OpShr, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, gpr8},
			Template: "SRL A", Cost: 8, Bytes: 2, Clobbers: flags},

		// ── Shifts — 16-bit (composite) ──────────────────────────────
		// SHL HL,1 = ADD HL,HL (most common, from x*2)
		{Name: "shl16_hl", MIROp: OpShl, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{hl, gpr8},
			Template: "ADD HL, HL", Cost: 11, Bytes: 1, Clobbers: flags},
		// SHR HL,1 = SRL H; RR L
		{Name: "shr16_hl", MIROp: OpShr, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{hl, gpr8},
			Template: "SRL H\n    RR L", Cost: 16, Bytes: 4, Clobbers: flags},

		// ── Bitwise — 16-bit (composite, via H/L halves) ─────────────
		{Name: "and16_hl_de", MIROp: OpAnd, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{hl, de},
			Template: "LD A, H\n    AND D\n    LD H, A\n    LD A, L\n    AND E\n    LD L, A",
			Cost: 28, Bytes: 6, Clobbers: flags.Or(a)},
		{Name: "or16_hl_de", MIROp: OpOr, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{hl, de},
			Template: "LD A, H\n    OR D\n    LD H, A\n    LD A, L\n    OR E\n    LD L, A",
			Cost: 28, Bytes: 6, Clobbers: flags.Or(a)},
		{Name: "xor16_hl_de", MIROp: OpXor, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{hl, de},
			Template: "LD A, H\n    XOR D\n    LD H, A\n    LD A, L\n    XOR E\n    LD L, A",
			Cost: 28, Bytes: 6, Clobbers: flags.Or(a)},

		// ── Memory loads ─────────────────────────────────────────────
		{Name: "ld_a_hl", MIROp: OpLoad, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{hl},
			Template: "LD A, (HL)", Cost: 7, Bytes: 1, Flags: PatMemRead},
		{Name: "ld_r_hl", MIROp: OpLoad, Width: 8, DstLocs: gpr8, SrcLocs: [2]LocSet{hl},
			Template: "LD {dst}, (HL)", Cost: 9, Bytes: 1, Flags: PatMemRead},
		{Name: "ld_a_de", MIROp: OpLoad, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{de},
			Template: "LD A, (DE)", Cost: 7, Bytes: 1, Flags: PatMemRead},
		{Name: "ld_a_bc", MIROp: OpLoad, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{bc},
			Template: "LD A, (BC)", Cost: 7, Bytes: 1, Flags: PatMemRead},
		// 16-bit load via HL pointer
		{Name: "ld16_hl_ind", MIROp: OpLoad, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{hl},
			Template: "LD A, (HL)\n    INC HL\n    LD H, (HL)\n    LD L, A",
			Cost: 22, Bytes: 4, Flags: PatMemRead},

		// ── Memory stores ────────────────────────────────────────────
		{Name: "ld_hl_a", MIROp: OpStore, Width: 8, SrcLocs: [2]LocSet{hl, a},
			Template: "LD (HL), A", Cost: 7, Bytes: 1, Flags: PatMemWrite},
		{Name: "ld_hl_r", MIROp: OpStore, Width: 8, SrcLocs: [2]LocSet{hl, gpr8},
			Template: "LD (HL), {src1}", Cost: 7, Bytes: 1, Flags: PatMemWrite},

		// ── Memory stores — 16-bit ───────────────────────────────────
		{Name: "ld_nn_hl_store", MIROp: OpStore, Width: 16, SrcLocs: [2]LocSet{spill, hl},
			Template: "LD ({src0}), HL", Cost: 16, Bytes: 3, Flags: PatMemWrite},

		// ── Combined 16-bit LE load (ISLE combining target) ──────────
		{Name: "ld16_le_hl", MIROp: OpLoad16LE, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{hl},
			Template: "LD A, (HL)\n    INC HL\n    LD H, (HL)\n    LD L, A",
			Cost: 22, Bytes: 4, Flags: PatMemRead},

		// ── IX-indexed loads (DD prefix) ─────────────────────────────
		{Name: "ld_r_ix_d", MIROp: OpLoad, Width: 8, DstLocs: gpr8, SrcLocs: [2]LocSet{ix},
			Template: "LD {dst}, (IX+{imm})", Cost: 19, Bytes: 3, Flags: PatMemRead | PatImmediate},

		// ── Spill (16-bit to absolute address) ───────────────────────
		{Name: "ld_nn_hl", MIROp: OpMove, Width: 16, DstLocs: spill, SrcLocs: [2]LocSet{hl},
			Template: "LD ({dst}), HL", Cost: 16, Bytes: 3},
		{Name: "ld_hl_nn", MIROp: OpMove, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{spill},
			Template: "LD HL, ({src0})", Cost: 16, Bytes: 3},

		// ── Spill (8-bit via A to absolute address) ──────────────────
		// LD (nn), A — store accumulator to memory spill slot
		{Name: "ld_nn_a", MIROp: OpMove, Width: 8, DstLocs: spill, SrcLocs: [2]LocSet{a},
			Template: "LD ({dst}), A", Cost: 13, Bytes: 3},
		// LD A, (nn) — load accumulator from memory spill slot
		{Name: "ld_a_nn", MIROp: OpMove, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{spill},
			Template: "LD A, ({src0})", Cost: 13, Bytes: 3},

		// ── Function calls ──────────────────────────────────────────
		// CALL sym — return value in A (u8) or HL (u16).
		// Clobbers all volatile registers (caller-save).
		{Name: "call_u8", MIROp: OpCall, Width: 8, DstLocs: a,
			Template: "CALL {sym}", Cost: 17, Bytes: 3,
			Clobbers: gpr8.Or(flags), Flags: PatCall},
		{Name: "call_u16", MIROp: OpCall, Width: 16, DstLocs: hl,
			Template: "CALL {sym}", Cost: 17, Bytes: 3,
			Clobbers: gpr8.Or(flags).Or(de).Or(bc), Flags: PatCall},
		{Name: "call_void", MIROp: OpCall, Width: 0,
			Template: "CALL {sym}", Cost: 17, Bytes: 3,
			Clobbers: gpr8.Or(flags), Flags: PatCall},

		// ── DJNZ (loop) — not used by isel but by LoopRotateDJNZ ────
		// DJNZ is a terminator pattern, not a regular instruction.
		// Counter must be in B. The pattern is used for cost estimation.
		{Name: "djnz", MIROp: OpSub, Width: 8, DstLocs: b, SrcLocs: [2]LocSet{b, gpr8},
			Template: "DJNZ {target}", Cost: 13, Bytes: 2},
	}
}

func generateZ80Rules(m *MachineDesc) []ConstraintRule {
	// DD/FD prefix conflict: IX/IY indexed addressing cannot use H,L as source/dest
	// because the opcode bytes overlap.
	h := m.LocSetByNames("H")
	l := m.LocSetByNames("L")
	hl := h.Or(l)
	ix := m.LocSetByNames("IX")
	iy := m.LocSetByNames("IY")

	return []ConstraintRule{
		{Name: "dd_prefix_no_h", DstSet: hl, SrcSet: ix, Cost: MaxCost},
		{Name: "dd_prefix_no_l", DstSet: hl, SrcSet: ix, Cost: MaxCost},
		{Name: "fd_prefix_no_h", DstSet: hl, SrcSet: iy, Cost: MaxCost},
		{Name: "fd_prefix_no_l", DstSet: hl, SrcSet: iy, Cost: MaxCost},
	}
}
