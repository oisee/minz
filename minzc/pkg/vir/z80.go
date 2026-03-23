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
		{Name: "mem0", Width: 16, Kind: LocMem}, // 33
		{Name: "mem1", Width: 16, Kind: LocMem}, // 34
		{Name: "mem2", Width: 16, Kind: LocMem}, // 35
		{Name: "mem3", Width: 16, Kind: LocMem}, // 36

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
		8, 8, 8, 8,                 // L2: IXH,IXL,IYH,IYL
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
	Z80_Shadow8 = m.LocSetByNames("B'", "C'", "D'", "E'", "H'", "L'")
	Z80_ShadowA = m.LocSetByNames("A'")
	Z80_TSMC = m.LocSetByNames("tsmc0", "tsmc1", "tsmc2", "tsmc3",
		"tsmc4", "tsmc5", "tsmc6", "tsmc7")
	Z80_Mem = m.LocSetByNames("mem0", "mem1", "mem2", "mem3")
	Z80_Stack = m.LocSetByNames("stk0", "stk1", "stk2", "stk3")

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

		// ── 8-bit moves ──────────────────────────────────────────────
		{Name: "ld_r_r", Op: OpMove, Width: 8, DstLocs: Z80_GPR8,
			SrcLocs: [2]LocSet{Z80_GPR8}, Template: "LD {dst}, {src0}",
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

		// Truncation routed through A (for ALU consumer)
		{Name: "trunc_hl_a", Op: OpMove, Width: 8,
			DstLocs: Z80_A, SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD A, L", Cost: 4, Bytes: 1},
		{Name: "trunc_de_a", Op: OpMove, Width: 8,
			DstLocs: Z80_A, SrcLocs: [2]LocSet{Z80_DE},
			Template: "LD A, E", Cost: 4, Bytes: 1},
		{Name: "trunc_bc_a", Op: OpMove, Width: 8,
			DstLocs: Z80_A, SrcLocs: [2]LocSet{Z80_BC},
			Template: "LD A, C", Cost: 4, Bytes: 1},

		// IX/IY half moves (call-safe, DD/FD prefix)
		{Name: "ld_ixh_r", Op: OpMove, Width: 8, DstLocs: Z80_IXHalves,
			SrcLocs: [2]LocSet{Z80_GPRNoHL}, Template: "LD {dst}, {src0}",
			Cost: 8, Bytes: 2},
		{Name: "ld_r_ixh", Op: OpMove, Width: 8, DstLocs: Z80_GPRNoHL,
			SrcLocs: [2]LocSet{Z80_IXHalves}, Template: "LD {dst}, {src0}",
			Cost: 8, Bytes: 2},

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
		{Name: "ex_de_hl", Op: OpMove, Width: 16, DstLocs: Z80_DE,
			SrcLocs: [2]LocSet{Z80_HL},
			Template: "EX DE, HL", Cost: 4, Bytes: 1},

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

		// 16-bit compare
		{Name: "cmp16_hl_de", Op: OpCmp, Width: 16, DstLocs: Z80_Flags,
			SrcLocs: [2]LocSet{Z80_HL, Z80_DE},
			Template: "OR A\n    SBC HL, DE\n    ADD HL, DE",
			Cost: 26, Bytes: 4, Clobbers: Z80_Flags},
		{Name: "cmp16_hl_bc", Op: OpCmp, Width: 16, DstLocs: Z80_Flags,
			SrcLocs: [2]LocSet{Z80_HL, Z80_BC},
			Template: "OR A\n    SBC HL, BC\n    ADD HL, BC",
			Cost: 26, Bytes: 4, Clobbers: Z80_Flags},

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

		// ── Memory loads ─────────────────────────────────────────────
		{Name: "ld_a_hl", Op: OpLoad, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD A, (HL)", Cost: 7, Bytes: 1, Flags: PatMemRead},
		{Name: "ld_r_hl", Op: OpLoad, Width: 8, DstLocs: Z80_GPR8,
			SrcLocs: [2]LocSet{Z80_HL},
			Template: "LD {dst}, (HL)", Cost: 9, Bytes: 1, Flags: PatMemRead},
		{Name: "ld_a_de", Op: OpLoad, Width: 8, DstLocs: Z80_A,
			SrcLocs: [2]LocSet{Z80_DE},
			Template: "LD A, (DE)", Cost: 7, Bytes: 1, Flags: PatMemRead},

		// ── Memory stores ────────────────────────────────────────────
		{Name: "ld_hl_a", Op: OpStore, Width: 8,
			SrcLocs: [2]LocSet{Z80_HL, Z80_A},
			Template: "LD (HL), A", Cost: 7, Bytes: 1, Flags: PatMemWrite},
		{Name: "ld_hl_r", Op: OpStore, Width: 8,
			SrcLocs: [2]LocSet{Z80_HL, Z80_GPR8},
			Template: "LD (HL), {src1}", Cost: 7, Bytes: 1, Flags: PatMemWrite},

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
