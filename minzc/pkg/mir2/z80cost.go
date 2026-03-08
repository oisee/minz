package mir2

// Z80PhysLocs is the canonical list of all physical storage locations on a Z80.
//
// Cost tiers (T-states):
//
//	Tier 0 — primary 8-bit:  A B C D E H L        (0T overhead)
//	Tier 0 — primary 16-bit: HL DE BC              (0T overhead)
//	Tier 1 — index 16-bit:   IX IY                 (+8T: DD/FD prefix per instr)
//	Tier 1 — index 8-bit:    IXH IXL IYH IYL       (+8T: undocumented halves)
//	Tier 2 — shadow 8-bit:   B' C' D' E' H' L'     (+8T: EXX in + EXX out per region)
//	Tier 2 — shadow acc:     A'                    (+4T: EX AF,AF' in + out)
//	Tier 3 — stack slot:     (SP-relative)          (21T: PUSH 11T + POP 10T)
//	Tier 4 — memory slot:    $F0xx                  (26–32T: LD (nn),r + LD r,(nn))
//	Special — CPU flag:      CY/Z flag              (0T: set by cmp/sub naturally)
var Z80PhysLocs = []PhysLoc{
	// ── Tier 0: primary 8-bit ────────────────────────────────────────────────
	{Kind: LocReg, Name: "A"},
	{Kind: LocReg, Name: "B"},
	{Kind: LocReg, Name: "C"},
	{Kind: LocReg, Name: "D"},
	{Kind: LocReg, Name: "E"},
	{Kind: LocReg, Name: "H"},
	{Kind: LocReg, Name: "L"},
	// ── Tier 0: primary 16-bit pairs ─────────────────────────────────────────
	{Kind: LocReg, Name: "HL"},
	{Kind: LocReg, Name: "DE"},
	{Kind: LocReg, Name: "BC"},
	// ── Tier 1: index 16-bit ─────────────────────────────────────────────────
	{Kind: LocIXY, Name: "IX"},
	{Kind: LocIXY, Name: "IY"},
	// ── Tier 1: index 8-bit halves (undocumented) ────────────────────────────
	{Kind: LocIXY8, Name: "IXH"},
	{Kind: LocIXY8, Name: "IXL"},
	{Kind: LocIXY8, Name: "IYH"},
	{Kind: LocIXY8, Name: "IYL"},
	// ── Tier 2: shadow via EXX ───────────────────────────────────────────────
	{Kind: LocShadow, Name: "B'"},
	{Kind: LocShadow, Name: "C'"},
	{Kind: LocShadow, Name: "D'"},
	{Kind: LocShadow, Name: "E'"},
	{Kind: LocShadow, Name: "H'"},
	{Kind: LocShadow, Name: "L'"},
	// ── Tier 2: shadow accumulator via EX AF,AF' ─────────────────────────────
	{Kind: LocShadow, Name: "A'"},
	// ── Tier 3: stack slot (PUSH/POP) ────────────────────────────────────────
	{Kind: LocStack, Name: "stack"},
	// ── Tier 4: absolute memory $F0xx ────────────────────────────────────────
	{Kind: LocMem, Name: "mem"},
	// ── Special: CPU flag register ───────────────────────────────────────────
	{Kind: LocFlag, Name: "F"},
}

// Z80CostTable is a concrete CostTable for Z80 targets.
//
// Cost semantics (abstract units ≈ T-states):
//
//	0        perfect fit (e.g. B for ClassCounter, HL for ClassPointer)
//	2        same-tier alternative with negligible penalty
//	4        same-tier but requires an extra MOV or EX
//	6        wrong tier but usable (e.g. BC as a secondary pointer)
//	8        index-register tier (DD/FD prefix on every instruction)
//	10       EXX shadow region overhead (in + out = ~8T)
//	21       stack PUSH/POP (11T + 10T)
//	28       memory $F0xx (LD (nn),r = 13T + LD r,(nn) = 13T + 2T setup)
//	InfCost  physically impossible for this class
type Z80CostTable struct{}

var _ CostTable = (*Z80CostTable)(nil)

// Locs returns all Z80 physical locations.
func (Z80CostTable) Locs() []PhysLoc { return Z80PhysLocs }

// Cost returns the assignment cost for a virtual register with preferred class cls
// to physical location loc.
func (Z80CostTable) Cost(cls RegClass, loc PhysLoc) int {
	switch cls {
	case ClassAcc:
		return costAcc(loc)
	case ClassCounter:
		return costCounter(loc)
	case ClassGeneral:
		return costGeneral(loc)
	case ClassPointer:
		return costPointer(loc)
	case ClassIndex:
		return costIndex(loc)
	case ClassPair:
		return costPair(loc)
	case ClassIX:
		return costIX(loc)
	case ClassIY:
		return costIY(loc)
	case ClassIXY8:
		return costIXY8(loc)
	case ClassShadow:
		return costShadow(loc)
	case ClassAccShadow:
		return costAccShadow(loc)
	case ClassStack:
		return costStack(loc)
	case ClassMem:
		return costMem(loc)
	case ClassFlag:
		return costFlag(loc)
	}
	return InfCost
}

// ── Per-class cost functions ───────────────────────────────────────────────────

// costAcc: ClassAcc → prefers A (8-bit accumulator).
// Most Z80 ALU ops write to A.  Other 8-bit regs require LD A,r / LD r,A wrapping.
//
// For 16-bit values (u16/ptr) locCompatible already filters out 8-bit locs,
// so the costs below for HL/DE/BC/IX/IY are only reached for 16-bit virtuals.
// InfCost is reserved for truly impossible combos (ClassAcc + CPU flag).
func costAcc(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "A":
			return 0
		case "B", "C", "D", "E":
			return 4 // LD A,r + LD r,A; often fused
		case "H", "L":
			return 4
		// 16-bit: u8 values can't reach here (locCompatible blocks them).
		// For u16 [acc] (e.g. ext u8→u16), HL is best: LD H,0 / LD L,A.
		case "HL":
			return 4
		case "DE":
			return 6
		case "BC":
			return 8
		}
	case LocIXY:
		return 10 // IX/IY: 16-bit, DD/FD prefix overhead; beats stack (21T)
	case LocIXY8:
		return 8 // undocumented halves; prefix overhead per use
	case LocShadow:
		// EX AF,AF' / EXX emission not yet implemented — shadow is physically
		// valid but assigning here generates broken code. InfCost until codegen
		// supports EXX regions. ClassAccShadow is the proper class for A'.
		return InfCost
	case LocStack:
		return 21
	case LocMem:
		return 28
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costCounter: ClassCounter → strongly prefers B (needed for DJNZ).
// Other 8-bit regs usable but require LD B,r before any DJNZ.
func costCounter(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "B":
			return 0
		case "C", "D", "E", "H", "L":
			return 4
		case "A":
			return 4
		case "HL", "DE", "BC":
			return InfCost // 16-bit pairs aren't byte counters
		}
	case LocIXY8:
		return 8
	case LocShadow:
		// EXX emission not implemented — shadow regs are off-limits for all
		// standard classes until codegen supports EXX region detection.
		return InfCost
	case LocStack:
		return 21
	case LocMem:
		return 28
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costGeneral: ClassGeneral → any register works.
//
// A and B are given a slightly higher cost (2) so that ClassAcc and
// ClassCounter always win A and B when there is competition.  C, D, E, H, L
// are the preferred "general" registers (cost 0).
//
// 16-bit values (locCompatible blocks u8→HL etc.) get reasonable pair costs.
func costGeneral(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "A", "B":
			return 2 // usable but prefer C/D/E/H/L so ClassAcc/Counter win
		case "C", "D", "E", "H", "L":
			return 0
		// 16-bit: only reachable for u16+ values (locCompatible blocks u8).
		case "HL":
			return 2 // HL is the best general-purpose 16-bit register
		case "DE":
			return 2
		case "BC":
			return 4
		}
	case LocIXY:
		return 8 // 16-bit index; prefix overhead
	case LocIXY8:
		return 8 // undocumented halves: prefix overhead
	case LocShadow:
		// EXX emission not yet implemented in codegen — shadow registers are
		// physically valid but assigning here would generate broken code.
		// Keep InfCost until EXX region detection is implemented.
		return InfCost
	case LocStack:
		return 21
	case LocMem:
		return 28
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costPointer: ClassPointer → strongly prefers HL (best for LD (HL),r, indirect).
// DE second (good for LDIR dest), BC limited (only LD A,(BC)).
func costPointer(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "HL":
			return 0
		case "DE":
			return 4 // EX DE,HL or use with restrictions
		case "BC":
			return 6 // very limited: only LD A,(BC)/LD (BC),A
		case "A", "B", "C", "D", "E", "H", "L":
			return InfCost // 8-bit registers cannot hold 16-bit addresses
		}
	case LocIXY:
		return 8 // IX/IY+d addressing; DD/FD prefix = +8T per instr
	case LocIXY8:
		return InfCost // 8-bit halves can't be 16-bit pointers
	case LocShadow:
		if loc.Name == "A'" {
			return InfCost
		}
		// HL',DE',BC' via EXX — accessible but expensive region overhead
		if loc.Name == "H'" || loc.Name == "L'" {
			return InfCost // shadow pair, not easily addressable as pointer
		}
		return InfCost
	case LocStack:
		return InfCost // SP-relative addressing requires extra ADD HL,SP
	case LocMem:
		return InfCost // memory-in-memory is circular
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costIndex: ClassIndex → prefers DE (LDIR source), then HL, then BC.
func costIndex(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "DE":
			return 0
		case "HL":
			return 4
		case "BC":
			return 6
		case "A", "B", "C", "D", "E", "H", "L":
			return InfCost
		}
	case LocIXY:
		return 8 // IX/IY usable as source with +d addressing
	case LocIXY8:
		return InfCost
	case LocShadow:
		return InfCost
	case LocStack:
		return InfCost
	case LocMem:
		return InfCost
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costPair: ClassPair → any 16-bit primary pair equally (HL, DE, BC).
func costPair(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "HL", "DE", "BC":
			return 0
		case "A", "B", "C", "D", "E", "H", "L":
			return InfCost
		}
	case LocIXY:
		return 8 // IX/IY are 16-bit but have prefix overhead
	case LocIXY8:
		return InfCost
	case LocShadow:
		return 10 // EXX shadow pairs: accessible as pairs but region overhead
	case LocStack:
		return 21
	case LocMem:
		return 28
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costIX: ClassIX → prefers IX, IY is a near-equal alternative.
func costIX(loc PhysLoc) int {
	switch loc.Kind {
	case LocIXY:
		switch loc.Name {
		case "IX":
			return 0
		case "IY":
			return 2 // same tier; needs LD IY,IX equivalent (or just use IY)
		}
	case LocReg:
		switch loc.Name {
		case "HL":
			return 8 // spill IX → HL: lose +d addressing, 8T per use
		case "DE":
			return 10
		case "BC":
			return 10
		case "A", "B", "C", "D", "E", "H", "L":
			return InfCost
		}
	case LocIXY8:
		return InfCost // halves ≠ full IX
	case LocShadow:
		return InfCost
	case LocStack:
		return 21
	case LocMem:
		return 28
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costIY: ClassIY → prefers IY, IX is a near-equal alternative.
func costIY(loc PhysLoc) int {
	switch loc.Kind {
	case LocIXY:
		switch loc.Name {
		case "IY":
			return 0
		case "IX":
			return 2
		}
	case LocReg:
		switch loc.Name {
		case "HL":
			return 8
		case "DE":
			return 10
		case "BC":
			return 10
		case "A", "B", "C", "D", "E", "H", "L":
			return InfCost
		}
	case LocIXY8:
		return InfCost
	case LocShadow:
		return InfCost
	case LocStack:
		return 21
	case LocMem:
		return 28
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costIXY8: ClassIXY8 → undocumented 8-bit halves IXH/IXL/IYH/IYL.
// Note: H and L cannot be mixed with IXH/IXL (prefixes conflict).
func costIXY8(loc PhysLoc) int {
	switch loc.Kind {
	case LocIXY8:
		return 0 // any of the four halves is perfect
	case LocReg:
		switch loc.Name {
		case "A", "B", "C", "D", "E":
			return 4 // primary 8-bit: needs LD r,IXH etc. (2B/8T)
		case "H", "L":
			return InfCost // H/L conflict with IX prefix
		case "HL", "DE", "BC":
			return InfCost
		}
	case LocShadow:
		return 12
	case LocStack:
		return 21
	case LocMem:
		return 28
	case LocFlag, LocIXY:
		return InfCost
	}
	return InfCost
}

// costShadow: ClassShadow → B',C',D',E',H',L' via EXX.
// Useful for preserving values across a region; each region pays ~8T (EXX + EXX).
func costShadow(loc PhysLoc) int {
	switch loc.Kind {
	case LocShadow:
		if loc.Name == "A'" {
			return InfCost // A' is ClassAccShadow territory
		}
		return 0
	case LocReg:
		switch loc.Name {
		case "B", "C", "D", "E", "H", "L":
			return 8 // primary reg: can store shadow value but wastes a primary reg
		case "A":
			return 8
		case "HL", "DE", "BC":
			return 8
		}
	case LocIXY8:
		return 12
	case LocStack:
		return 21
	case LocMem:
		return 28
	case LocFlag, LocIXY:
		return InfCost
	}
	return InfCost
}

// costAccShadow: ClassAccShadow → A' via EX AF,AF'.
func costAccShadow(loc PhysLoc) int {
	switch loc.Kind {
	case LocShadow:
		if loc.Name == "A'" {
			return 0
		}
		return InfCost // other shadow regs not accessible as A'
	case LocReg:
		if loc.Name == "A" {
			return 4 // EX AF,AF' = 4T each way
		}
		if loc.Name == "B" || loc.Name == "C" || loc.Name == "D" ||
			loc.Name == "E" || loc.Name == "H" || loc.Name == "L" {
			return 8 // needs A as intermediary
		}
		return InfCost
	case LocStack:
		return 21
	case LocMem:
		return 28
	case LocFlag, LocIXY, LocIXY8:
		return InfCost
	}
	return InfCost
}

// costStack: ClassStack → prefers a PUSH/POP slot.
func costStack(loc PhysLoc) int {
	switch loc.Kind {
	case LocStack:
		return 0
	case LocMem:
		return 4 // absolute mem slightly worse than frame-relative
	case LocReg:
		switch loc.Name {
		case "HL", "DE", "BC":
			return 21 // PUSH/POP semantics: 21T round-trip
		case "A", "B", "C", "D", "E", "H", "L":
			return 21 // push AF, pop AF, extract
		}
	case LocShadow:
		return 10
	case LocIXY:
		return 21 // PUSH IX / POP IX = 15T+14T = 29T on NMOS, ~21T on CMOS
	case LocIXY8:
		return InfCost
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costMem: ClassMem → prefers an absolute $F0xx memory slot.
func costMem(loc PhysLoc) int {
	switch loc.Kind {
	case LocMem:
		return 0
	case LocStack:
		return 4 // frame slot: slightly cheaper addressing
	case LocReg:
		switch loc.Name {
		case "A":
			return 26 // LD A,(nn) = 13T, LD (nn),A = 13T
		case "B", "C", "D", "E", "H", "L":
			return 28 // LD A,(nn) + LD r,A = 13T+4T = 17T load overhead
		case "HL", "DE", "BC":
			return 28
		}
	case LocIXY:
		return 28
	case LocIXY8:
		return 28
	case LocShadow:
		return 30
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costFlag: ClassFlag → CPU flag register (CY/Z).
// Materialising a flag into a register is expensive; prefer keeping it as a flag.
func costFlag(loc PhysLoc) int {
	switch loc.Kind {
	case LocFlag:
		return 0
	case LocReg:
		if loc.Name == "A" {
			// Materialise: LD A,0 / JR NC,$+3 / INC A — ~15T; or SCF+SBC A,A — 8T
			return 8
		}
		// Other regs: need A as intermediary
		if loc.Name == "B" || loc.Name == "C" || loc.Name == "D" ||
			loc.Name == "E" || loc.Name == "H" || loc.Name == "L" {
			return 12
		}
		return InfCost
	case LocStack:
		return 21 // PUSH AF / POP AF
	case LocMem:
		return 28
	case LocIXY, LocIXY8, LocShadow:
		return InfCost
	}
	return InfCost
}
