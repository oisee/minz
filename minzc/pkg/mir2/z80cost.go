package mir2

import "github.com/minz/minzc/pkg/z80timing"

// Z80PhysLocs is the canonical list of all physical storage locations on a Z80.
//
// Cost tiers (T-states from pkg/z80timing):
//
//	Tier 0 — primary 8-bit:  A B C D E H L        (0T overhead)
//	Tier 0 — primary 16-bit: HL DE BC              (0T overhead)
//	Tier 1 — index 16-bit:   IX IY                 (+4T: DD/FD prefix per instr)
//	Tier 1 — index 8-bit:    IXH IXL IYH IYL       (+4T: undocumented halves)
//	Tier 2 — shadow via EXX: B' C' D' E' H' L'     (+8T: EXX in + EXX out per region)
//	Tier 2 — shadow acc:     A'                    (+8T: EX AF,AF' in + out)
//	Tier 3 — stack slot:     (SP-relative)          (z80timing.StackRoundTrip = 21T)
//	Tier 4 — memory slot:    $F0xx                  (z80timing.MemRoundTrip8 = 26T)
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
	// ── 32-bit: shadow pair (main rr + shadow rr' via EXX) ───────────────────
	{Kind: LocDWord, Name: "HL"}, // HL (lo) + H'L' (hi) via EXX
	{Kind: LocDWord, Name: "DE"}, // DE (lo) + D'E' (hi) via EXX
	{Kind: LocDWord, Name: "BC"}, // BC (lo) + B'C' (hi) via EXX
}

// Z80CostTable is a concrete CostTable for Z80 targets.
//
// All costs are in T-states as defined in pkg/z80timing.
// Key derived constants:
//
//	Reg-reg move:      z80timing.RegRegMove      =  4T  (LD r, r')
//	Immediate load:    z80timing.LD_r_n           =  7T  (LD r, n)
//	Stack round-trip:  z80timing.StackRoundTrip   = 21T  (PUSH+POP)
//	Memory round-trip: z80timing.MemRoundTrip8    = 26T  (LD (nn),A + LD A,(nn))
//	IXY overhead:      z80timing.IXY_OVERHEAD     =  4T  (DD/FD prefix per instr)
//	Flag materialise:  z80timing.FlagMaterialise8 =  8T  (SCF+SBC A,A)
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
	case ClassRegC:
		return costRegC(loc)
	case ClassRegD:
		return costHardPin(loc, "D")
	case ClassRegE:
		return costHardPin(loc, "E")
	case ClassRegH:
		return costHardPin(loc, "H")
	case ClassRegL:
		return costHardPin(loc, "L")
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
	case ClassDWord:
		return costDWord(loc)
	}
	return InfCost
}

// ── Per-class cost functions ───────────────────────────────────────────────────

// costAcc: ClassAcc → prefers A (8-bit accumulator).
// Most Z80 ALU ops write to A.  Other 8-bit regs require LD A,r / LD r,A wrapping.
func costAcc(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "A":
			return 0
		case "B", "C", "D", "E", "H", "L":
			// LD A,r (4T) + LD r,A (4T) = 8T per round-trip,
			// but in practice only one direction matters → cost 4.
			return z80timing.RegRegMove
		// 16-bit: u8 values can't reach here (locCompatible blocks them).
		// For u16 [acc] (e.g. ext u8→u16), HL is best: LD H,0 / LD L,A.
		case "HL":
			return z80timing.RegRegMove
		case "DE":
			return z80timing.RegRegMove + 2
		case "BC":
			return z80timing.RegRegMove + 4
		}
	case LocIXY:
		// IX/IY: 16-bit, DD/FD prefix overhead; beats stack.
		return z80timing.RegRegMove + z80timing.IXY_OVERHEAD + 2
	case LocIXY8:
		// Undocumented halves; prefix overhead per use.
		return z80timing.RegRegMove + z80timing.IXY_OVERHEAD
	case LocShadow:
		// EX AF,AF' / EXX not yet implemented in codegen.
		return InfCost
	case LocStack:
		// Stack spills (PUSH/POP) are not implemented in codegen — would emit
		// "AND stack" etc.  Use InfCost until codegen supports SP-relative access.
		return InfCost
	case LocMem:
		return z80timing.MemRoundTrip8 + 2
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
		case "C", "D", "E", "H", "L", "A":
			return z80timing.RegRegMove
		case "HL", "DE", "BC":
			return InfCost // 16-bit pairs aren't byte counters
		}
	case LocIXY8:
		return z80timing.RegRegMove + z80timing.IXY_OVERHEAD
	case LocShadow:
		// EXX not implemented.
		return InfCost
	case LocStack:
		// Stack spills (PUSH/POP) are not implemented in codegen — would emit
		// "AND stack" etc.  Use InfCost until codegen supports SP-relative access.
		return InfCost
	case LocMem:
		return z80timing.MemRoundTrip8 + 2
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costHardPin returns 0 for exactly one named register, InfCost for everything else.
// Used by @z80_X annotations to hard-pin a parameter to a specific physical register.
func costHardPin(loc PhysLoc, name string) int {
	if loc.Kind == LocReg && loc.Name == name {
		return 0
	}
	return InfCost
}

// costRegC: ClassRegC → hard pin to C register (for @z80_c inline asm).
func costRegC(loc PhysLoc) int { return costHardPin(loc, "C") }

// costGeneral: ClassGeneral → any register works.
//
// A and B are given a slightly higher cost (2) so that ClassAcc and
// ClassCounter always win A and B when there is competition.  C, D, E, H, L
// are the preferred "general" registers (cost 0).
func costGeneral(loc PhysLoc) int {
	switch loc.Kind {
	case LocReg:
		switch loc.Name {
		case "A", "B":
			// Usable but prefer C/D/E/H/L so ClassAcc/Counter win.
			return 2
		case "C", "D", "E", "H", "L":
			return 0
		// 16-bit: only reachable for u16+ values (locCompatible blocks u8).
		case "HL":
			return 2
		case "DE":
			return 2
		case "BC":
			return z80timing.RegRegMove
		}
	case LocIXY:
		return z80timing.RegRegMove + z80timing.IXY_OVERHEAD
	case LocIXY8:
		return z80timing.RegRegMove + z80timing.IXY_OVERHEAD
	case LocShadow:
		// EXX not implemented.
		return InfCost
	case LocStack:
		// Stack spills (PUSH/POP) are not implemented in codegen — would emit
		// "AND stack" etc.  Use InfCost until codegen supports SP-relative access.
		return InfCost
	case LocMem:
		return z80timing.MemRoundTrip8 + 2
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
			return z80timing.RegRegMove // EX DE,HL or use with restrictions
		case "BC":
			return z80timing.RegRegMove + 2 // very limited: only LD A,(BC)/LD (BC),A
		case "A", "B", "C", "D", "E", "H", "L":
			return InfCost // 8-bit registers cannot hold 16-bit addresses
		}
	case LocIXY:
		// IX/IY+d addressing; DD/FD prefix = IXY_OVERHEAD per instr
		return z80timing.RegRegMove + z80timing.IXY_OVERHEAD
	case LocIXY8:
		return InfCost // 8-bit halves can't be 16-bit pointers
	case LocShadow:
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
			return z80timing.RegRegMove
		case "BC":
			return z80timing.RegRegMove + 2
		case "A", "B", "C", "D", "E", "H", "L":
			return InfCost
		}
	case LocIXY:
		return z80timing.RegRegMove + z80timing.IXY_OVERHEAD
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
		return z80timing.RegRegMove + z80timing.IXY_OVERHEAD // IX/IY are 16-bit but have prefix overhead
	case LocIXY8:
		return InfCost
	case LocShadow:
		// HL',DE',BC' via EXX — region overhead ~8T per region
		return z80timing.EXX * 2
	case LocStack:
		// Stack spills (PUSH/POP) are not implemented in codegen — would emit
		// "AND stack" etc.  Use InfCost until codegen supports SP-relative access.
		return InfCost
	case LocMem:
		return z80timing.MemRoundTrip8 + 2
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
			return z80timing.IXY_OVERHEAD * 2 // spill IX → HL: lose +d addressing
		case "DE":
			return z80timing.IXY_OVERHEAD*2 + 2
		case "BC":
			return z80timing.IXY_OVERHEAD*2 + 2
		case "A", "B", "C", "D", "E", "H", "L":
			return InfCost
		}
	case LocIXY8:
		return InfCost // halves ≠ full IX
	case LocShadow:
		return InfCost
	case LocStack:
		// Stack spills (PUSH/POP) are not implemented in codegen — would emit
		// "AND stack" etc.  Use InfCost until codegen supports SP-relative access.
		return InfCost
	case LocMem:
		return z80timing.MemRoundTrip8 + 2
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
			return z80timing.IXY_OVERHEAD * 2
		case "DE":
			return z80timing.IXY_OVERHEAD*2 + 2
		case "BC":
			return z80timing.IXY_OVERHEAD*2 + 2
		case "A", "B", "C", "D", "E", "H", "L":
			return InfCost
		}
	case LocIXY8:
		return InfCost
	case LocShadow:
		return InfCost
	case LocStack:
		// Stack spills (PUSH/POP) are not implemented in codegen — would emit
		// "AND stack" etc.  Use InfCost until codegen supports SP-relative access.
		return InfCost
	case LocMem:
		return z80timing.MemRoundTrip8 + 2
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
			// LD r, IXH etc. (2B/8T) — one direction only
			return z80timing.RegRegMove + z80timing.IXY_OVERHEAD
		case "H", "L":
			return InfCost // H/L conflict with IX prefix
		case "HL", "DE", "BC":
			return InfCost
		}
	case LocShadow:
		return z80timing.EXX + z80timing.RegRegMove
	case LocStack:
		// Stack spills (PUSH/POP) are not implemented in codegen — would emit
		// "AND stack" etc.  Use InfCost until codegen supports SP-relative access.
		return InfCost
	case LocMem:
		return z80timing.MemRoundTrip8 + 2
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
		case "B", "C", "D", "E", "H", "L", "A":
			return z80timing.EXX * 2 // primary reg: region overhead
		case "HL", "DE", "BC":
			return z80timing.EXX * 2
		}
	case LocIXY8:
		return z80timing.EXX + z80timing.RegRegMove
	case LocStack:
		// Stack spills (PUSH/POP) are not implemented in codegen — would emit
		// "AND stack" etc.  Use InfCost until codegen supports SP-relative access.
		return InfCost
	case LocMem:
		return z80timing.MemRoundTrip8 + 2
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
			return z80timing.EX_AF_AF // single exchange = 4T
		}
		if loc.Name == "B" || loc.Name == "C" || loc.Name == "D" ||
			loc.Name == "E" || loc.Name == "H" || loc.Name == "L" {
			return z80timing.EX_AF_AF + z80timing.RegRegMove // needs A as intermediary
		}
		return InfCost
	case LocStack:
		// Stack spills (PUSH/POP) are not implemented in codegen — would emit
		// "AND stack" etc.  Use InfCost until codegen supports SP-relative access.
		return InfCost
	case LocMem:
		return z80timing.MemRoundTrip8 + 2
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
			return z80timing.StackRoundTrip
		case "A", "B", "C", "D", "E", "H", "L":
			return z80timing.StackRoundTrip // push AF, pop AF, extract
		}
	case LocShadow:
		return z80timing.EXX * 2
	case LocIXY:
		return z80timing.PUSH_IX + z80timing.POP_IX // 15T+14T = 29T
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
			return z80timing.MemRoundTrip8 // LD (nn),A + LD A,(nn) = 26T
		case "B", "C", "D", "E", "H", "L":
			// LD A,(nn) (13T) + LD r,A (4T) = 17T load; store same as A
			return z80timing.MemRoundTrip8 + 2
		case "HL", "DE", "BC":
			return z80timing.MemRoundTrip8 + 2
		}
	case LocIXY:
		return z80timing.MemRoundTrip8 + 2
	case LocIXY8:
		return z80timing.MemRoundTrip8 + 2
	case LocShadow:
		return z80timing.MemRoundTrip8 + 4
	case LocFlag:
		return InfCost
	}
	return InfCost
}

// costDWord: ClassDWord → 32-bit shadow pair (HL+H'L', DE+D'E', BC+B'C').
//
// Each 32-bit operation costs ~8T in EXX overhead (two EXX instructions ×4T each).
// HL is strongly preferred: Z80 has ADD HL,rr and ADC HL,rr for 32-bit add/sub.
// DE and BC require temporary HL use for arithmetic, adding extra overhead.
//
// Cost convention: base cost excludes the per-operation EXX overhead, which is
// captured implicitly in the codegen.  HL = 0 ensures the allocator assigns
// arithmetic results to HL first; DE = 4, BC = 6 as fallbacks for non-arithmetic
// values (e.g. loop invariants, function parameters).
func costDWord(loc PhysLoc) int {
	switch loc.Kind {
	case LocDWord:
		switch loc.Name {
		case "HL":
			return 0 // preferred: ADD HL,rr / ADC HL,rr available natively
		case "DE":
			return 4 // usable; arithmetic requires routing via HL
		case "BC":
			return 6 // least preferred; same limitation as DE
		}
	case LocMem:
		// Spill: 4 bytes × LD (nn),r / LD r,(nn) = 4 × 26T = 104T
		return z80timing.MemRoundTrip8 * 4
	case LocStack:
		// Two PUSH+POP pairs (hi + lo) = 2 × 21T = 42T
		return z80timing.StackRoundTrip * 2
	}
	return InfCost
}

// costFlag: ClassFlag → CPU flag register (CY/Z).
// A flag value is already set "for free" as a side-effect of CMP/SUB/ADD etc.
// Materialising it into a register costs ~8T (SCF+SBC A,A pattern).
func costFlag(loc PhysLoc) int {
	switch loc.Kind {
	case LocFlag:
		return 0
	case LocReg:
		if loc.Name == "A" {
			// Materialise via SCF+SBC A,A: 4T+4T = 8T
			return z80timing.FlagMaterialise8
		}
		if loc.Name == "B" || loc.Name == "C" || loc.Name == "D" ||
			loc.Name == "E" || loc.Name == "H" || loc.Name == "L" {
			// Materialise to A (8T) + LD r,A (4T) = 12T
			return z80timing.FlagMaterialise8 + z80timing.RegRegMove
		}
		return InfCost
	case LocStack:
		// Stack spills not implemented in codegen — would emit "AND stack".
		return InfCost
	case LocMem:
		return z80timing.MemRoundTrip8 + 2
	case LocIXY, LocIXY8, LocShadow:
		return InfCost
	}
	return InfCost
}
