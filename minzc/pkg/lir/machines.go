package lir

// ── Predefined Machine Descriptors ──────────────────────────────────────────
//
// These form a spectrum from "ideal" to "extremely constrained":
//
//   RISC32  →  RISC8  →  CISC  →  MICRO  →  Z80  →  6502
//   (easy)                                          (hard)
//
// Convergence testing: MIR2-VM result must match LIR-VM on ALL of these.
// If it matches on RISC32 but not CISC, the bug is in constraint handling.
// If it fails on all, the bug is in MIR2→LIR lowering (isel).

// RISC32 is an idealized RISC machine: 32 symmetric registers, all operations
// work on any register, no special cases. This is the "everything should work"
// baseline. If LIR-VM fails here, the isel is fundamentally broken.
var RISC32 = &MachineDesc{
	Name:     "risc32",
	WordSize: 16,
	Locs: func() []Loc {
		locs := make([]Loc, 32)
		for i := range locs {
			locs[i] = Loc{
				Name:  riscRegName(i),
				Width: 16,
				Kind:  LocReg,
			}
		}
		// r0 is also the accumulator (but any reg works for ALU)
		locs[0].Kind = LocAcc
		return locs
	}(),
	// Patterns: every op works on any register
	Patterns: nil, // filled by init
	Rules:    nil, // no constraints — fully orthogonal
}

// RISC8 is a reduced RISC: 8 symmetric registers, same orthogonality as RISC32
// but with real register pressure. Tests spilling and allocation under pressure.
var RISC8 = &MachineDesc{
	Name:     "risc8",
	WordSize: 16,
	Locs: func() []Loc {
		locs := make([]Loc, 8)
		for i := range locs {
			locs[i] = Loc{
				Name:  riscRegName(i),
				Width: 16,
				Kind:  LocReg,
			}
		}
		return locs
	}(),
	Patterns: nil,
	Rules:    nil,
}

// CISC is a Z80-like asymmetric machine WITHOUT the DD prefix madness.
// 2 accumulators (A for 8-bit ALU, HL for 16-bit), 2 pointer regs,
// 2 general purpose. Tests asymmetric allocation without encoding traps.
//
// Registers: A(8), F(flags), B(8), C(8), HL(16), DE(16)
// Constraints: 8-bit ALU → A only; 16-bit ADD → HL only
var CISC = &MachineDesc{
	Name:     "cisc",
	WordSize: 8,
	Locs: []Loc{
		{Name: "A", Width: 8, Kind: LocAcc},
		{Name: "B", Width: 8, Kind: LocReg},
		{Name: "C", Width: 8, Kind: LocReg},
		{Name: "D", Width: 8, Kind: LocReg},
		{Name: "E", Width: 8, Kind: LocReg},
		{Name: "HL", Width: 16, Kind: LocIndex},
		{Name: "DE", Width: 16, Kind: LocIndex},
		{Name: "BC", Width: 16, Kind: LocPair},
		{Name: "F", Width: 1, Kind: LocFlag},
		{Name: "mem", Width: 16, Kind: LocMem},
	},
	Rules: []ConstraintRule{
		// 8-bit ALU destination must be A
		{Name: "alu8_acc_only", DstSet: 0, SrcSet: 0, OpMask: 0, Cost: 0},
		// 16-bit ADD destination must be HL
		// (filled programmatically based on pattern DstLocs)
	},
}

// MICRO is a 6502-like extremely constrained machine.
// 1 accumulator (A), 2 index registers (X, Y), all 8-bit.
// 16-bit operations require pairs of 8-bit ops. Tests extreme pressure.
//
// Registers: A(8, ALU), X(8, index), Y(8, index), F(flags)
// Constraints: ALU → A only; indexing → X or Y only (not both modes)
var MICRO = &MachineDesc{
	Name:     "micro",
	WordSize: 8,
	Locs: []Loc{
		{Name: "A", Width: 8, Kind: LocAcc},
		{Name: "X", Width: 8, Kind: LocIndex},
		{Name: "Y", Width: 8, Kind: LocIndex},
		{Name: "F", Width: 1, Kind: LocFlag},
		{Name: "zp0", Width: 8, Kind: LocMem},
		{Name: "zp1", Width: 8, Kind: LocMem},
		{Name: "zp2", Width: 8, Kind: LocMem},
		{Name: "zp3", Width: 8, Kind: LocMem},
	},
	Rules: []ConstraintRule{
		// All ALU ops require A as destination
		{Name: "alu_acc_only", Cost: MaxCost},
	},
}

func riscRegName(i int) string {
	if i < 10 {
		return "r" + string(rune('0'+i))
	}
	return "r" + string(rune('0'+i/10)) + string(rune('0'+i%10))
}

// ── Pattern Generators ──────────────────────────────────────────────────────

// These will be filled by init() or by explicit calls.
// For now, define the MIR2 opcode constants we need (decoupled from mir2 package).

const (
	OpConst = 1
	OpMove  = 2
	OpAdd   = 3
	OpSub   = 4
	OpMul   = 5
	OpAnd   = 6
	OpOr    = 7
	OpXor   = 8
	OpCmp   = 9
	OpLoad  = 10
	OpStore = 11
	OpCall     = 12
	OpRet      = 13
	OpShl      = 14 // shift left
	OpShr      = 15 // shift right (logical)
	OpLoad16LE = 16 // combined: load 16-bit little-endian from memory address
)

// GenerateRISCPatterns creates fully orthogonal patterns for a RISC machine.
// Every register can be src or dst for every operation.
func GenerateRISCPatterns(m *MachineDesc) []Pattern {
	allRegs := m.LocsOfWidth(m.WordSize)
	all8 := m.LocsOfWidth(8)
	if allRegs.IsEmpty() {
		allRegs = all8
	}

	return []Pattern{
		{Name: "const", MIROp: OpConst, DstLocs: allRegs, Template: "li {dst}, {imm}", Cost: 1, Bytes: 4, Flags: PatImmediate},
		{Name: "move", MIROp: OpMove, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs}, Template: "mov {dst}, {src0}", Cost: 1, Bytes: 2},
		{Name: "add", MIROp: OpAdd, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "add {dst}, {src0}, {src1}", Cost: 1, Bytes: 4, Flags: PatCommutative},
		{Name: "sub", MIROp: OpSub, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "sub {dst}, {src0}, {src1}", Cost: 1, Bytes: 4},
		{Name: "shl", MIROp: OpShl, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "sll {dst}, {src0}, {src1}", Cost: 1, Bytes: 4},
		{Name: "shr", MIROp: OpShr, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "srl {dst}, {src0}, {src1}", Cost: 1, Bytes: 4},
		{Name: "and", MIROp: OpAnd, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "and {dst}, {src0}, {src1}", Cost: 1, Bytes: 4, Flags: PatCommutative},
		{Name: "or", MIROp: OpOr, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "or {dst}, {src0}, {src1}", Cost: 1, Bytes: 4, Flags: PatCommutative},
		{Name: "xor", MIROp: OpXor, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "xor {dst}, {src0}, {src1}", Cost: 1, Bytes: 4, Flags: PatCommutative},
		{Name: "cmp", MIROp: OpCmp, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "cmp {dst}, {src0}, {src1}", Cost: 1, Bytes: 4},
		{Name: "load", MIROp: OpLoad, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs}, Template: "ld {dst}, [{src0}]", Cost: 2, Bytes: 4, Flags: PatMemRead},
		{Name: "store", MIROp: OpStore, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "st [{src0}], {src1}", Cost: 2, Bytes: 4, Flags: PatMemWrite},
		// Combined 16-bit little-endian load: loads 2 bytes from [src0] into dst
		{Name: "load16_le", MIROp: OpLoad16LE, Width: 16, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs}, Template: "ld16 {dst}, [{src0}]", Cost: 3, Bytes: 4, Flags: PatMemRead},
	}
}

// GenerateCISCPatterns creates Z80-like asymmetric patterns.
// 8-bit ALU: A is destination, any 8-bit reg as source.
// 16-bit ADD: HL is destination, BC/DE as source.
// Moves: any reg→reg within same width.
func GenerateCISCPatterns(m *MachineDesc) []Pattern {
	a := m.LocSetByNames("A")
	regs8 := m.LocSetByNames("A", "B", "C", "D", "E")
	pairs := m.LocSetByNames("HL", "DE", "BC")
	hl := m.LocSetByNames("HL")
	bcde := m.LocSetByNames("BC", "DE")
	flags := m.LocSetByNames("F")
	mem := m.LocSetByNames("mem")
	allRegs := regs8.Or(pairs)

	return []Pattern{
		// Constants
		{Name: "ld_r_n", MIROp: OpConst, Width: 8, DstLocs: regs8, Template: "LD {dst}, {imm}", Cost: 7, Bytes: 2, Flags: PatImmediate},
		{Name: "ld_rr_nn", MIROp: OpConst, Width: 16, DstLocs: pairs, Template: "LD {dst}, {imm}", Cost: 10, Bytes: 3, Flags: PatImmediate},

		// 8-bit moves
		{Name: "ld_r_r", MIROp: OpMove, Width: 8, DstLocs: regs8, SrcLocs: [2]LocSet{regs8}, Template: "LD {dst}, {src0}", Cost: 4, Bytes: 1},
		// 16-bit moves (no LD rr,rr on Z80 — use PUSH/POP or byte copy)
		{Name: "mov16_push_pop", MIROp: OpMove, Width: 16, DstLocs: pairs, SrcLocs: [2]LocSet{pairs}, Template: "PUSH {src0}\n    POP {dst}", Cost: 21, Bytes: 2},

		// 8-bit ALU — A is implicit destination
		{Name: "add_a_r", MIROp: OpAdd, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, regs8}, Template: "ADD A, {src1}", Cost: 4, Bytes: 1, Clobbers: flags, Flags: PatCommutative},
		{Name: "sub_a_r", MIROp: OpSub, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, regs8}, Template: "SUB {src1}", Cost: 4, Bytes: 1, Clobbers: flags},
		{Name: "and_a_r", MIROp: OpAnd, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, regs8}, Template: "AND {src1}", Cost: 4, Bytes: 1, Clobbers: flags, Flags: PatCommutative},
		{Name: "or_a_r", MIROp: OpOr, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, regs8}, Template: "OR {src1}", Cost: 4, Bytes: 1, Clobbers: flags, Flags: PatCommutative},
		{Name: "xor_a_r", MIROp: OpXor, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, regs8}, Template: "XOR {src1}", Cost: 4, Bytes: 1, Clobbers: flags, Flags: PatCommutative},
		{Name: "cp_r", MIROp: OpCmp, Width: 8, DstLocs: flags, SrcLocs: [2]LocSet{a, regs8}, Template: "CP {src1}", Cost: 4, Bytes: 1, Clobbers: flags},

		// 16-bit ALU — HL is implicit destination
		{Name: "add_hl_rr", MIROp: OpAdd, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{hl, bcde.Or(hl)}, Template: "ADD HL, {src1}", Cost: 11, Bytes: 1, Clobbers: flags},
		{Name: "sbc_hl_rr", MIROp: OpSub, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{hl, bcde.Or(hl)}, Template: "OR A\n    SBC HL, {src1}", Cost: 15, Bytes: 2, Clobbers: flags},

		// Memory — 8-bit via HL pointer
		{Name: "ld_a_hl", MIROp: OpLoad, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{hl}, Template: "LD A, (HL)", Cost: 7, Bytes: 1, Flags: PatMemRead},
		{Name: "ld_hl_a", MIROp: OpStore, Width: 8, SrcLocs: [2]LocSet{hl, a}, Template: "LD (HL), A", Cost: 7, Bytes: 1, Flags: PatMemWrite},
		// Memory — 8-bit via DE pointer
		{Name: "ld_a_de", MIROp: OpLoad, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{m.LocSetByNames("DE")}, Template: "LD A, (DE)", Cost: 7, Bytes: 1, Flags: PatMemRead},

		// Spill to memory (16-bit)
		{Name: "ld_nn_rr", MIROp: OpMove, Width: 16, DstLocs: mem, SrcLocs: [2]LocSet{pairs}, Template: "LD ({dst}), {src0}", Cost: 20, Bytes: 4},
		{Name: "ld_rr_nn_mem", MIROp: OpMove, Width: 16, DstLocs: pairs, SrcLocs: [2]LocSet{mem}, Template: "LD {dst}, ({src0})", Cost: 20, Bytes: 4},

		// INC/DEC shortcuts
		{Name: "inc_r", MIROp: OpAdd, Width: 8, DstLocs: regs8, SrcLocs: [2]LocSet{regs8, allRegs}, Template: "INC {dst}", Cost: 4, Bytes: 1},
		{Name: "inc_rr", MIROp: OpAdd, Width: 16, DstLocs: pairs, SrcLocs: [2]LocSet{pairs, allRegs}, Template: "INC {dst}", Cost: 6, Bytes: 1},

		// Shifts (8-bit: SLA/SRL via A)
		{Name: "sla_a", MIROp: OpShl, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, regs8}, Template: "SLA A", Cost: 8, Bytes: 2, Clobbers: flags},
		{Name: "srl_a", MIROp: OpShr, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, regs8}, Template: "SRL A", Cost: 8, Bytes: 2, Clobbers: flags},

		// Combined 16-bit little-endian load via HL pointer
		// LD L,(HL); INC HL; LD H,(HL) — 3 insts but 1 combined op
		{Name: "ld16_le_hl", MIROp: OpLoad16LE, Width: 16, DstLocs: hl, SrcLocs: [2]LocSet{hl},
			Template: "LD L, (HL)\n    INC HL\n    LD H, (HL)", Cost: 18, Bytes: 3, Flags: PatMemRead},
	}
}

// GenerateMICROPatterns creates 6502-like extremely constrained patterns.
func GenerateMICROPatterns(m *MachineDesc) []Pattern {
	a := m.LocSetByNames("A")
	xy := m.LocSetByNames("X", "Y")
	all := a.Or(xy)
	flags := m.LocSetByNames("F")
	zp := m.LocSetByNames("zp0", "zp1", "zp2", "zp3")

	return []Pattern{
		{Name: "lda_imm", MIROp: OpConst, Width: 8, DstLocs: a, Template: "LDA #{imm}", Cost: 2, Bytes: 2, Flags: PatImmediate},
		{Name: "ldx_imm", MIROp: OpConst, Width: 8, DstLocs: m.LocSetByNames("X"), Template: "LDX #{imm}", Cost: 2, Bytes: 2, Flags: PatImmediate},
		{Name: "ldy_imm", MIROp: OpConst, Width: 8, DstLocs: m.LocSetByNames("Y"), Template: "LDY #{imm}", Cost: 2, Bytes: 2, Flags: PatImmediate},

		// Transfers
		{Name: "tax", MIROp: OpMove, Width: 8, DstLocs: m.LocSetByNames("X"), SrcLocs: [2]LocSet{a}, Template: "TAX", Cost: 2, Bytes: 1},
		{Name: "tay", MIROp: OpMove, Width: 8, DstLocs: m.LocSetByNames("Y"), SrcLocs: [2]LocSet{a}, Template: "TAY", Cost: 2, Bytes: 1},
		{Name: "txa", MIROp: OpMove, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{m.LocSetByNames("X")}, Template: "TXA", Cost: 2, Bytes: 1},
		{Name: "tya", MIROp: OpMove, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{m.LocSetByNames("Y")}, Template: "TYA", Cost: 2, Bytes: 1},

		// ALU — A only
		{Name: "adc", MIROp: OpAdd, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, all.Or(zp)}, Template: "CLC\n    ADC {src1}", Cost: 4, Bytes: 3, Clobbers: flags},
		{Name: "sbc", MIROp: OpSub, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, all.Or(zp)}, Template: "SEC\n    SBC {src1}", Cost: 4, Bytes: 3, Clobbers: flags},
		{Name: "and", MIROp: OpAnd, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, all.Or(zp)}, Template: "AND {src1}", Cost: 2, Bytes: 2, Clobbers: flags},
		{Name: "ora", MIROp: OpOr, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, all.Or(zp)}, Template: "ORA {src1}", Cost: 2, Bytes: 2, Clobbers: flags},
		{Name: "eor", MIROp: OpXor, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, all.Or(zp)}, Template: "EOR {src1}", Cost: 2, Bytes: 2, Clobbers: flags},
		{Name: "cmp", MIROp: OpCmp, Width: 8, DstLocs: flags, SrcLocs: [2]LocSet{a, all.Or(zp)}, Template: "CMP {src1}", Cost: 2, Bytes: 2, Clobbers: flags},

		// Memory — zero-page
		{Name: "sta_zp", MIROp: OpStore, Width: 8, SrcLocs: [2]LocSet{zp, a}, Template: "STA {src0}", Cost: 3, Bytes: 2, Flags: PatMemWrite},
		{Name: "lda_zp", MIROp: OpLoad, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{zp}, Template: "LDA {src0}", Cost: 3, Bytes: 2, Flags: PatMemRead},

		// Shifts
		{Name: "asl", MIROp: OpShl, Width: 8, DstLocs: a, SrcLocs: [2]LocSet{a, all.Or(zp)}, Template: "ASL A", Cost: 2, Bytes: 1, Clobbers: flags},

		// 16-bit load via zero-page pointer (indirect)
		{Name: "lda16_zp", MIROp: OpLoad16LE, Width: 16, DstLocs: a.Or(xy), SrcLocs: [2]LocSet{zp},
			Template: "LDY #0\n    LDA ({src0}),Y\n    TAX\n    INY\n    LDA ({src0}),Y", Cost: 10, Bytes: 8, Flags: PatMemRead},
	}
}

func init() {
	RISC32.Patterns = GenerateRISCPatterns(RISC32)
	RISC8.Patterns = GenerateRISCPatterns(RISC8)
	CISC.Patterns = GenerateCISCPatterns(CISC)
	MICRO.Patterns = GenerateMICROPatterns(MICRO)
}
