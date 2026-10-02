// Package mir2loc adapts MIR2 PhysLoc values to z80spec. Keeping the dependency
// here lets MIR2 import the independent spec later without an import cycle.
// Only zero-offset entries of Z80PhysLocs are represented: dynamic addresses
// and frame offsets require a separate storage/address model.
package mir2loc

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80spec"
)

// From maps Kind and Name together: LocDWord HL is HL32, not HL.
func From(loc mir2.PhysLoc) (z80spec.Location, bool) {
	if loc.Offset != 0 {
		return z80spec.Location{}, false
	}
	name := loc.Name
	if loc.Kind == mir2.LocDWord {
		name += "32"
	}
	canonical, ok := z80spec.Lookup(name)
	if !ok {
		return z80spec.Location{}, false
	}
	back, ok := To(name)
	return canonical, ok && back == loc
}

// To returns a MIR2 location only when this storage has a MIR2 counterpart.
// SP, AF, shadow pairs, F', SMC slots, numbered memory/stack slots and the GPU
// spill sentinel have none. Generic mem/stack retain their sentinel names.
func To(name string) (mir2.PhysLoc, bool) {
	loc, ok := z80spec.Lookup(name)
	if !ok {
		return mir2.PhysLoc{}, false
	}
	var kind mir2.LocKind
	switch loc.Kind {
	case z80spec.GPR8:
		kind = mir2.LocReg
	case z80spec.Pair16:
		if name == "AF" {
			return mir2.PhysLoc{}, false
		}
		kind = mir2.LocReg
	case z80spec.Index:
		if name == "SP" {
			return mir2.PhysLoc{}, false
		}
		kind = mir2.LocIXY
	case z80spec.IndexHalf:
		kind = mir2.LocIXY8
	case z80spec.Flags:
		kind = mir2.LocFlag
	case z80spec.Shadow:
		if loc.Bits != 8 || name == "F'" {
			return mir2.PhysLoc{}, false
		}
		kind = mir2.LocShadow
	case z80spec.MemorySlot:
		if name != "mem" {
			return mir2.PhysLoc{}, false
		}
		kind = mir2.LocMem
	case z80spec.StackSlot:
		if name != "stack" {
			return mir2.PhysLoc{}, false
		}
		kind = mir2.LocStack
	case z80spec.DWordPair:
		kind = mir2.LocDWord
		name = name[:len(name)-2]
	default:
		return mir2.PhysLoc{}, false
	}
	return mir2.PhysLoc{Kind: kind, Name: name}, true
}
