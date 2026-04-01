package vir

// Z80 is the production Z80 machine descriptor with all spill tiers.
//
// Spill tier hierarchy (cheapest first):
//   L1: GPR          {A,B,C,D,E,H,L,BC,DE,HL,SP,IX,IY,F}  0-4T
//   L2: IX halves    {IXH,IXL,IYH,IYL}                      8T  (DD/FD prefix, call-safe)
//   L3: Shadow       {B',C',D',E',H',L',A'}                 8T  (EXX/EX AF,AF')
//   L4: TSMC         {tsmc0..tsmc7}                          20T (self-modifying code)
//   L5: Memory       {mem0..mem3}                             26T (absolute address)
//   L6: Stack        {stk0..stk3}                             22T (PUSH/POP)
var Z80 = &MachineDesc{
	Name:     "z80",
	WordSize: 8,
	Locs: []Loc{
		// L1: 8-bit GPR (index 0-6)
		{Name: "A", Width: 8, Kind: LocAcc},  // 0
		{Name: "B", Width: 8, Kind: LocGPR},  // 1
		{Name: "C", Width: 8, Kind: LocGPR},  // 2
		{Name: "D", Width: 8, Kind: LocGPR},  // 3
		{Name: "E", Width: 8, Kind: LocGPR},  // 4
		{Name: "H", Width: 8, Kind: LocGPR},  // 5
		{Name: "L", Width: 8, Kind: LocGPR},  // 6

		// L1: 16-bit pairs (index 7-12)
		{Name: "BC", Width: 16, Kind: LocPair},  // 7
		{Name: "DE", Width: 16, Kind: LocPair},  // 8
		{Name: "HL", Width: 16, Kind: LocIndex}, // 9
		{Name: "SP", Width: 16, Kind: LocIndex}, // 10
		{Name: "IX", Width: 16, Kind: LocIndex}, // 11
		{Name: "IY", Width: 16, Kind: LocIndex}, // 12

		// L1: Flags (index 13)
		{Name: "F", Width: 1, Kind: LocFlag}, // 13

		// L2: IX/IY halves (index 14-17) — undocumented, call-safe
		{Name: "IXH", Width: 8, Kind: LocIXHalf}, // 14
		{Name: "IXL", Width: 8, Kind: LocIXHalf}, // 15
		{Name: "IYH", Width: 8, Kind: LocIXHalf}, // 16
		{Name: "IYL", Width: 8, Kind: LocIXHalf}, // 17

		// L3: Shadow registers (index 18-24) — EXX / EX AF,AF'
		{Name: "B'", Width: 8, Kind: LocShadow},    // 18
		{Name: "C'", Width: 8, Kind: LocShadow},    // 19
		{Name: "D'", Width: 8, Kind: LocShadow},    // 20
		{Name: "E'", Width: 8, Kind: LocShadow},    // 21
		{Name: "H'", Width: 8, Kind: LocShadow},    // 22
		{Name: "L'", Width: 8, Kind: LocShadow},    // 23
		{Name: "A'", Width: 8, Kind: LocShadowAcc}, // 24

		// L4: TSMC spill slots (index 25-32)
		// Cost: 13T store (LD (label+1), A) + 7T reload (LD A, patched_imm)
		{Name: "tsmc0", Width: 8, Kind: LocTSMC}, // 25
		{Name: "tsmc1", Width: 8, Kind: LocTSMC}, // 26
		{Name: "tsmc2", Width: 8, Kind: LocTSMC}, // 27
		{Name: "tsmc3", Width: 8, Kind: LocTSMC}, // 28
		{Name: "tsmc4", Width: 8, Kind: LocTSMC}, // 29
		{Name: "tsmc5", Width: 8, Kind: LocTSMC}, // 30
		{Name: "tsmc6", Width: 8, Kind: LocTSMC}, // 31
		{Name: "tsmc7", Width: 8, Kind: LocTSMC}, // 32

		// L5: Memory spill slots (index 33-36) — absolute address LD (nn)
		{Name: "_vir_mem0", Width: 16, Kind: LocMem}, // 33
		{Name: "_vir_mem1", Width: 16, Kind: LocMem}, // 34
		{Name: "_vir_mem2", Width: 16, Kind: LocMem}, // 35
		{Name: "_vir_mem3", Width: 16, Kind: LocMem}, // 36

		// L6: Stack slots (index 37-40) — PUSH/POP
		{Name: "stk0", Width: 16, Kind: LocStack}, // 37
		{Name: "stk1", Width: 16, Kind: LocStack}, // 38
		{Name: "stk2", Width: 16, Kind: LocStack}, // 39
		{Name: "stk3", Width: 16, Kind: LocStack}, // 40
	},
}

// Precomputed LocSet groups for Z80 (set in init).
var (
	Z80_A        LocSet
	Z80_GPR8     LocSet // {A,B,C,D,E,H,L}
	Z80_GPRNoHL  LocSet // {A,B,C,D,E} — compatible with DD/FD prefix
	Z80_Pairs    LocSet // {BC,DE,HL}
	Z80_HL       LocSet
	Z80_DE       LocSet
	Z80_BC       LocSet
	Z80_IX       LocSet
	Z80_IY       LocSet
	Z80_Flags    LocSet
	Z80_IXHalves LocSet // {IXH,IXL,IYH,IYL}
	Z80_HL8      LocSet // {H,L} — GPR8 excluded from DD/FD prefix moves
	Z80_Shadow8  LocSet // {B',C',D',E',H',L'}
	Z80_ShadowA  LocSet // {A'}
	Z80_TSMC     LocSet // {tsmc0..tsmc7}
	Z80_Mem      LocSet // {mem0..mem3}
	Z80_Stack    LocSet // {stk0..stk3}
)

func init() {
	m := Z80

	// Per-location cost in T-states
	m.LocCost = []int{
		0, 0, 0, 0, 0, 0, 0,       // L1: A,B,C,D,E,H,L
		0, 0, 0, 0,                 // BC,DE,HL,SP
		4, 4,                       // IX,IY (DD/FD prefix)
		0,                          // F (flags)
		1, 1, 1, 1,                 // L2: IXH,IXL,IYH,IYL (cost=1: slight preference for primary, but usable)
		8, 8, 8, 8, 8, 8,          // L3: B',C',D',E',H',L'
		8,                          // A' (EX AF,AF')
		20, 20, 20, 20, 20, 20, 20, 20, // L4: tsmc0-7
		26, 26, 26, 26,             // L5: mem0-3
		22, 22, 22, 22,             // L6: stk0-3
	}

	// Precompute LocSet groups
	Z80_A = m.LocSetByNames("A")
	Z80_GPR8 = m.LocSetByNames("A", "B", "C", "D", "E", "H", "L")
	Z80_GPRNoHL = m.LocSetByNames("A", "B", "C", "D", "E")
	Z80_Pairs = m.LocSetByNames("BC", "DE", "HL")
	Z80_HL = m.LocSetByNames("HL")
	Z80_DE = m.LocSetByNames("DE")
	Z80_BC = m.LocSetByNames("BC")
	Z80_IX = m.LocSetByNames("IX")
	Z80_IY = m.LocSetByNames("IY")
	Z80_Flags = m.LocSetByNames("F")
	Z80_IXHalves = m.LocSetByNames("IXH", "IXL", "IYH", "IYL")
	Z80_HL8 = m.LocSetByNames("H", "L")
	Z80_Shadow8 = m.LocSetByNames("B'", "C'", "D'", "E'", "H'", "L'")
	Z80_ShadowA = m.LocSetByNames("A'")
	Z80_TSMC = m.LocSetByNames("tsmc0", "tsmc1", "tsmc2", "tsmc3",
		"tsmc4", "tsmc5", "tsmc6", "tsmc7")
	Z80_Mem = m.LocSetByNames("_vir_mem0", "_vir_mem1", "_vir_mem2", "_vir_mem3")
	Z80_Stack = m.LocSetByNames("stk0", "stk1", "stk2", "stk3")

	// Exclude from general Z3 vreg allocation:
	// - F (flags): not freely readable/writable as storage
	// - SP (stack pointer): must not be reassigned
	// - Shadow regs (B',C',...,A'): EXX/EX AF,AF' swaps the whole bank atomically;
	//   individual shadow moves don't exist. EXX zone support is not yet wired.
	// - TSMC spill slots: require self-modifying code setup
	// - mem/stack spill slots: require special emit infrastructure (labels, PUSH/POP frames)
	//   that cfgsolver doesn't produce; let PBQP handle spilling instead.
	m.NonAllocatable = m.LocSetByNames("F", "SP").
		Or(Z80_Shadow8).Or(Z80_ShadowA).
		Or(Z80_TSMC).Or(Z80_Mem).Or(Z80_Stack)

	m.Patterns = generateZ80Patterns(m)
	m.Rules = generateZ80Rules(m)
}

func generateZ80Patterns(m *MachineDesc) []Pattern {
	return []Pattern{
		// ── Constants ─────────────────────────────────────────────────
		{Name: "ld_r_n", Op: OpConst, Width: 8, DstLocs: Z80_GPR8,
			Template: "LD {dst}, {imm}", Cost: 7, Bytes: 2, Flags: PatImmediate},
		{Name: "ld_rr_nn", Op: OpConst, Width: 16,
			DstLocs: Z80_Pairs.Or(Z80_IX).Or(Z80_IY),
			Template: "LD {dst}, {imm}", Cost: 10, Bytes: 3, Flags: PatImmediate},

		// ── Constants to IXH (spill-tier storage) ────────────────────
		{Name: "ld_ixh_n", Op: OpConst, Width: 8, DstLocs: Z80_IXHalves,
			Template: "LD {dst}, {imm}", Cost: 11, Bytes: 3, Flags: PatImmediate},

		// ── 8-bit moves ──────────────────────────────────────────────
		{Name: "ld_r_r", Op: OpMove, Width: 8, DstLocs: Z80_GPR8,
			SrcLocs: [2]LocSet{Z80_GPR8}, Template: "LD {dst}, {src0}",
			Cost: 4, Bytes: 1},

		// Flag→A materialization: SBC A,A produces 0xFF (CY=1) or 0x00 (CY=0).
		// Used for bool return values conveyed via CY flag.
		{Name: "sbc_a_a", Op: OpMove, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_Flags}, Template: "SBC A, A",
			Cost: 4, Bytes: 1},

		// Truncation: pair → low byte (zero-cost alias)
		{Name: "trunc_hl_l", Op: OpMove, Width: 8,
			DstLocs: m.LocSetByNames("L"), SrcLocs: [2]LocSet{Z80_HL},
			Template: "; trunc HL->L (alias)", Cost: 0, Bytes: 0},
		{Name: "trunc_de_e", Op: OpMove, Width: 8,
			DstLocs: m.LocSetByNames("E"), SrcLocs: [2]LocSet{Z80_DE},
			Template: "; trunc DE->E (alias)", Cost: 0, Bytes: 0},
		{Name: "trunc_bc_c", Op: OpMove, Width: 8,
			DstLocs: m.LocSetByNames("C"), SrcLocs: [2]LocSet{Z80_BC},
			Template: "; trunc BC->C (alias)", Cost: 0, Bytes: 0},

		// Truncation to A (for ALU consumer)
		{Name: "trunc_hl_a", Op: OpMove, Width: 8,
			DstLocs: Z80_A, SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD A, L", Cost: 4, Bytes: 1},
		{Name: "trunc_de_a", Op: OpMove, Width: 8,
			DstLocs: Z80_A, SrcLocs: [2]LocSet{Z80_DE},
			Template: "LD A, E", Cost: 4, Bytes: 1},
		{Name: "trunc_bc_a", Op: OpMove, Width: 8,
			DstLocs: Z80_A, SrcLocs: [2]LocSet{Z80_BC},
			Template: "LD A, C", Cost: 4, Bytes: 1},
		// Truncation to any GPR8 (low byte of pair)
		{Name: "trunc_hl_r", Op: OpMove, Width: 8,
			DstLocs: Z80_GPR8, SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD {dst}, L", Cost: 4, Bytes: 1},
		{Name: "trunc_de_r", Op: OpMove, Width: 8,
			DstLocs: Z80_GPR8, SrcLocs: [2]LocSet{Z80_DE},
			Template: "LD {dst}, E", Cost: 4, Bytes: 1},
		{Name: "trunc_bc_r", Op: OpMove, Width: 8,
			DstLocs: Z80_GPR8, SrcLocs: [2]LocSet{Z80_BC},
			Template: "LD {dst}, C", Cost: 4, Bytes: 1},

		// IX/IY half moves (call-safe, DD/FD prefix)
		// Direct: A,B,C,D,E ↔ IXH/IXL/IYH/IYL (no H,L — DD/FD conflict)
		{Name: "ld_ixh_r", Op: OpMove, Width: 8, DstLocs: Z80_IXHalves,
			SrcLocs: [2]LocSet{Z80_GPRNoHL}, Template: "LD {dst}, {src0}",
			Cost: 8, Bytes: 2},
		{Name: "ld_r_ixh", Op: OpMove, Width: 8, DstLocs: Z80_GPRNoHL,
			SrcLocs: [2]LocSet{Z80_IXHalves}, Template: "LD {dst}, {src0}",
			Cost: 8, Bytes: 2},
		// H/L ↔ IXHalves: DD/FD prefix replaces H/L with IXH/IXL — must go via A
		// LD A, H (4T,1B) + LD IXH, A (8T,2B) = 12T, 3B
		{Name: "ld_ixh_hl8", Op: OpMove, Width: 8, DstLocs: Z80_IXHalves,
			SrcLocs: [2]LocSet{Z80_HL8}, Template: "LD A, {src0}\n    LD {dst}, A",
			Cost: 12, Bytes: 3},
		{Name: "ld_hl8_ixh", Op: OpMove, Width: 8, DstLocs: Z80_HL8,
			SrcLocs: [2]LocSet{Z80_IXHalves}, Template: "LD A, {src0}\n    LD {dst}, A",
			Cost: 12, Bytes: 3},
		// Pair → IXHalf truncation (extract low byte)
		// DE/BC: E and C are GPRNoHL, so DD/FD prefix works directly (8T,2B)
		{Name: "trunc_de_ixh", Op: OpMove, Width: 8, DstLocs: Z80_IXHalves,
			SrcLocs: [2]LocSet{Z80_DE}, Template: "LD {dst}, E",
			Cost: 8, Bytes: 2},
		{Name: "trunc_bc_ixh", Op: OpMove, Width: 8, DstLocs: Z80_IXHalves,
			SrcLocs: [2]LocSet{Z80_BC}, Template: "LD {dst}, C",
			Cost: 8, Bytes: 2},
		// HL→IXHalf: L is replaced by IXL in DD context, must go via A
		// LD A, L (4T,1B) + LD IXH, A (8T,2B) = 12T, 3B
		{Name: "trunc_hl_ixh", Op: OpMove, Width: 8, DstLocs: Z80_IXHalves,
			SrcLocs: [2]LocSet{Z80_HL}, Template: "LD A, L\n    LD {dst}, A",
			Cost: 12, Bytes: 3},
		// IX→IXHalf: extract low byte via IXL (goes through A; src0 is the IX pair)
		// LD A, IXL (DD 7D, 8T,2B) + LD {dst}, A (8T,2B) = 16T, 4B
		{Name: "trunc_ix_ixhalf", Op: OpMove, Width: 8, DstLocs: Z80_IXHalves,
			SrcLocs: [2]LocSet{Z80_IX}, Template: "LD A, IXL\n    LD {dst}, A",
			Cost: 16, Bytes: 4},
		// IY→IXHalf: extract low byte via IYL
		// LD A, IYL (FD 7D, 8T,2B) + LD {dst}, A (8T,2B) = 16T, 4B
		{Name: "trunc_iy_ixhalf", Op: OpMove, Width: 8, DstLocs: Z80_IXHalves,
			SrcLocs: [2]LocSet{Z80_IY}, Template: "LD A, IYL\n    LD {dst}, A",
			Cost: 16, Bytes: 4},
		// IXHalf→IXHalf: both in DD/FD context, must go via A (16T, 4B)
		{Name: "ld_ixhalf_ixhalf", Op: OpMove, Width: 8, DstLocs: Z80_IXHalves,
			SrcLocs: [2]LocSet{Z80_IXHalves}, Template: "LD A, {src0}\n    LD {dst}, A",
			Cost: 16, Bytes: 4},
		// IXHalf → pair (store into low byte): fixes 'LD BC, IXH' invalid emit.
		// findMovePatternRemap dst-remap emits with wrong dst; explicit patterns avoid this.
		// BC/DE: C and E are GPRNoHL, direct (8T,2B)
		{Name: "trunc_ixh_bc", Op: OpMove, Width: 8, DstLocs: Z80_BC,
			SrcLocs: [2]LocSet{Z80_IXHalves}, Template: "LD C, {src0}",
			Cost: 8, Bytes: 2},
		{Name: "trunc_ixh_de", Op: OpMove, Width: 8, DstLocs: Z80_DE,
			SrcLocs: [2]LocSet{Z80_IXHalves}, Template: "LD E, {src0}",
			Cost: 8, Bytes: 2},
		// HL: L is HL8, must go via A (12T,3B)
		{Name: "trunc_ixh_hl", Op: OpMove, Width: 8, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{Z80_IXHalves}, Template: "LD A, {src0}\n    LD L, A",
			Cost: 12, Bytes: 3},

		// Zero-extend 8→16
		{Name: "zext_hl_r", Op: OpMove, Width: 16, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{Z80_GPR8},
			Template: "LD L, {src0}\n    LD H, 0", Cost: 11, Bytes: 3},
		{Name: "zext_bc_r", Op: OpMove, Width: 16, DstLocs: Z80_BC,
			SrcLocs: [2]LocSet{Z80_GPR8},
			Template: "LD C, {src0}\n    LD B, 0", Cost: 11, Bytes: 3},
		{Name: "zext_de_r", Op: OpMove, Width: 16, DstLocs: Z80_DE,
			SrcLocs: [2]LocSet{Z80_GPR8},
			Template: "LD E, {src0}\n    LD D, 0", Cost: 11, Bytes: 3},

		// 16-bit moves
		{Name: "push_pop", Op: OpMove, Width: 16, DstLocs: Z80_Pairs,
			SrcLocs: [2]LocSet{Z80_Pairs},
			Template: "PUSH {src0}\n    POP {dst}", Cost: 21, Bytes: 2},
		// IX/IY ↔ pairs (PUSH/POP is only general path; LD IX,HL doesn't exist)
		// PUSH IX (DD E5) = 15T,2B; POP rr = 10T,1B → 25T, 3B
		{Name: "push_pop_ix_pair", Op: OpMove, Width: 16, DstLocs: Z80_Pairs,
			SrcLocs: [2]LocSet{Z80_IX}, Template: "PUSH {src0}\n    POP {dst}",
			Cost: 25, Bytes: 3},
		{Name: "push_pop_iy_pair", Op: OpMove, Width: 16, DstLocs: Z80_Pairs,
			SrcLocs: [2]LocSet{Z80_IY}, Template: "PUSH {src0}\n    POP {dst}",
			Cost: 25, Bytes: 3},
		// PUSH rr = 11T,1B; POP IX (DD E1) = 14T,2B → 25T, 3B
		{Name: "push_pop_pair_ix", Op: OpMove, Width: 16, DstLocs: Z80_IX,
			SrcLocs: [2]LocSet{Z80_Pairs}, Template: "PUSH {src0}\n    POP {dst}",
			Cost: 25, Bytes: 3},
		{Name: "push_pop_pair_iy", Op: OpMove, Width: 16, DstLocs: Z80_IY,
			SrcLocs: [2]LocSet{Z80_Pairs}, Template: "PUSH {src0}\n    POP {dst}",
			Cost: 25, Bytes: 3},
		// PUSH IX (15T,2B) + POP IY (DD E1, 14T,2B) = 29T, 4B
		{Name: "push_pop_ix_iy", Op: OpMove, Width: 16, DstLocs: Z80_IY,
			SrcLocs: [2]LocSet{Z80_IX}, Template: "PUSH {src0}\n    POP {dst}",
			Cost: 29, Bytes: 4},
		{Name: "push_pop_iy_ix", Op: OpMove, Width: 16, DstLocs: Z80_IX,
			SrcLocs: [2]LocSet{Z80_IY}, Template: "PUSH {src0}\n    POP {dst}",
			Cost: 29, Bytes: 4},
		// SP ↔ HL
		// LD SP, HL = F9 = 6T, 1B
		{Name: "ld_sp_hl", Op: OpMove, Width: 16, DstLocs: m.LocSetByNames("SP"),
			SrcLocs: [2]LocSet{Z80_HL}, Template: "LD SP, HL",
			Cost: 6, Bytes: 1},
		// LD HL, 0 (10T,3B) + ADD HL, SP (11T,1B) = 21T, 4B
		{Name: "ld_hl_sp", Op: OpMove, Width: 16, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{m.LocSetByNames("SP")},
			Template: "LD HL, 0\n    ADD HL, SP", Cost: 21, Bytes: 4},
		{Name: "ex_de_hl", Op: OpMove, Width: 16, DstLocs: Z80_DE,
			SrcLocs: [2]LocSet{Z80_HL},
			Template: "EX DE, HL", Cost: 4, Bytes: 1},
		{Name: "ex_hl_de", Op: OpMove, Width: 16, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{Z80_DE},
			Template: "EX DE, HL", Cost: 4, Bytes: 1},
		// LD HL, BC via LD H,B / LD L,C (8T, 2 bytes — cheaper than PUSH/POP)
		{Name: "ld_hl_bc", Op: OpMove, Width: 16, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{Z80_BC},
			Template: "LD H, B\n    LD L, C", Cost: 8, Bytes: 2},
		{Name: "ld_bc_hl", Op: OpMove, Width: 16, DstLocs: Z80_BC,
			SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD B, H\n    LD C, L", Cost: 8, Bytes: 2},
		{Name: "ld_de_bc", Op: OpMove, Width: 16, DstLocs: Z80_DE,
			SrcLocs: [2]LocSet{Z80_BC},
			Template: "LD D, B\n    LD E, C", Cost: 8, Bytes: 2},
		{Name: "ld_bc_de", Op: OpMove, Width: 16, DstLocs: Z80_BC,
			SrcLocs: [2]LocSet{Z80_DE},
			Template: "LD B, D\n    LD C, E", Cost: 8, Bytes: 2},
		{Name: "ld_de_hl", Op: OpMove, Width: 16, DstLocs: Z80_DE,
			SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD D, H\n    LD E, L", Cost: 8, Bytes: 2},
		{Name: "ld_hl_de", Op: OpMove, Width: 16, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{Z80_DE},
			Template: "LD H, D\n    LD L, E", Cost: 8, Bytes: 2},

		// ── 8-bit ALU (A = accumulator, dst tied to src0) ───────────
		{Name: "add_a_r", Op: OpAdd, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A, Z80_GPR8},
			Template: "ADD A, {src1}", Cost: 4, Bytes: 1,
			Clobbers: Z80_Flags, Flags: PatCommutative, TiedDstSrc: true},
		{Name: "sub_a_r", Op: OpSub, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A, Z80_GPR8},
			Template: "SUB {src1}", Cost: 4, Bytes: 1,
			Clobbers: Z80_Flags, TiedDstSrc: true},
		{Name: "and_a_r", Op: OpAnd, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A, Z80_GPR8},
			Template: "AND {src1}", Cost: 4, Bytes: 1,
			Clobbers: Z80_Flags, Flags: PatCommutative, TiedDstSrc: true},
		{Name: "or_a_r", Op: OpOr, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A, Z80_GPR8},
			Template: "OR {src1}", Cost: 4, Bytes: 1,
			Clobbers: Z80_Flags, Flags: PatCommutative, TiedDstSrc: true},
		{Name: "xor_a_r", Op: OpXor, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A, Z80_GPR8},
			Template: "XOR {src1}", Cost: 4, Bytes: 1,
			Clobbers: Z80_Flags, Flags: PatCommutative, TiedDstSrc: true},
		{Name: "cp_r", Op: OpCmp, Width: 8, DstLocs: Z80_Flags,
			SrcLocs: [2]LocSet{Z80_A, Z80_GPR8},
			Template: "CP {src1}", Cost: 4, Bytes: 1, Clobbers: Z80_Flags},

		// 8-bit ALU immediate (dst tied to src0 = A)
		{Name: "add_a_n", Op: OpAddImm, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A},
			Template: "ADD A, {imm}", Cost: 7, Bytes: 2,
			Clobbers: Z80_Flags, Flags: PatImmediate, TiedDstSrc: true},
		{Name: "sub_a_n", Op: OpSubImm, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A},
			Template: "SUB {imm}", Cost: 7, Bytes: 2,
			Clobbers: Z80_Flags, Flags: PatImmediate, TiedDstSrc: true},
		{Name: "cp_n", Op: OpCmpImm, Width: 8, DstLocs: Z80_Flags,
			SrcLocs: [2]LocSet{Z80_A},
			Template: "CP {imm}", Cost: 7, Bytes: 2,
			Clobbers: Z80_Flags, Flags: PatImmediate},
		{Name: "and_a_n", Op: OpAndImm, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A},
			Template: "AND {imm}", Cost: 7, Bytes: 2,
			Clobbers: Z80_Flags, Flags: PatImmediate, TiedDstSrc: true},
		{Name: "or_a_n", Op: OpOrImm, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A},
			Template: "OR {imm}", Cost: 7, Bytes: 2,
			Clobbers: Z80_Flags, Flags: PatImmediate, TiedDstSrc: true},
		{Name: "xor_a_n", Op: OpXorImm, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A},
			Template: "XOR {imm}", Cost: 7, Bytes: 2,
			Clobbers: Z80_Flags, Flags: PatImmediate, TiedDstSrc: true},

		// INC/DEC (any GPR, in-place, immediate +1/-1 only, dst tied to src0)
		{Name: "inc_r", Op: OpAddImm, Width: 8, DstLocs: Z80_GPR8,
			SrcLocs:  [2]LocSet{Z80_GPR8},
			Template: "INC {dst}", Cost: 4, Bytes: 1, ImmGuard: immGuard(1),
			Flags: PatImmediate, TiedDstSrc: true},
		{Name: "dec_r", Op: OpSubImm, Width: 8, DstLocs: Z80_GPR8,
			SrcLocs:  [2]LocSet{Z80_GPR8},
			Template: "DEC {dst}", Cost: 4, Bytes: 1, ImmGuard: immGuard(1),
			Flags: PatImmediate, TiedDstSrc: true},

		// 8-bit self-add: ADD A, A (x+x = double, dst tied to src0)
		{Name: "add_a_a", Op: OpAdd, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A, Z80_A},
			Template: "ADD A, A", Cost: 4, Bytes: 1,
			Clobbers: Z80_Flags, SelfSrc: true, TiedDstSrc: true},

		// ── 16-bit ALU (HL destination) ──────────────────────────────
		{Name: "add_hl_rr", Op: OpAdd, Width: 16, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{Z80_HL, Z80_BC.Or(Z80_DE).Or(Z80_HL)},
			Template: "ADD HL, {src1}", Cost: 11, Bytes: 1,
			Clobbers: Z80_Flags, TiedDstSrc: true},
		{Name: "sbc_hl_rr", Op: OpSub, Width: 16, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{Z80_HL, Z80_BC.Or(Z80_DE).Or(Z80_HL)},
			Template: "OR A\n    SBC HL, {src1}", Cost: 19, Bytes: 3,
			Clobbers: Z80_Flags, TiedDstSrc: true},
		{Name: "inc_rr", Op: OpAddImm, Width: 16, DstLocs: Z80_Pairs,
			SrcLocs:  [2]LocSet{Z80_Pairs},
			Template: "INC {dst}", Cost: 6, Bytes: 1, ImmGuard: immGuard(1),
			Flags: PatImmediate, TiedDstSrc: true},
		// General 16-bit add immediate: LD BC,N / ADD HL,BC
		{Name: "add_hl_nn", Op: OpAddImm, Width: 16, DstLocs: Z80_HL,
			SrcLocs:  [2]LocSet{Z80_HL},
			Template: "LD BC, {imm}\n    ADD HL, BC", Cost: 21, Bytes: 4,
			Clobbers: Z80_Flags.Or(Z80_BC), Flags: PatImmediate, TiedDstSrc: true},

		// ── 16-bit ALU via EX DE,HL (value in DE, avoids HL contention) ─
		// add16_de: EX DE,HL / ADD HL,BC / EX DE,HL — DE = DE + BC
		{Name: "add16_de_bc", Op: OpAdd, Width: 16, DstLocs: Z80_DE,
			SrcLocs: [2]LocSet{Z80_DE, Z80_BC},
			Template: "EX DE, HL\n    ADD HL, BC\n    EX DE, HL",
			Cost: 19, Bytes: 3, Clobbers: Z80_Flags, TiedDstSrc: true},
		// add16_de_hl: EX DE,HL / ADD HL,DE / EX DE,HL — DE = DE + HL
		// After first EX: old_DE→HL, old_HL→DE. ADD HL, DE = old_DE + old_HL. EX: result→DE.
		{Name: "add16_de_hl", Op: OpAdd, Width: 16, DstLocs: Z80_DE,
			SrcLocs: [2]LocSet{Z80_DE, Z80_HL},
			Template: "EX DE, HL\n    ADD HL, DE\n    EX DE, HL",
			Cost: 19, Bytes: 3, Clobbers: Z80_Flags, TiedDstSrc: true},
		// sub16_de: EX DE,HL / OR A / SBC HL,BC / EX DE,HL
		{Name: "sub16_de_bc", Op: OpSub, Width: 16, DstLocs: Z80_DE,
			SrcLocs: [2]LocSet{Z80_DE, Z80_BC},
			Template: "EX DE, HL\n    OR A\n    SBC HL, BC\n    EX DE, HL",
			Cost: 27, Bytes: 5, Clobbers: Z80_Flags, TiedDstSrc: true},

		// ── 16-bit ALU via IX/IY (additional accumulators) ──────
		// ADD IX, rr: DD 09/19/29 — src1 = BC, DE, or IX only
		{Name: "add_ix_rr", Op: OpAdd, Width: 16, DstLocs: Z80_IX,
			SrcLocs: [2]LocSet{Z80_IX, Z80_BC.Or(Z80_DE).Or(Z80_IX)},
			Template: "ADD IX, {src1}", Cost: 15, Bytes: 2,
			Clobbers: Z80_Flags, TiedDstSrc: true},
		// ADD IY, rr: FD 09/19/29
		{Name: "add_iy_rr", Op: OpAdd, Width: 16, DstLocs: Z80_IY,
			SrcLocs: [2]LocSet{Z80_IY, Z80_BC.Or(Z80_DE).Or(Z80_IY)},
			Template: "ADD IY, {src1}", Cost: 15, Bytes: 2,
			Clobbers: Z80_Flags, TiedDstSrc: true},
		// INC IX/IY
		{Name: "inc_ix", Op: OpAddImm, Width: 16, DstLocs: Z80_IX,
			SrcLocs: [2]LocSet{Z80_IX},
			Template: "INC IX", Cost: 10, Bytes: 2, ImmGuard: immGuard(1),
			Flags: PatImmediate, TiedDstSrc: true},
		{Name: "inc_iy", Op: OpAddImm, Width: 16, DstLocs: Z80_IY,
			SrcLocs: [2]LocSet{Z80_IY},
			Template: "INC IY", Cost: 10, Bytes: 2, ImmGuard: immGuard(1),
			Flags: PatImmediate, TiedDstSrc: true},
		// addImm via DE: EX DE,HL / LD BC,N / ADD HL,BC / EX DE,HL
		{Name: "add_de_nn", Op: OpAddImm, Width: 16, DstLocs: Z80_DE,
			SrcLocs:  [2]LocSet{Z80_DE},
			Template: "EX DE, HL\n    LD BC, {imm}\n    ADD HL, BC\n    EX DE, HL",
			Cost: 29, Bytes: 6, Clobbers: Z80_Flags.Or(Z80_BC),
			Flags: PatImmediate, TiedDstSrc: true},

		// ── 16-bit load/store via DE (EX DE,HL bracket) ──────────────
		// Load 16-bit via DE pointer
		// EX(4T)+LD A,(HL)(7T)+INC HL(6T)+LD H,(HL)(7T)+LD L,A(4T)+EX(4T) = 32T
		{Name: "ld16_de_ind", Op: OpLoad, Width: 16, DstLocs: Z80_DE,
			SrcLocs: [2]LocSet{Z80_DE},
			Template: "EX DE, HL\n    LD A, (HL)\n    INC HL\n    LD H, (HL)\n    LD L, A\n    EX DE, HL",
			Cost: 32, Bytes: 6, Flags: PatMemRead, TiedDstSrc: true},
		// Store 16-bit: ptr in DE, value in HL (EX bracket)
		// EX(4T)+LD(HL),E(7T)+INC HL(6T)+LD(HL),D(7T)+DEC HL(6T)+EX(4T) = 34T
		{Name: "st16_de_hl", Op: OpStore, Width: 16,
			SrcLocs: [2]LocSet{Z80_DE, Z80_HL},
			Template: "EX DE, HL\n    LD (HL), E\n    INC HL\n    LD (HL), D\n    DEC HL\n    EX DE, HL",
			Cost: 34, Bytes: 6, Flags: PatMemWrite},
		// Store 16-bit: ptr in DE, value in HL (via (DE), A as temp)
		// LD A,L(4T)+LD(DE),A(7T)+INC DE(6T)+LD A,H(4T)+LD(DE),A(7T)+DEC DE(6T) = 34T
		{Name: "st16_de_hl_via_a", Op: OpStore, Width: 16,
			SrcLocs: [2]LocSet{Z80_DE, Z80_HL},
			Template: "LD A, L\n    LD (DE), A\n    INC DE\n    LD A, H\n    LD (DE), A\n    DEC DE",
			Cost: 34, Bytes: 6, Flags: PatMemWrite,
			Clobbers: Z80_A},
		// Store 16-bit: ptr in DE, value in BC (EX bracket)
		// EX(4T)+LD(HL),C(7T)+INC HL(6T)+LD(HL),B(7T)+DEC HL(6T)+EX(4T) = 34T
		{Name: "st16_de_bc", Op: OpStore, Width: 16,
			SrcLocs: [2]LocSet{Z80_DE, Z80_BC},
			Template: "EX DE, HL\n    LD (HL), C\n    INC HL\n    LD (HL), B\n    DEC HL\n    EX DE, HL",
			Cost: 34, Bytes: 6, Flags: PatMemWrite},
		// Store 16-bit: ptr in DE, value in BC (via (DE), A as temp)
		// LD A,C(4T)+LD(DE),A(7T)+INC DE(6T)+LD A,B(4T)+LD(DE),A(7T)+DEC DE(6T) = 34T
		{Name: "st16_de_bc_via_a", Op: OpStore, Width: 16,
			SrcLocs: [2]LocSet{Z80_DE, Z80_BC},
			Template: "LD A, C\n    LD (DE), A\n    INC DE\n    LD A, B\n    LD (DE), A\n    DEC DE",
			Cost: 34, Bytes: 6, Flags: PatMemWrite,
			Clobbers: Z80_A},

		// 16-bit compare
		// OR A(4T) + SBC HL,rr(15T) + ADD HL,rr(11T) = 30T, 4B
		{Name: "cmp16_hl_de", Op: OpCmp, Width: 16, DstLocs: Z80_Flags,
			SrcLocs: [2]LocSet{Z80_HL, Z80_DE},
			Template: "OR A\n    SBC HL, DE\n    ADD HL, DE",
			Cost: 30, Bytes: 4, Clobbers: Z80_Flags},
		{Name: "cmp16_hl_bc", Op: OpCmp, Width: 16, DstLocs: Z80_Flags,
			SrcLocs: [2]LocSet{Z80_HL, Z80_BC},
			Template: "OR A\n    SBC HL, BC\n    ADD HL, BC",
			Cost: 30, Bytes: 4, Clobbers: Z80_Flags},

		// ── Shifts ───────────────────────────────────────────────────
		{Name: "sla_a", Op: OpShl, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A, Z80_GPR8},
			Template: "SLA A", Cost: 8, Bytes: 2,
			Clobbers: Z80_Flags, TiedDstSrc: true},
		{Name: "srl_a", Op: OpShr, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A, Z80_GPR8},
			Template: "SRL A", Cost: 8, Bytes: 2,
			Clobbers: Z80_Flags, TiedDstSrc: true},
		{Name: "shl16_hl", Op: OpShl, Width: 16, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{Z80_HL, Z80_GPR8},
			Template: "ADD HL, HL", Cost: 11, Bytes: 1,
			Clobbers: Z80_Flags, ImmGuard: immGuard(1), TiedDstSrc: true},

		// ── Memory loads — 8-bit ─────────────────────────────────────
		{Name: "ld_a_hl", Op: OpLoad, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD A, (HL)", Cost: 7, Bytes: 1, Flags: PatMemRead},
		{Name: "ld_r_hl", Op: OpLoad, Width: 8, DstLocs: Z80_GPR8,
			SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD {dst}, (HL)", Cost: 7, Bytes: 1, Flags: PatMemRead},
		{Name: "ld_a_de", Op: OpLoad, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_DE},
			Template: "LD A, (DE)", Cost: 7, Bytes: 1, Flags: PatMemRead},

		// ── Memory loads — 16-bit ────────────────────────────────────
		// Load 16-bit via HL pointer: LD A,(HL)(7T) / INC HL(6T) / LD H,(HL)(7T) / LD L,A(4T) = 24T
		{Name: "ld16_hl_ind", Op: OpLoad, Width: 16, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD A, (HL)\n    INC HL\n    LD H, (HL)\n    LD L, A",
			Cost: 24, Bytes: 4, Flags: PatMemRead, TiedDstSrc: true},
		// Load16LE: fused FatFS ld_word — same Z80 code
		{Name: "load16_le_hl", Op: OpLoad16LE, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD A, (HL)\n    INC HL\n    LD H, (HL)\n    LD L, A",
			Cost: 24, Bytes: 4, Flags: PatMemRead, TiedDstSrc: true},

		// ── Memory stores — 8-bit ────────────────────────────────────
		{Name: "ld_hl_a", Op: OpStore, Width: 8,
			SrcLocs: [2]LocSet{Z80_HL, Z80_A},
			Template: "LD (HL), A", Cost: 7, Bytes: 1, Flags: PatMemWrite},
		{Name: "ld_hl_r", Op: OpStore, Width: 8,
			SrcLocs: [2]LocSet{Z80_HL, Z80_GPR8},
			Template: "LD (HL), {src1}", Cost: 7, Bytes: 1, Flags: PatMemWrite},

		// ── Memory stores — 16-bit ───────────────────────────────────
		// Store 16-bit via HL pointer, value in DE
		// LD (HL),E(7T) + INC HL(6T) + LD (HL),D(7T) + DEC HL(6T) = 26T
		{Name: "st16_hl_de", Op: OpStore, Width: 16,
			SrcLocs: [2]LocSet{Z80_HL, Z80_DE},
			Template: "LD (HL), E\n    INC HL\n    LD (HL), D\n    DEC HL",
			Cost: 26, Bytes: 4, Flags: PatMemWrite},
		// Store 16-bit via HL pointer, value in BC
		{Name: "st16_hl_bc", Op: OpStore, Width: 16,
			SrcLocs: [2]LocSet{Z80_HL, Z80_BC},
			Template: "LD (HL), C\n    INC HL\n    LD (HL), B\n    DEC HL",
			Cost: 26, Bytes: 4, Flags: PatMemWrite},

		// ── Global store/load ────────────────────────────────────────
		{Name: "st_global_hl", Op: OpStoreGlobal, Width: 16,
			SrcLocs: [2]LocSet{Z80_HL, Z80_HL},
			Template: "LD ({imm}), HL", Cost: 16, Bytes: 3,
			Flags: PatMemWrite | PatImmediate},
		{Name: "st_global_a", Op: OpStoreGlobal, Width: 8,
			SrcLocs: [2]LocSet{Z80_A, Z80_A},
			Template: "LD ({imm}), A", Cost: 13, Bytes: 3,
			Flags: PatMemWrite | PatImmediate},
		{Name: "ld_global_hl", Op: OpLoadGlobal, Width: 16,
			DstLocs: Z80_HL,
			Template: "LD HL, ({imm})", Cost: 16, Bytes: 3,
			Flags: PatMemRead | PatImmediate},

		// ── 16-bit move to/from memory (spill/reload) ───────────────
		{Name: "ld_nn_hl", Op: OpMove, Width: 16, DstLocs: Z80_Mem,
			SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD ({dst}), HL", Cost: 16, Bytes: 3, Flags: PatMemWrite},
		{Name: "ld_hl_nn", Op: OpMove, Width: 16, DstLocs: Z80_HL,
			SrcLocs: [2]LocSet{Z80_Mem},
			Template: "LD HL, ({src0})", Cost: 16, Bytes: 3, Flags: PatMemRead},

		// ── Negate ───────────────────────────────────────────────────
		{Name: "neg_a", Op: OpNeg, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_A},
			Template: "NEG", Cost: 8, Bytes: 2,
			Clobbers: Z80_Flags, TiedDstSrc: true},

		// ── Call / Return ────────────────────────────────────────────
		{Name: "call", Op: OpCall, DstLocs: Z80_A.Or(Z80_HL),
			Template: "CALL {imm}", Cost: 17, Bytes: 3,
			Flags: PatCall | PatImmediate},
		{Name: "ret", Op: OpRet,
			Template: "RET", Cost: 10, Bytes: 1},

		// ── Inline asm block ────────────────────────────────────────
		{Name: "asm_block", Op: OpAsmBlock,
			DstLocs: Z80_GPR8.Or(Z80_Pairs),
			Template: "", Cost: 10, Bytes: 0,
			Flags: PatCall}, // treated like a call (has clobbers)
	}
}

func generateZ80Rules(m *MachineDesc) []ConstraintRule {
	return []ConstraintRule{
		// DD/FD prefix conflict: H,L cannot be used with IX/IY half ops
		{Name: "dd_prefix_hl_conflict",
			DstSet: Z80_IXHalves,
			SrcSet: m.LocSetByNames("H", "L"),
			Cost:   MaxCost},
		// ADD HL,rr — src1 must be a pair, not IXH/IXL
		{Name: "add_hl_pair_only",
			DstSet: Z80_HL,
			SrcSet: Z80_IXHalves,
			OpMask: OpAdd,
			Cost:   MaxCost},
	}
}
