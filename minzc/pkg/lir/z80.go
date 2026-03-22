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
		// L3: Shadow registers (EXX swaps BC↔BC', DE↔DE', HL↔HL')
		// EXX = 4T, toggles all 3 pairs. Call-safe: ISR saves/restores.
		{Name: "B'", Width: 8, Kind: LocShadow},  // 18
		{Name: "C'", Width: 8, Kind: LocShadow},  // 19
		{Name: "D'", Width: 8, Kind: LocShadow},  // 20
		{Name: "E'", Width: 8, Kind: LocShadow},  // 21
		{Name: "H'", Width: 8, Kind: LocShadow},  // 22
		{Name: "L'", Width: 8, Kind: LocShadow},  // 23
		// Shadow A via EX AF,AF' (4T, independent of EXX)
		{Name: "A'", Width: 8, Kind: LocShadowAcc}, // 24
		// L4: TSMC spill slots (self-modifying code: LD r, imm8 patched at runtime)
		// Cost: 13T store (LD (label+1), A) + 7T reload (LD A, imm8 patched)
		{Name: "tsmc0", Width: 8, Kind: LocTSMC},   // 25
		{Name: "tsmc1", Width: 8, Kind: LocTSMC},   // 26
		{Name: "tsmc2", Width: 8, Kind: LocTSMC},   // 27
		{Name: "tsmc3", Width: 8, Kind: LocTSMC},   // 28
		{Name: "tsmc4", Width: 8, Kind: LocTSMC},   // 29
		{Name: "tsmc5", Width: 8, Kind: LocTSMC},   // 30
		{Name: "tsmc6", Width: 8, Kind: LocTSMC},   // 31
		{Name: "tsmc7", Width: 8, Kind: LocTSMC},   // 32
		// L5: Memory spill slots (absolute address LD A,(nn) / LD (nn),A)
		// Cost: 26T round-trip (13T load + 13T store)
		{Name: "mem0", Width: 16, Kind: LocMem},     // 33
		{Name: "mem1", Width: 16, Kind: LocMem},     // 34
		{Name: "mem2", Width: 16, Kind: LocMem},     // 35
		{Name: "mem3", Width: 16, Kind: LocMem},     // 36
	},
}

func init() {
	// Per-location cost in T-states. WFC uses this to prefer cheaper locs.
	Z80.LocCost = []int{
		0, 0, 0, 0, 0, 0, 0, // L1: A,B,C,D,E,H,L — free
		0, 0, 0, 0,           // BC,DE,HL,SP — free (16-bit)
		4, 4,                 // IX,IY — DD/FD prefix
		0,                    // F — flags (free for booleans)
		4, 4, 4, 4,          // IXH,IXL,IYH,IYL — DD/FD prefix
		8, 8, 8, 8, 8, 8,    // L3: B',C',D',E',H',L' — EXX bracket (4T in + 4T out)
		8,                    // A' — EX AF,AF' bracket
		20, 20, 20, 20, 20, 20, 20, 20, // L4: tsmc0-7 — 13T store + 7T reload
		26, 26, 26, 26,       // L5: mem0-3 — 13T LD A,(nn) + 13T LD (nn),A
	}
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
	// L3: Shadow registers (EXX batch swap, 4T per region entry/exit)
	shadow8 := m.LocSetByNames("B'", "C'", "D'", "E'", "H'", "L'")
	shadowA := m.LocSetByNames("A'")
	// L4: TSMC spill slots (self-modifying code, 13T store / 7T reload)
	tsmc := m.LocSetByNames("tsmc0", "tsmc1", "tsmc2", "tsmc3",
		"tsmc4", "tsmc5", "tsmc6", "tsmc7")
	// L5: Memory spill slots (absolute address, 26T round-trip)
	spill := m.LocSetByNames("mem0", "mem1", "mem2", "mem3")
	// All 8-bit storage: GPR + IXY halves + shadow + shadow A + TSMC
	all8 := gpr8.Or(ixHalves).Or(shadow8).Or(shadowA).Or(tsmc)
	_ = all8 // available for future patterns

	return []Pattern{
		// ── Constants ─────────────────────────────────────────────────
		{Name: "ld_r_n", MIROp: OpConst, Width: 8, DstLocs: gpr8,
			Template: "LD {dst}, {imm}", Cost: 7, Bytes: 2, Flags: PatImmediate},
		{Name: "ld_rr_nn", MIROp: OpConst, Width: 16, DstLocs: pairs.Or(ix).Or(iy),
			Template: "LD {dst}, {imm}", Cost: 10, Bytes: 3, Flags: PatImmediate},

		// ── 8-bit moves ──────────────────────────────────────────────
		{Name: "ld_r_r", MIROp: OpMove, Width: 8, DstLocs: gpr8, SrcLocs: [2]LocSet{gpr8},
			Template: "LD {dst}, {src0}", Cost: 4, Bytes: 1},

		// ── Truncation: pair → low byte (zero-cost alias) ───────────
		// The low byte of HL is L, of DE is E, of BC is C.
		// These are "free" — no instruction needed, just register aliasing.
		// WFC constrains dst to the low-byte register of the source pair.
		{Name: "trunc_hl_l", MIROp: OpMove, Width: 8, DstLocs: m.LocSetByNames("L"), SrcLocs: [2]LocSet{hl},
			Template: "; trunc HL→L (alias)", Cost: 0, Bytes: 0},
		{Name: "trunc_de_e", MIROp: OpMove, Width: 8, DstLocs: m.LocSetByNames("E"), SrcLocs: [2]LocSet{de},
			Template: "; trunc DE→E (alias)", Cost: 0, Bytes: 0},
		{Name: "trunc_bc_c", MIROp: OpMove, Width: 8, DstLocs: m.LocSetByNames("C"), SrcLocs: [2]LocSet{bc},
			Template: "; trunc BC→C (alias)", Cost: 0, Bytes: 0},
		// When dst must be A (e.g. for ALU), route: LD A, L (low byte of HL)
		{Name: "trunc_hl_a", MIROp: OpMove, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{hl},
			Template: "LD A, L", Cost: 4, Bytes: 1},
		{Name: "trunc_de_a", MIROp: OpMove, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{de},
			Template: "LD A, E", Cost: 4, Bytes: 1},
		{Name: "trunc_bc_a", MIROp: OpMove, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{bc},
			Template: "LD A, C", Cost: 4, Bytes: 1},

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
		// LD (nn), HL — store 16-bit pair to absolute address
		{Name: "ld_nn_hl_store", MIROp: OpStore, Width: 16, SrcLocs: [2]LocSet{spill, hl},
			Template: "LD ({src0}), HL", Cost: 16, Bytes: 3, Flags: PatMemWrite},
		// 16-bit store via HL pointer: value in DE or BC (no self-conflict)
		{Name: "st16_hl_de", MIROp: OpStore, Width: 16, SrcLocs: [2]LocSet{hl, de},
			Template: "LD (HL), E\n    INC HL\n    LD (HL), D\n    DEC HL",
			Cost: 22, Bytes: 4, Flags: PatMemWrite},
		{Name: "st16_hl_bc", MIROp: OpStore, Width: 16, SrcLocs: [2]LocSet{hl, bc},
			Template: "LD (HL), C\n    INC HL\n    LD (HL), B\n    DEC HL",
			Cost: 22, Bytes: 4, Flags: PatMemWrite},
		// 16-bit self-store: ptr=HL, val=HL (same register).
		// CHEAP variant (22T): LD (HL),L / INC HL / LD (HL),H / DEC HL
		//   ⚠ EDGE CASE: if L=0xFF, INC HL carries into H, so LD (HL+1),H
		//   writes the incremented H. Safe when L≠0xFF (vast majority of ptrs).
		{Name: "st16_hl_hl_fast", MIROp: OpStore, Width: 16, SrcLocs: [2]LocSet{hl, hl},
			Template: "LD (HL), L\n    INC HL\n    LD (HL), H\n    DEC HL",
			Cost: 22, Bytes: 4, Flags: PatMemWrite},
		// SAFE variant (30T): evacuate to DE first, always correct.
		{Name: "st16_hl_hl_safe", MIROp: OpStore, Width: 16, SrcLocs: [2]LocSet{hl, hl},
			Template: "LD D, H\n    LD E, L\n    LD (HL), E\n    INC HL\n    LD (HL), D\n    DEC HL",
			Cost: 30, Bytes: 6, Flags: PatMemWrite},

		// ── Global store/load (LD (nn), HL / LD HL, (nn)) ──────────
		// Direct 16-bit store to absolute address (global variable).
		// Z80: LD (nn), HL = 16T, 3 bytes. Much better than loading addr into
		// register + byte-by-byte store (10T + 22T = 32T).
		{Name: "st_global_hl", MIROp: OpStoreGlobal, Width: 16, SrcLocs: [2]LocSet{hl, hl},
			Template: "LD ({imm}), HL", Cost: 16, Bytes: 3, Flags: PatMemWrite | PatImmediate},
		// LD (nn), A — 8-bit store to absolute address (global u8 variable).
		{Name: "st_global_a", MIROp: OpStoreGlobal, Width: 8, SrcLocs: [2]LocSet{a, a},
			Template: "LD ({imm}), A", Cost: 13, Bytes: 3, Flags: PatMemWrite | PatImmediate},
		// LD HL, (nn) — load 16-bit from absolute address.
		{Name: "ld_global_hl", MIROp: OpLoadGlobal, Width: 16, DstLocs: hl,
			Template: "LD HL, ({imm})", Cost: 16, Bytes: 3, Flags: PatMemRead | PatImmediate},
		// LD A, (nn) — load 8-bit from absolute address.
		{Name: "ld_global_a", MIROp: OpLoadGlobal, Width: 8, DstLocs: a,
			Template: "LD A, ({imm})", Cost: 13, Bytes: 3, Flags: PatMemRead | PatImmediate},

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

		// ── IX-indexed stores (DD prefix) ────────────────────────────
		{Name: "ld_ix_d_r", MIROp: OpStore, Width: 8, SrcLocs: [2]LocSet{ix, gprNoHL},
			Template: "LD (IX+{imm}), {src1}", Cost: 19, Bytes: 3, Flags: PatMemWrite | PatImmediate},

		// ── IY-indexed loads/stores (FD prefix) ──────────────────────
		{Name: "ld_r_iy_d", MIROp: OpLoad, Width: 8, DstLocs: gprNoHL, SrcLocs: [2]LocSet{iy},
			Template: "LD {dst}, (IY+{imm})", Cost: 19, Bytes: 3, Flags: PatMemRead | PatImmediate},
		{Name: "ld_iy_d_r", MIROp: OpStore, Width: 8, SrcLocs: [2]LocSet{iy, gprNoHL},
			Template: "LD (IY+{imm}), {src1}", Cost: 19, Bytes: 3, Flags: PatMemWrite | PatImmediate},

		// ── Store via DE/BC pointer ──────────────────────────────────
		{Name: "ld_de_a", MIROp: OpStore, Width: 8, SrcLocs: [2]LocSet{de, a},
			Template: "LD (DE), A", Cost: 7, Bytes: 1, Flags: PatMemWrite},
		{Name: "ld_bc_a", MIROp: OpStore, Width: 8, SrcLocs: [2]LocSet{bc, a},
			Template: "LD (BC), A", Cost: 7, Bytes: 1, Flags: PatMemWrite},

		// ── 16-bit DEC ───────────────────────────────────────────────
		{Name: "dec_rr", MIROp: OpSub, Width: 16, DstLocs: pairs,
			SrcLocs: [2]LocSet{pairs, pairs},
			Template: "DEC {dst}", Cost: 6, Bytes: 1},

		// ── EX DE,HL reverse (HL ← DE) ──────────────────────────────
		{Name: "ex_hl_de", MIROp: OpMove, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{de},
			Template: "EX DE, HL", Cost: 4, Bytes: 1},

		// ── Flag → register materialization ──────────────────────────
		// SBC A,A: A = 0xFF if carry set (true), 0x00 if clear (false). 4T.
		{Name: "flag_to_a", MIROp: OpMove, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{flags},
			Template: "SBC A, A", Cost: 4, Bytes: 1, Clobbers: flags},

		// ── NEG A (two's complement negate) ──────────────────────────
		{Name: "neg_a", MIROp: OpNeg, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a},
			Template: "NEG", Cost: 8, Bytes: 2, Clobbers: flags},

		// ── Immediate ALU (fold const into instruction) ──────────────
		// These match OpAddImm etc. produced by ISLE const-folding rules.
		{Name: "add_a_n", MIROp: OpAddImm, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a},
			Template: "ADD A, {imm}", Cost: 7, Bytes: 2, Clobbers: flags, Flags: PatImmediate},
		{Name: "sub_a_n", MIROp: OpSubImm, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a},
			Template: "SUB {imm}", Cost: 7, Bytes: 2, Clobbers: flags, Flags: PatImmediate},
		{Name: "and_a_n", MIROp: OpAndImm, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a},
			Template: "AND {imm}", Cost: 7, Bytes: 2, Clobbers: flags, Flags: PatImmediate},
		{Name: "or_a_n", MIROp: OpOrImm, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a},
			Template: "OR {imm}", Cost: 7, Bytes: 2, Clobbers: flags, Flags: PatImmediate},
		{Name: "xor_a_n", MIROp: OpXorImm, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a},
			Template: "XOR {imm}", Cost: 7, Bytes: 2, Clobbers: flags, Flags: PatImmediate},
		{Name: "cp_n", MIROp: OpCmpImm, Width: 8, DstLocs: flags, SrcLocs: [2]LocSet{a},
			Template: "CP {imm}", Cost: 7, Bytes: 2, Clobbers: flags, Flags: PatImmediate},

		// ── DJNZ (loop) — not used by isel but by LoopRotateDJNZ ────
		// DJNZ is a terminator pattern, not a regular instruction.
		// Counter must be in B. The pattern is used for cost estimation.
		{Name: "djnz", MIROp: OpSub, Width: 8, DstLocs: b, SrcLocs: [2]LocSet{b, gpr8},
			Template: "DJNZ {target}", Cost: 13, Bytes: 2},

		// ══════════════════════════════════════════════════════════════
		// L3: Shadow register moves via EXX / EX AF,AF'
		// ══════════════════════════════════════════════════════════════
		//
		// EXX swaps BC↔BC', DE↔DE', HL↔HL' simultaneously (4T, 1 byte).
		// To use shadow as spill: EXX, LD r',val, EXX (save), or
		// EXX, LD r,r', EXX (restore). Cost = 4T + 4T + 4T = 12T.
		//
		// GPR → shadow: save to shadow register via EXX bracket.
		// Template uses {dst} which resolves to "B'" etc. The emitter's
		// shadowMainName() strips the prime: B' → B (accessed as B inside EXX).
		{Name: "save_shadow", MIROp: OpMove, Width: 8, DstLocs: shadow8, SrcLocs: [2]LocSet{gpr8},
			Template: "EXX\n    LD {dst}, {src0}\n    EXX", Cost: 12, Bytes: 3},
		// shadow → GPR: restore from shadow register via EXX bracket.
		{Name: "load_shadow", MIROp: OpMove, Width: 8, DstLocs: gpr8, SrcLocs: [2]LocSet{shadow8},
			Template: "EXX\n    LD {dst}, {src0}\n    EXX", Cost: 12, Bytes: 3},

		// A ↔ A' via EX AF,AF' (4T each way, independent of EXX)
		{Name: "save_shadow_a", MIROp: OpMove, Width: 8, DstLocs: shadowA, SrcLocs: [2]LocSet{a},
			Template: "EX AF, AF'", Cost: 4, Bytes: 1},
		{Name: "load_shadow_a", MIROp: OpMove, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{shadowA},
			Template: "EX AF, AF'", Cost: 4, Bytes: 1},

		// ══════════════════════════════════════════════════════════════
		// L4: TSMC spill — Self-Modifying Code
		// ══════════════════════════════════════════════════════════════
		//
		// Store: patch the immediate byte of a LD A,imm8 instruction.
		//   LD A, val
		//   LD ({tsmc_label}+1), A    ; patch imm8 of reload instruction
		// Reload: execute the patched LD A,imm8.
		//   {tsmc_label}: LD A, 0     ; imm8 was patched to actual value
		//
		// Cost: store=13T (LD A,r 4T + LD (nn),A 13T but often A already has val)
		//        reload=7T (LD A,imm8 — the imm8 IS the stored value)
		//
		// GPR → TSMC: store (patch the reload instruction's immediate)
		{Name: "tsmc_store", MIROp: OpMove, Width: 8, DstLocs: tsmc, SrcLocs: [2]LocSet{a},
			Template: "LD ({dst}+1), A", Cost: 13, Bytes: 3},
		// TSMC → A: reload (execute the patched LD A,imm8)
		{Name: "tsmc_reload", MIROp: OpMove, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{tsmc},
			Template: "{src0}: LD A, 0", Cost: 7, Bytes: 2},

		// ══════════════════════════════════════════════════════════════
		// L5: Memory spill — absolute address LD A,(nn) / LD (nn),A
		// ══════════════════════════════════════════════════════════════
		//
		// Only A can do direct memory access on Z80.
		// For non-A GPR → memory: route through A (extra 4T each way).
		//
		// GPR → mem: store via A
		{Name: "spill_r_mem", MIROp: OpMove, Width: 8, DstLocs: spill, SrcLocs: [2]LocSet{gpr8},
			Template: "LD A, {src0}\n    LD ({dst}), A", Cost: 17, Bytes: 4},
		// A → mem: direct store
		{Name: "spill_a_mem", MIROp: OpMove, Width: 8, DstLocs: spill, SrcLocs: [2]LocSet{a},
			Template: "LD ({dst}), A", Cost: 13, Bytes: 3},
		// mem → A: direct load
		{Name: "unspill_mem_a", MIROp: OpMove, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{spill},
			Template: "LD A, ({src0})", Cost: 13, Bytes: 3},
		// mem → GPR: load via A
		{Name: "unspill_mem_r", MIROp: OpMove, Width: 8, DstLocs: gpr8, SrcLocs: [2]LocSet{spill},
			Template: "LD A, ({src0})\n    LD {dst}, A", Cost: 17, Bytes: 4},

		// Constants directly to spill tiers (avoid GPR round-trip)
		{Name: "const_shadow_a", MIROp: OpConst, Width: 8, DstLocs: shadowA,
			Template: "EX AF, AF'\n    LD A, {imm}\n    EX AF, AF'", Cost: 15, Bytes: 4, Flags: PatImmediate},
		{Name: "const_tsmc", MIROp: OpConst, Width: 8, DstLocs: tsmc,
			Template: "LD A, {imm}\n    LD ({dst}+1), A", Cost: 20, Bytes: 5, Flags: PatImmediate},
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
