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
	OpCall  = 12
	OpRet   = 13
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
		{Name: "and", MIROp: OpAnd, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "and {dst}, {src0}, {src1}", Cost: 1, Bytes: 4, Flags: PatCommutative},
		{Name: "or", MIROp: OpOr, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "or {dst}, {src0}, {src1}", Cost: 1, Bytes: 4, Flags: PatCommutative},
		{Name: "xor", MIROp: OpXor, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "xor {dst}, {src0}, {src1}", Cost: 1, Bytes: 4, Flags: PatCommutative},
		{Name: "cmp", MIROp: OpCmp, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "cmp {dst}, {src0}, {src1}", Cost: 1, Bytes: 4},
		{Name: "load", MIROp: OpLoad, DstLocs: allRegs, SrcLocs: [2]LocSet{allRegs}, Template: "ld {dst}, [{src0}]", Cost: 2, Bytes: 4, Flags: PatMemRead},
		{Name: "store", MIROp: OpStore, SrcLocs: [2]LocSet{allRegs, allRegs}, Template: "st [{src0}], {src1}", Cost: 2, Bytes: 4, Flags: PatMemWrite},
	}
}

func init() {
	RISC32.Patterns = GenerateRISCPatterns(RISC32)
	RISC8.Patterns = GenerateRISCPatterns(RISC8)
}
