package mir2

// M6502 Cost Table — PFCCO register allocation costs for MOS 6502.
//
// The 6502 has only 3 CPU registers (A, X, Y), making it the most
// register-constrained popular architecture. 16-bit values must live
// in zero-page pairs. The 256-byte hardware stack is precious —
// prefer register/ZP passing over stack.
//
// Cost tiers (6502 cycles):
//
//	Tier 0 — CPU registers: A (accumulator), X, Y  (0-2 cycles transfer)
//	Tier 1 — Zero-page byte: ZP $00-$FF             (3 cycles: LDA zp / STA zp)
//	Tier 2 — Zero-page pair: ZP $00-$01, ...        (6 cycles: 2x LDA/STA)
//	Tier 3 — Stack: PHA/PLA                          (6 cycles round-trip)
//	Tier 4 — Absolute memory: $0200+                 (8 cycles round-trip)
//
// Key asymmetries:
//   - A is the ONLY register for arithmetic (ADC, SBC, AND, ORA, EOR, CMP)
//   - X is for (zp,X) indexed and DEX/INX
//   - Y is for (zp),Y indirect-indexed and DEY/INY
//   - X and Y CANNOT be transferred directly — must go through A (TXA;TAY = 4 cycles)
//   - Zero-page is the de facto register file for 16-bit values

// M6502PhysLocs enumerates all physical storage locations on a 6502.
var M6502PhysLocs = []PhysLoc{
	// ── Tier 0: CPU registers ─────────────────────────────────────────────────
	{Kind: LocReg, Name: "A"},  // accumulator — only ALU operand
	{Kind: LocReg, Name: "X"},  // index X — (zp,X), DEX/INX, TAX/TXA
	{Kind: LocReg, Name: "Y"},  // index Y — (zp),Y, DEY/INY, TAY/TYA
	// ── Tier 1: zero-page 8-bit ───────────────────────────────────────────────
	{Kind: LocMem, Name: "zp0", Offset: 0x00},
	{Kind: LocMem, Name: "zp1", Offset: 0x01},
	{Kind: LocMem, Name: "zp2", Offset: 0x02},
	{Kind: LocMem, Name: "zp3", Offset: 0x03},
	{Kind: LocMem, Name: "zp4", Offset: 0x04},
	{Kind: LocMem, Name: "zp5", Offset: 0x05},
	{Kind: LocMem, Name: "zp6", Offset: 0x06},
	{Kind: LocMem, Name: "zp7", Offset: 0x07},
	// ── Tier 2: zero-page 16-bit pairs ────────────────────────────────────────
	// Convention: pair N uses zp[2N], zp[2N+1] (lo, hi)
	{Kind: LocMem, Name: "zp01", Offset: 0x00},  // zp $00-$01
	{Kind: LocMem, Name: "zp23", Offset: 0x02},   // zp $02-$03
	{Kind: LocMem, Name: "zp45", Offset: 0x04},   // zp $04-$05
	{Kind: LocMem, Name: "zp67", Offset: 0x06},   // zp $06-$07
	// ── Tier 3: stack ─────────────────────────────────────────────────────────
	{Kind: LocStack, Name: "stack"},
	// ── Tier 4: absolute memory ───────────────────────────────────────────────
	{Kind: LocMem, Name: "abs", Offset: 0x0200},
	// ── Special: flags (C, Z, N, V) ──────────────────────────────────────────
	{Kind: LocFlag, Name: "P"},
}

// M6502CostTable implements CostTable for the MOS 6502.
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

const (
	c6502_Transfer = 2  // TAX, TAY, TXA, TYA — 2 cycles each
	c6502_LDA_zp   = 3  // LDA zp — 3 cycles
	c6502_STA_zp   = 3  // STA zp — 3 cycles
	c6502_LDA_abs  = 4  // LDA abs — 4 cycles
	c6502_STA_abs  = 4  // STA abs — 4 cycles
	c6502_PHA      = 3  // PHA — 3 cycles
	c6502_PLA      = 4  // PLA — 4 cycles
	c6502_StackRT  = 7  // PHA + PLA = 3 + 4 = 7 cycles round-trip
	c6502_ZP_RT    = 6  // STA zp + LDA zp = 3 + 3 = 6 cycles round-trip
	c6502_Abs_RT   = 8  // STA abs + LDA abs = 4 + 4 = 8 cycles round-trip
	c6502_X_to_Y   = 4  // TXA; TAY — must go through A
	c6502_ZP16_RT  = 12 // 2× (STA zp + LDA zp) for 16-bit pair
)

// ── Per-class cost functions ──────────────────────────────────────────────────

// ClassAcc: strongly prefers A. All ALU operations (ADC, SBC, AND, ORA, EOR,
// CMP, ASL A, LSR A, ROL A, ROR A) require the accumulator.
func m6502CostAcc(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "A":
			return 0
		case "X":
			return c6502_Transfer // TAX/TXA
		case "Y":
			return c6502_Transfer // TAY/TYA
		}
	case LocMem:
		if loc.Offset < 0x100 {
			return c6502_ZP_RT // zero-page: STA zp / LDA zp
		}
		return c6502_Abs_RT // absolute
	case LocStack:
		return c6502_StackRT
	case LocFlag:
		return InfCost // can't hold a value in flags
	}
	return InfCost
}

// ClassCounter: prefers X (DEX + BNE loop pattern) or Y (DEY + BNE).
// 6502 has no DJNZ equivalent; the common idiom is DEX/DEY + BNE.
// X is slightly preferred over Y (more addressing modes).
func m6502CostCounter(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "X":
			return 0 // DEX + BNE is the standard loop pattern
		case "Y":
			return 1 // DEY + BNE also works, slightly less versatile
		case "A":
			return c6502_Transfer + 2 // must transfer to X/Y for loop
		}
	case LocMem:
		if loc.Offset < 0x100 {
			return c6502_ZP_RT + 2 // DEC zp + LDA zp + BNE
		}
		return c6502_Abs_RT + 4
	case LocStack:
		return c6502_StackRT + 4
	}
	return InfCost
}

// ClassGeneral: any 8-bit location. Prefers A > X/Y > ZP.
func m6502CostGeneral(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "A":
			return 0
		case "X", "Y":
			return 1 // slightly less versatile
		}
	case LocMem:
		if loc.Offset < 0x100 {
			return c6502_LDA_zp // zero-page = cheap memory
		}
		return c6502_LDA_abs
	case LocStack:
		return c6502_StackRT
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// ClassPointer: 16-bit pointer for memory access. Must be in zero-page pair
// because 6502 indirect addressing uses (zp),Y or (zp,X).
func m6502CostPointer(loc PhysLoc) int {
	switch loc.Kind {
	case LocMem:
		if loc.Offset < 0x100 {
			// ZP pair — usable for (zp),Y indirect-indexed addressing
			if isZPPair(loc.Name) {
				return 0 // perfect: LDA (zp),Y
			}
			return c6502_ZP16_RT // 8-bit ZP slot — can't hold 16-bit
		}
		return InfCost // absolute memory can't be an indirect base
	case LocReg:
		return InfCost // no 16-bit CPU registers on 6502
	case LocStack:
		return c6502_StackRT * 2 // 2 bytes pushed/pulled
	}
	return InfCost
}

// ClassIndex: secondary pointer/index. On 6502, also a ZP pair.
func m6502CostIndex(loc PhysLoc) int {
	switch loc.Kind {
	case LocMem:
		if loc.Offset < 0x100 && isZPPair(loc.Name) {
			return 2 // usable but slightly less preferred than ClassPointer
		}
		if loc.Offset < 0x100 {
			return c6502_ZP16_RT
		}
		return InfCost
	case LocReg:
		return InfCost
	case LocStack:
		return c6502_StackRT * 2
	}
	return InfCost
}

// ClassPair: any 16-bit storage. ZP pairs are the only option.
func m6502CostPair(loc PhysLoc) int {
	switch loc.Kind {
	case LocMem:
		if loc.Offset < 0x100 && isZPPair(loc.Name) {
			return 0 // zero-page pair is the native 16-bit location
		}
		if loc.Offset < 0x100 {
			return c6502_ZP16_RT
		}
		return c6502_Abs_RT * 2
	case LocStack:
		return c6502_StackRT * 2
	}
	return InfCost
}

// ClassFlag: CPU status register flags (C, Z, N, V).
func m6502CostFlag(loc PhysLoc) int {
	if loc.Kind == LocFlag {
		return 0
	}
	return InfCost
}

// ClassStack: value lives on hardware stack.
func m6502CostStack(loc PhysLoc) int {
	if loc.Kind == LocStack {
		return 0
	}
	switch loc.Kind {
	case LocReg:
		return c6502_StackRT // PHA/PLA
	case LocMem:
		if loc.Offset < 0x100 {
			return c6502_ZP_RT
		}
		return c6502_Abs_RT
	}
	return InfCost
}

// ClassMem: value in memory (ZP or absolute).
func m6502CostMem(loc PhysLoc) int {
	switch loc.Kind {
	case LocMem:
		if loc.Offset < 0x100 {
			return 0 // zero-page is cheapest memory
		}
		return 2 // absolute: 1 extra cycle per access
	case LocReg:
		return c6502_LDA_zp // must load to register first
	case LocStack:
		return c6502_StackRT
	}
	return InfCost
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func isZPPair(name string) bool {
	switch name {
	case "zp01", "zp23", "zp45", "zp67":
		return true
	}
	return false
}
