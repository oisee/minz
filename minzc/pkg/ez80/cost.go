package ez80

import "github.com/minz/minzc/pkg/mir2"

// eZ80 timing constants (T-states at 18.432 MHz).
//
// The eZ80 executes most instructions in fewer T-states than the Z80 at 3.5 MHz,
// AND runs at ~5.3× the clock rate.  Combined: ~10-30× throughput for typical code.
// Cost values below are in T-states (abstract, not wall-clock).
const (
	// Register-register move: LD r, r' — 1T on eZ80 (vs 4T Z80)
	ez80RegMove = 1

	// IXY prefix overhead per instruction: DD/FD prefix — ~2T on eZ80 (vs 4T Z80)
	// On eZ80, IX/IY are first-class: prefix decode is pipelined.
	ez80IXYOverhead = 2

	// Stack round-trip: PUSH rr + POP rr — 3 bytes per slot on ADL
	// PUSH ~4T + POP ~4T = ~8T (vs 21T on Z80)
	ez80StackRoundTrip = 8

	// Shadow register swap: EXX — 1T on eZ80 (vs 4T Z80)
	ez80EXX = 1

	// EX AF,AF' — 1T
	ez80EXAF = 1

	// Flag materialise: SCF + SBC A,A — ~4T on eZ80 (vs 8T Z80)
	ez80FlagMat = 4

	// MLT rr — hardware 8×8 multiply — 6T (unique to eZ80)
	ez80MLT = 6

	// LEA rr, IX+d — load effective address — 3T (unique to eZ80)
	ez80LEA = 3

	// IXH/IXL are official registers on eZ80 (undocumented on Z80).
	// Same prefix overhead but guaranteed to work.
	ez80IXY8Cost = ez80RegMove + ez80IXYOverhead
)

// EZ80PhysLocs is the canonical list of physical storage locations on eZ80 ADL.
//
// Key differences from Z80PhysLocs:
//   - IXH/IXL/IYH/IYL promoted to Tier 0.5 (official, not undocumented)
//   - No LocMem ($F0xx) — 24-bit address space, absolute locals impractical
//   - No LocDWord — u24 is native register width, no shadow-pair needed
//   - Stack slot = 3 bytes (not 2)
var EZ80PhysLocs = []mir2.PhysLoc{
	// Tier 0: primary 8-bit
	{Kind: mir2.LocReg, Name: "A"},
	{Kind: mir2.LocReg, Name: "B"},
	{Kind: mir2.LocReg, Name: "C"},
	{Kind: mir2.LocReg, Name: "D"},
	{Kind: mir2.LocReg, Name: "E"},
	{Kind: mir2.LocReg, Name: "H"},
	{Kind: mir2.LocReg, Name: "L"},
	// Tier 0: primary 24-bit pairs
	{Kind: mir2.LocReg, Name: "HL"},
	{Kind: mir2.LocReg, Name: "DE"},
	{Kind: mir2.LocReg, Name: "BC"},
	// Tier 0.5: index 8-bit halves (official on eZ80)
	{Kind: mir2.LocIXY8, Name: "IXH"},
	{Kind: mir2.LocIXY8, Name: "IXL"},
	{Kind: mir2.LocIXY8, Name: "IYH"},
	{Kind: mir2.LocIXY8, Name: "IYL"},
	// Tier 1: index 24-bit
	{Kind: mir2.LocIXY, Name: "IX"},
	{Kind: mir2.LocIXY, Name: "IY"},
	// Tier 2: shadow via EXX
	{Kind: mir2.LocShadow, Name: "B'"},
	{Kind: mir2.LocShadow, Name: "C'"},
	{Kind: mir2.LocShadow, Name: "D'"},
	{Kind: mir2.LocShadow, Name: "E'"},
	{Kind: mir2.LocShadow, Name: "H'"},
	{Kind: mir2.LocShadow, Name: "L'"},
	// Tier 2: shadow accumulator via EX AF,AF'
	{Kind: mir2.LocShadow, Name: "A'"},
	// Tier 3: stack slot (3 bytes per PUSH/POP in ADL)
	{Kind: mir2.LocStack, Name: "stack"},
	// Special: CPU flag register
	{Kind: mir2.LocFlag, Name: "F"},
}

// EZ80CostTable is the CostTable implementation for eZ80 ADL targets.
//
// Compared to Z80CostTable:
//   - All move costs lower (1T vs 4T at register level)
//   - IXY prefix overhead halved (2T vs 4T)
//   - IXH/IXL/IYH/IYL much cheaper (official, no risk of breaking)
//   - Stack spill viable (8T round-trip vs 21T on Z80)
//   - No LocMem — absolute $F0xx addressing doesn't work in 24-bit space
//   - Shadow registers cheaper (EXX = 1T vs 4T)
type EZ80CostTable struct{}

var _ mir2.CostTable = (*EZ80CostTable)(nil)

func (EZ80CostTable) Locs() []mir2.PhysLoc { return EZ80PhysLocs }

func (EZ80CostTable) Cost(cls mir2.RegClass, loc mir2.PhysLoc) int {
	switch cls {
	case mir2.ClassAcc:
		return ez80CostAcc(loc)
	case mir2.ClassCounter:
		return ez80CostCounter(loc)
	case mir2.ClassGeneral:
		return ez80CostGeneral(loc)
	case mir2.ClassPointer:
		return ez80CostPointer(loc)
	case mir2.ClassIndex:
		return ez80CostIndex(loc)
	case mir2.ClassPair:
		return ez80CostPair(loc)
	case mir2.ClassIX:
		return ez80CostIX(loc)
	case mir2.ClassIY:
		return ez80CostIY(loc)
	case mir2.ClassIXY8:
		return ez80CostIXY8(loc)
	case mir2.ClassShadow:
		return ez80CostShadow(loc)
	case mir2.ClassAccShadow:
		return ez80CostAccShadow(loc)
	case mir2.ClassStack:
		return ez80CostStack(loc)
	case mir2.ClassMem:
		return mir2.InfCost // no absolute memory on eZ80 ADL
	case mir2.ClassFlag:
		return ez80CostFlag(loc)
	case mir2.ClassDWord:
		return ez80CostDWord(loc)
	}
	return mir2.InfCost
}

// ── Per-class cost functions ─────────────────────────────────────────────────

func ez80CostAcc(loc mir2.PhysLoc) int {
	switch loc.Kind {
	case mir2.LocReg:
		switch loc.Name {
		case "A":
			return 0
		case "B", "C", "D", "E", "H", "L":
			return ez80RegMove
		case "HL":
			return ez80RegMove
		case "DE":
			return ez80RegMove + 1
		case "BC":
			return ez80RegMove + 2
		}
	case mir2.LocIXY:
		return ez80RegMove + ez80IXYOverhead
	case mir2.LocIXY8:
		return ez80IXY8Cost
	case mir2.LocShadow:
		if loc.Name == "A'" {
			return ez80EXAF * 2 // EX AF,AF' in + out
		}
		return ez80EXX*2 + ez80RegMove
	case mir2.LocStack:
		return ez80StackRoundTrip
	case mir2.LocFlag:
		return mir2.InfCost
	}
	return mir2.InfCost
}

func ez80CostCounter(loc mir2.PhysLoc) int {
	switch loc.Kind {
	case mir2.LocReg:
		switch loc.Name {
		case "B":
			return 0
		case "C", "D", "E", "H", "L", "A":
			return ez80RegMove
		case "HL", "DE", "BC":
			return mir2.InfCost
		}
	case mir2.LocIXY8:
		return ez80IXY8Cost
	case mir2.LocShadow:
		return ez80EXX*2 + ez80RegMove
	case mir2.LocStack:
		return ez80StackRoundTrip
	case mir2.LocFlag:
		return mir2.InfCost
	}
	return mir2.InfCost
}

func ez80CostGeneral(loc mir2.PhysLoc) int {
	switch loc.Kind {
	case mir2.LocReg:
		switch loc.Name {
		case "A", "B":
			return 2 // prefer C/D/E/H/L so ClassAcc/Counter win
		case "C", "D", "E", "H", "L":
			return 0
		case "HL":
			return 2
		case "DE":
			return 2
		case "BC":
			return ez80RegMove + 1
		}
	case mir2.LocIXY:
		return ez80RegMove + ez80IXYOverhead
	case mir2.LocIXY8:
		// eZ80 IXH/IXL are official — viable general registers
		return ez80IXY8Cost
	case mir2.LocShadow:
		return ez80EXX*2 + ez80RegMove
	case mir2.LocStack:
		return ez80StackRoundTrip
	case mir2.LocFlag:
		return mir2.InfCost
	}
	return mir2.InfCost
}

func ez80CostPointer(loc mir2.PhysLoc) int {
	switch loc.Kind {
	case mir2.LocReg:
		switch loc.Name {
		case "HL":
			return 0
		case "DE":
			return ez80RegMove // EX DE,HL
		case "BC":
			return ez80RegMove + 1 // limited: only LD A,(BC)
		case "A", "B", "C", "D", "E", "H", "L":
			return mir2.InfCost // 8-bit can't hold 24-bit address
		}
	case mir2.LocIXY:
		// IX/IY are excellent 24-bit pointers on eZ80 — LEA available
		return ez80LEA // LEA HL, IX+0 costs only 3T
	case mir2.LocIXY8:
		return mir2.InfCost
	case mir2.LocShadow:
		return ez80EXX*2 + ez80RegMove
	case mir2.LocStack:
		return ez80StackRoundTrip
	case mir2.LocFlag:
		return mir2.InfCost
	}
	return mir2.InfCost
}

func ez80CostIndex(loc mir2.PhysLoc) int {
	switch loc.Kind {
	case mir2.LocReg:
		switch loc.Name {
		case "DE":
			return 0
		case "HL":
			return ez80RegMove
		case "BC":
			return ez80RegMove + 1
		case "A", "B", "C", "D", "E", "H", "L":
			return mir2.InfCost
		}
	case mir2.LocIXY:
		return ez80LEA // LEA DE, IX+0
	case mir2.LocIXY8:
		return mir2.InfCost
	case mir2.LocShadow:
		return ez80EXX*2 + ez80RegMove
	case mir2.LocStack:
		return ez80StackRoundTrip
	case mir2.LocFlag:
		return mir2.InfCost
	}
	return mir2.InfCost
}

func ez80CostPair(loc mir2.PhysLoc) int {
	switch loc.Kind {
	case mir2.LocReg:
		switch loc.Name {
		case "HL", "DE", "BC":
			return 0
		default:
			return mir2.InfCost
		}
	case mir2.LocIXY:
		return ez80RegMove + ez80IXYOverhead
	case mir2.LocIXY8:
		return mir2.InfCost
	case mir2.LocShadow:
		return ez80EXX * 2 // EXX in + out
	case mir2.LocStack:
		return ez80StackRoundTrip
	case mir2.LocFlag:
		return mir2.InfCost
	}
	return mir2.InfCost
}

func ez80CostIX(loc mir2.PhysLoc) int {
	switch loc.Kind {
	case mir2.LocIXY:
		switch loc.Name {
		case "IX":
			return 0
		case "IY":
			return 2
		}
	case mir2.LocReg:
		switch loc.Name {
		case "HL":
			return ez80LEA // LEA HL, IX+0
		case "DE":
			return ez80LEA + 1
		case "BC":
			return ez80LEA + 1
		default:
			return mir2.InfCost
		}
	case mir2.LocShadow:
		return ez80EXX*2 + ez80LEA
	case mir2.LocStack:
		return ez80StackRoundTrip
	}
	return mir2.InfCost
}

func ez80CostIY(loc mir2.PhysLoc) int {
	switch loc.Kind {
	case mir2.LocIXY:
		switch loc.Name {
		case "IY":
			return 0
		case "IX":
			return 2
		}
	case mir2.LocReg:
		switch loc.Name {
		case "HL":
			return ez80LEA
		case "DE":
			return ez80LEA + 1
		case "BC":
			return ez80LEA + 1
		default:
			return mir2.InfCost
		}
	case mir2.LocShadow:
		return ez80EXX*2 + ez80LEA
	case mir2.LocStack:
		return ez80StackRoundTrip
	}
	return mir2.InfCost
}

func ez80CostIXY8(loc mir2.PhysLoc) int {
	switch loc.Kind {
	case mir2.LocIXY8:
		return 0
	case mir2.LocReg:
		switch loc.Name {
		case "A", "B", "C", "D", "E", "H", "L":
			return ez80IXY8Cost
		default:
			return mir2.InfCost
		}
	case mir2.LocShadow:
		return ez80EXX*2 + ez80IXY8Cost
	case mir2.LocStack:
		return ez80StackRoundTrip
	}
	return mir2.InfCost
}

func ez80CostShadow(loc mir2.PhysLoc) int {
	switch loc.Kind {
	case mir2.LocShadow:
		return 0
	case mir2.LocReg:
		return ez80EXX*2 + ez80RegMove
	case mir2.LocStack:
		return ez80StackRoundTrip
	}
	return mir2.InfCost
}

func ez80CostAccShadow(loc mir2.PhysLoc) int {
	switch loc.Kind {
	case mir2.LocShadow:
		if loc.Name == "A'" {
			return 0
		}
		return mir2.InfCost
	case mir2.LocReg:
		if loc.Name == "A" {
			return ez80EXAF * 2
		}
		return ez80EXAF*2 + ez80RegMove
	case mir2.LocStack:
		return ez80StackRoundTrip
	}
	return mir2.InfCost
}

func ez80CostStack(loc mir2.PhysLoc) int {
	if loc.Kind == mir2.LocStack {
		return 0
	}
	return mir2.InfCost
}

func ez80CostFlag(loc mir2.PhysLoc) int {
	if loc.Kind == mir2.LocFlag {
		return 0
	}
	// Materialise flag to register: ~4T on eZ80
	if loc.Kind == mir2.LocReg && loc.Name == "A" {
		return ez80FlagMat
	}
	return mir2.InfCost
}

func ez80CostDWord(loc mir2.PhysLoc) int {
	// On eZ80 ADL, 24-bit values fit in a single register pair — no shadow needed.
	// ClassDWord maps to a regular pair.
	switch loc.Kind {
	case mir2.LocReg:
		switch loc.Name {
		case "HL":
			return 0
		case "DE":
			return ez80RegMove
		case "BC":
			return ez80RegMove + 1
		default:
			return mir2.InfCost
		}
	case mir2.LocIXY:
		return ez80LEA
	case mir2.LocShadow:
		return ez80EXX * 2
	case mir2.LocStack:
		return ez80StackRoundTrip
	}
	return mir2.InfCost
}
