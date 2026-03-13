package mir2

// M6502 Cost Table — PFCCO register allocation costs for MOS 6502 (NMOS).
//
// The 6502 has only 3 CPU registers (A, X, Y), making it the most
// register-constrained popular architecture.  16-bit values must live
// in zero-page pairs.  The 256-byte hardware stack is LIFO-only (no
// SP-relative addressing on NMOS) — prefer register/ZP passing.
//
// ┌─────────────────────────────────────────────────────────────────────┐
// │ Instruction           Bytes  Cycles  Notes                        │
// ├─────────────────────────────────────────────────────────────────────┤
// │ LDA #imm              2      2       immediate                    │
// │ LDA zp                2      3       zero-page                    │
// │ LDA abs               3      4       absolute                     │
// │ LDA (zp),Y            2      5*      indirect-indexed (+1 pgx)    │
// │ LDA (zp,X)            2      6       indexed-indirect             │
// │ STA zp                2      3                                    │
// │ STA abs               3      4                                    │
// │ STA (zp),Y            2      6       always 6 (no +1 trick)       │
// │ TAX/TAY/TXA/TYA       1      2       register transfer            │
// │ PHA                   1      3       push A to stack              │
// │ PLA                   1      4       pull A from stack             │
// │ ADC/SBC #imm          2      2                                    │
// │ ADC/SBC zp            2      3                                    │
// │ ADC/SBC abs           3      4                                    │
// │ CMP #imm              2      2                                    │
// │ CMP zp                2      3                                    │
// │ CPX/CPY #imm          2      2                                    │
// │ CPX/CPY zp            2      3                                    │
// │ INX/INY/DEX/DEY       1      2       register inc/dec             │
// │ INC/DEC zp            2      5       read-modify-write            │
// │ INC/DEC abs           3      6       read-modify-write            │
// │ ASL/LSR/ROL/ROR A     1      2       accumulator shifts           │
// │ ASL/LSR/ROL/ROR zp    2      5       memory shifts (RMW)          │
// │ JSR abs               3      6                                    │
// │ RTS                   1      6                                    │
// │ JMP abs               3      3                                    │
// │ BEQ/BNE/BCC/BCS rel   2      2/3/4  not-taken/taken/taken+pgx    │
// └─────────────────────────────────────────────────────────────────────┘
//
// Key asymmetries:
//   - A is the ONLY register for arithmetic (ADC, SBC, AND, ORA, EOR)
//   - A is the ONLY register for shifts (ASL A, LSR A, ROL A, ROR A)
//   - CMP works on A; CPX/CPY exist but only support #imm, zp, abs
//   - X is for (zp,X) pre-indexed addressing + DEX/INX
//   - Y is for (zp),Y post-indexed addressing + DEY/INY — most useful index
//   - X↔Y requires going through A: TXA;TAY or TYA;TAX (4 cycles, clobbers A)
//   - No DEC A / INC A on NMOS 6502 (65C02 adds these)
//   - Stack is LIFO-only: PHA/PLA, no SP-relative addressing (65C02: none, 65816: yes)
//   - Zero-page ($00-$FF) is the de facto register file for 16-bit values
//   - Only PHA/PLA exist — no PHX/PLX/PHY/PLY on NMOS (65C02 adds these)

// M6502PhysLocs enumerates all physical storage locations on a 6502.
//
// ZP layout: addresses $02-$09 for 8-bit temps, pairs at ($02,$03) ($04,$05)
// ($06,$07) ($08,$09) for 16-bit values.  $00-$01 reserved (C64: I/O direction
// and data port; many systems use $00-$01 for hardware).  Real ZP base address
// is configurable per target; we use symbolic names here.
var M6502PhysLocs = []PhysLoc{
	// ── Tier 0: CPU registers ─────────────────────────────────────────────────
	{Kind: LocReg, Name: "A"},  // accumulator — only ALU/shift operand
	{Kind: LocReg, Name: "X"},  // index X — (zp,X), DEX/INX, TAX/TXA, CPX
	{Kind: LocReg, Name: "Y"},  // index Y — (zp),Y, DEY/INY, TAY/TYA, CPY

	// ── Tier 1: zero-page 8-bit (non-overlapping with pairs) ─────────────────
	// Using $02-$09 to avoid $00-$01 (hardware-reserved on most platforms).
	{Kind: LocMem, Name: "zp2", Offset: 0x02},
	{Kind: LocMem, Name: "zp3", Offset: 0x03},
	{Kind: LocMem, Name: "zp4", Offset: 0x04},
	{Kind: LocMem, Name: "zp5", Offset: 0x05},
	{Kind: LocMem, Name: "zp6", Offset: 0x06},
	{Kind: LocMem, Name: "zp7", Offset: 0x07},
	{Kind: LocMem, Name: "zp8", Offset: 0x08},
	{Kind: LocMem, Name: "zp9", Offset: 0x09},

	// ── Tier 2: zero-page 16-bit pairs ────────────────────────────────────────
	// Each pair aliases two 8-bit slots above — interference must be modeled
	// externally (allocator sees these as separate locations; the codegen
	// must not assign zp2 and zp23 simultaneously).
	// Convention: pair at ($N, $N+1) = (lo, hi).
	{Kind: LocMem, Name: "zp23", Offset: 0x02}, // zp $02-$03
	{Kind: LocMem, Name: "zp45", Offset: 0x04}, // zp $04-$05
	{Kind: LocMem, Name: "zp67", Offset: 0x06}, // zp $06-$07
	{Kind: LocMem, Name: "zp89", Offset: 0x08}, // zp $08-$09

	// ── Tier 3: stack ─────────────────────────────────────────────────────────
	// NMOS 6502 stack: 256 bytes at $0100-$01FF, LIFO-only (PHA/PLA).
	// No SP-relative addressing. Only A can be pushed/pulled (no PHX/PLX).
	{Kind: LocStack, Name: "stack"},

	// ── Tier 4: absolute memory ───────────────────────────────────────────────
	{Kind: LocMem, Name: "abs", Offset: 0x0200},

	// ── Special: processor status flags (C, Z, N, V) ─────────────────────────
	{Kind: LocFlag, Name: "P"},
}

// M6502CostTable implements CostTable for the NMOS MOS 6502.
type M6502CostTable struct{}

var _ CostTable = (*M6502CostTable)(nil)

func (M6502CostTable) Locs() []PhysLoc { return M6502PhysLocs }

func (M6502CostTable) Cost(cls RegClass, loc PhysLoc) int {
	switch cls {
	case ClassAcc:
		return m6502CostAcc(loc)
	case ClassCounter:
		return m6502CostCounter(loc)
	case ClassGeneral:
		return m6502CostGeneral(loc)
	case ClassPointer:
		return m6502CostPointer(loc)
	case ClassIndex:
		return m6502CostIndex(loc)
	case ClassPair:
		return m6502CostPair(loc)
	case ClassFlag:
		return m6502CostFlag(loc)
	case ClassStack:
		return m6502CostStack(loc)
	case ClassMem:
		return m6502CostMem(loc)
	}
	return InfCost
}

// ── 6502 cycle constants ──────────────────────────────────────────────────────
//
// All cycle counts are from the NMOS 6502 data sheet.
// 65C02 timings differ for some instructions (notably INC A, DEC A exist).

const (
	// ── Single instruction costs ──────────────────────────────────────────────
	c6502_Imm      = 2 // LDA #imm, ADC #imm, CMP #imm, etc.
	c6502_Transfer = 2 // TAX, TAY, TXA, TYA — 1 byte, 2 cycles
	c6502_LDA_zp   = 3 // LDA zp — 2 bytes, 3 cycles
	c6502_STA_zp   = 3 // STA zp — 2 bytes, 3 cycles
	c6502_LDA_abs  = 4 // LDA abs — 3 bytes, 4 cycles
	c6502_STA_abs  = 4 // STA abs — 3 bytes, 4 cycles
	c6502_PHA      = 3 // PHA — 1 byte, 3 cycles
	c6502_PLA      = 4 // PLA — 1 byte, 4 cycles
	c6502_INX      = 2 // INX/INY/DEX/DEY — 1 byte, 2 cycles
	c6502_INC_zp   = 5 // INC zp / DEC zp — 2 bytes, 5 cycles (RMW)
	c6502_INC_abs  = 6 // INC abs / DEC abs — 3 bytes, 6 cycles (RMW)
	c6502_LDA_indY = 5 // LDA (zp),Y — 2 bytes, 5 cycles (+1 page cross)
	c6502_STA_indY = 6 // STA (zp),Y — 2 bytes, 6 cycles (always 6)
	c6502_LDA_Xind = 6 // LDA (zp,X) — 2 bytes, 6 cycles

	// ── Compound costs (round-trip or multi-instruction) ──────────────────────
	c6502_StackRT    = 7  // PHA + PLA = 3 + 4 = 7 cycles
	c6502_ZP_RT      = 6  // STA zp + LDA zp = 3 + 3 = 6 cycles
	c6502_Abs_RT     = 8  // STA abs + LDA abs = 4 + 4 = 8 cycles
	c6502_X_to_Y     = 4  // TXA;TAY — must go through A, clobbers A
	c6502_ZP16_RT    = 12 // 2×(STA zp + LDA zp) for 16-bit pair
	c6502_DecA       = 4  // SEC;SBC #1 — no DEC A on NMOS 6502
	c6502_CounterZP  = 10 // DEC zp(5) + LDA zp(3) + BNE(2) per iteration
	c6502_CounterAbs = 13 // DEC abs(6) + LDA abs(4) + BNE(3)
)

// ── Per-class cost functions ──────────────────────────────────────────────────

// ClassAcc: strongly prefers A.  All ALU operations (ADC, SBC, AND, ORA, EOR,
// CMP, ASL A, LSR A, ROL A, ROR A) require the accumulator.
//
// Placing an "accumulator class" value in X or Y means EVERY arithmetic use
// requires TXA/TYA before and TAX/TAY after = 4 cycles per use, not just a
// one-time transfer.  Cost reflects per-use overhead.
func m6502CostAcc(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "A":
			return 0
		case "X":
			return c6502_Transfer * 2 // TXA before + TAX after per ALU use
		case "Y":
			return c6502_Transfer * 2 // TYA before + TAY after per ALU use
		}
	case LocMem:
		if loc.Offset < 0x100 {
			return c6502_ZP_RT // LDA zp before + STA zp after
		}
		return c6502_Abs_RT
	case LocStack:
		return c6502_StackRT
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// ClassCounter: prefers X or Y for DEX/DEY + BNE loop pattern (4 cycles/iter).
// X slightly preferred over Y: more addressing modes available inside the loop.
//
// NMOS 6502 has NO DEC A instruction.  Using A as a counter requires
// SEC;SBC #1 (4 cycles) + BNE (2-3 cycles) = 6-7 cycles/iter vs
// DEX;BNE = 4 cycles/iter.
//
// Zero-page counter: DEC zp (5 cycles) sets Z flag, but DEC doesn't set
// carry, and the result is in memory.  Need LDA zp (3) to branch on it
// or use BNE directly after DEC (DEC zp does set Z): DEC zp + BNE = 7 cycles.
// Wait — DEC zp DOES set Z and N flags!  So: DEC zp(5) + BNE(2) = 7/iter.
func m6502CostCounter(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "X":
			return 0 // DEX(2) + BNE(2) = 4 cycles/iter — optimal
		case "Y":
			return 1 // DEY(2) + BNE(2) = 4 cycles/iter — same speed, fewer addr modes
		case "A":
			// SEC(2) + SBC #1(2) + BNE(2) = 6 cycles/iter.
			// Also clobbers carry flag, complicating surrounding code.
			return c6502_DecA + 2
		}
	case LocMem:
		if loc.Offset < 0x100 {
			// DEC zp(5) + BNE(2) = 7 cycles/iter.  DEC sets Z flag directly.
			return 7
		}
		// DEC abs(6) + BNE(2) = 8 cycles/iter
		return 8
	case LocStack:
		return InfCost // can't decrement stack values without popping
	}
	return InfCost
}

// ClassGeneral: any 8-bit location.  Prefers A (most versatile) > X/Y > ZP > abs.
//
// X and Y are less versatile than A but still useful as they can be loaded,
// stored, compared (CPX/CPY), and incremented/decremented without going
// through A.
func m6502CostGeneral(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "A":
			return 0
		case "X":
			return 1 // CPX exists, but no arithmetic — must go through A for ALU
		case "Y":
			return 1 // CPY exists, same story
		}
	case LocMem:
		if loc.Offset < 0x100 {
			return c6502_LDA_zp // 3 cycles to bring into A for use
		}
		return c6502_LDA_abs // 4 cycles
	case LocStack:
		return c6502_StackRT // PLA(4) to access, PHA(3) to save = 7
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// ClassPointer: 16-bit pointer for memory access.
//
// 6502 indirect addressing modes:
//   - (zp),Y  — post-indexed indirect: ptr in ZP pair, offset in Y
//                LDA (zp),Y = 5 cycles.  THE primary array access mode.
//   - (zp,X)  — pre-indexed indirect: array of pointers in ZP, index in X
//                LDA (zp,X) = 6 cycles.  Rarely useful for general pointers.
//
// Pointer MUST live in a zero-page pair.  No 16-bit CPU registers exist.
// Absolute memory cannot serve as an indirect base on NMOS 6502.
func m6502CostPointer(loc PhysLoc) int {
	switch loc.Kind {
	case LocMem:
		if loc.Offset < 0x100 {
			if isZPPair(loc.Name) {
				return 0 // perfect: LDA (zp),Y uses this directly
			}
			// 8-bit ZP slot can't hold a 16-bit pointer
			return InfCost
		}
		return InfCost // absolute memory can't be indirect base
	case LocReg:
		return InfCost // no 16-bit CPU registers on 6502
	case LocStack:
		// Must PLA twice to get pointer into ZP, then push back.
		// PLA(4) + STA zp(3) + PLA(4) + STA zp+1(3) = 14 cycles just to set up.
		return c6502_StackRT * 2
	}
	return InfCost
}

// ClassIndex: 8-bit index register for array offset.
//
// On 6502, Y is the natural array index: LDA (zp),Y uses Y as the offset.
// X is the secondary choice: LDA abs,X or LDA zp,X for table lookups.
// Zero-page is expensive (must LDY zp before each access).
func m6502CostIndex(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "Y":
			return 0 // LDA (zp),Y — the primary indexed mode
		case "X":
			return 1 // LDA abs,X / LDA zp,X — secondary indexed mode
		case "A":
			// Must TAY or TAX before use as index. Also clobbers A.
			return c6502_Transfer + 1
		}
	case LocMem:
		if loc.Offset < 0x100 {
			return c6502_LDA_zp + c6502_Transfer // LDY zp = 3 cycles to bring to Y
		}
		return c6502_LDA_abs + c6502_Transfer
	case LocStack:
		return InfCost // impractical
	}
	return InfCost
}

// ClassPair: any 16-bit storage.  ZP pairs are the only practical option.
func m6502CostPair(loc PhysLoc) int {
	switch loc.Kind {
	case LocMem:
		if loc.Offset < 0x100 && isZPPair(loc.Name) {
			return 0
		}
		if loc.Offset < 0x100 {
			// Single ZP byte can't hold 16 bits
			return InfCost
		}
		// Absolute pair: LDA abs(4) + LDA abs+1(4) = 8 to read, same to write
		return c6502_Abs_RT * 2
	case LocStack:
		return c6502_StackRT * 2
	case LocReg:
		return InfCost // no 16-bit registers
	}
	return InfCost
}

// ClassFlag: CPU status register flags (C, Z, N, V).
// On 6502 these are in the P register and set implicitly by most instructions.
//
// Flag clobber note: nearly all 6502 instructions modify Z and N flags.
// ADC/SBC additionally modify C and V.  This is modeled via Contract.Clobbers
// (ClassFlag ∈ clobber set for virtually every non-trivial function).
// No special "flag mode tracking" is needed beyond the existing MIR2
// clobber infrastructure — the codegen emits explicit CLC/SEC before
// ADC/SBC to establish the carry input, regardless of prior flag state.
func m6502CostFlag(loc PhysLoc) int {
	if loc.Kind == LocFlag {
		return 0
	}
	return InfCost
}

// ClassStack: value lives on hardware stack.
// On NMOS 6502, only A can be pushed/pulled (PHA/PLA).
// PHP/PLP push/pull the status register, not general data.
func m6502CostStack(loc PhysLoc) int {
	if loc.Kind == LocStack {
		return 0
	}
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "A":
			return c6502_StackRT // PHA(3) + PLA(4) = 7
		case "X":
			// TXA(2) + PHA(3) ... PLA(4) + TAX(2) = 11
			return c6502_Transfer + c6502_StackRT + c6502_Transfer
		case "Y":
			// TYA(2) + PHA(3) ... PLA(4) + TAY(2) = 11
			return c6502_Transfer + c6502_StackRT + c6502_Transfer
		}
	case LocMem:
		if loc.Offset < 0x100 {
			return c6502_ZP_RT
		}
		return c6502_Abs_RT
	}
	return InfCost
}

// ClassMem: value should live in memory (ZP or absolute).
func m6502CostMem(loc PhysLoc) int {
	switch loc.Kind {
	case LocMem:
		if loc.Offset < 0x100 {
			return 0 // zero-page is cheapest memory
		}
		return 2 // absolute: +1 byte, +1 cycle per access
	case LocReg:
		// Keeping a "memory class" value in a register wastes the register
		// and requires STA to spill.  Mild preference for memory.
		return c6502_STA_zp
	case LocStack:
		return c6502_StackRT
	}
	return InfCost
}

// M6502CodegenCostTable wraps M6502CostTable but filters out locations that
// the current 6502 codegen cannot handle (flags "P", stack, absolute memory).
// This forces the allocator to place all values in A/X/Y/ZP where the codegen
// can emit valid instructions.
type M6502CodegenCostTable struct {
	M6502CostTable
	locs []PhysLoc
}

func NewM6502CodegenCostTable() M6502CodegenCostTable {
	var filtered []PhysLoc
	for _, loc := range M6502PhysLocs {
		switch loc.Kind {
		case LocFlag, LocStack:
			continue // codegen can't store to flags or stack
		case LocMem:
			if loc.Offset >= 0x100 {
				continue // codegen can't handle absolute memory
			}
		}
		filtered = append(filtered, loc)
	}
	return M6502CodegenCostTable{locs: filtered}
}

func (ct M6502CodegenCostTable) Locs() []PhysLoc { return ct.locs }

// ── Helpers ──────────────────────────────────────────────────────────────────

func isZPPair(name string) bool {
	switch name {
	case "zp23", "zp45", "zp67", "zp89":
		return true
	}
	return false
}
