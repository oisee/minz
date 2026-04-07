// enriched_index.go — Enumeration index computation for Z80T binary tables.
//
// Vendored from github.com/oisee/z80-optimizer/pkg/regalloc/index.go
package vir

import "fmt"

// Loc set definitions — must match regalloc-enum exactly.
var locSets8 = [][]int{
	{0},                   // 0: must be A
	{2},                   // 1: must be C
	{0, 1, 2, 3, 4, 5, 6}, // 2: any GPR8
	{1, 2, 3, 4, 5, 6},    // 3: any GPR8 except A
}

var locSets8IX = [][]int{
	{0},                             // 0: must be A
	{2},                             // 1: must be C
	{0, 1, 2, 3, 4, 5, 6},           // 2: any GPR8
	{1, 2, 3, 4, 5, 6},              // 3: any GPR8 except A
	{10, 11, 12, 13},                // 4: must be IX/IY half
	{0, 1, 2, 3, 4, 10, 11, 12, 13}, // 5: any 8-bit except H/L
}

var locSets16 = [][]int{
	{9},       // 0: must be HL
	{8},       // 1: must be DE
	{7, 8, 9}, // 2: any pair
}

// Shape describes a register allocation constraint shape for table lookup.
type EnrichedShape struct {
	NVregs       int    // number of virtual registers (2-6)
	Widths       []int  // per-vreg width: 8 or 16
	LocSetIndex  []int  // per-vreg index into locSets8 (0-3) or locSets16 (0-2)
	Interference uint32 // bitmask of interference edges (bit K = edge K present)
}

// LocSetIndexFor returns the loc set index for a given set of allowed locations.
// Returns -1 if the loc set doesn't match any known pattern.
func LocSetIndexFor(width int, locs []int) int {
	var sets [][]int
	if width == 16 {
		sets = locSets16
	} else {
		sets = locSets8IX
	}
	for i, s := range sets {
		if len(s) != len(locs) {
			continue
		}
		match := true
		for j := range s {
			if s[j] != locs[j] {
				match = false
				break
			}
		}
		if match {
			return i
		}
	}
	return -1
}

// IndexOf computes the enumeration index for a shape in the binary table.
// The index corresponds to the position in the file produced by regalloc-enum.
//
// Enumeration order (nested loops, innermost varies fastest):
//
//	for nVregs in 2..maxVregs:
//	  for widthCombo in 0..2^nVregs:
//	    for locSetCombo in 0..product(locSetCounts):
//	      for interferenceGraph in 0..2^(nVregs*(nVregs-1)/2):
//	        → shape at this index
//
// Returns -1 if the shape is invalid or doesn't match known loc sets.
func EnrichedIndexOf(s EnrichedShape, maxVregs int) (int, error) {
	return EnrichedIndexOfWithLocSets(s, maxVregs, len(locSets8), len(locSets16))
}

// EnrichedIndexOfWithLocSets computes the enumeration index for a table that
// may use an expanded 8-bit loc-set space (for example Z80T v2 with IX halves).
func EnrichedIndexOfWithLocSets(s EnrichedShape, maxVregs, nLocSets8, nLocSets16 int) (int, error) {
	if s.NVregs < 2 || s.NVregs > maxVregs || s.NVregs > 6 {
		return -1, fmt.Errorf("nVregs %d out of range [2, %d]", s.NVregs, maxVregs)
	}
	if len(s.Widths) != s.NVregs || len(s.LocSetIndex) != s.NVregs {
		return -1, fmt.Errorf("widths/locsets length mismatch: %d/%d vs nVregs=%d",
			len(s.Widths), len(s.LocSetIndex), s.NVregs)
	}

	index := 0

	// Add offset for all smaller nVregs values
	for nv := 2; nv < s.NVregs; nv++ {
		index += countShapesWithLocSets(nv, nLocSets8, nLocSets16)
	}

	// Width combo: bit i = vreg i is 16-bit
	widthCombo := 0
	for i := 0; i < s.NVregs; i++ {
		if s.Widths[i] == 16 {
			widthCombo |= 1 << i
		}
	}

	// Offset within this nVregs: enumerate width combos before ours
	nEdges := s.NVregs * (s.NVregs - 1) / 2
	nIntfGraphs := 1 << nEdges

	for wc := 0; wc < widthCombo; wc++ {
		nLocCombos := locComboCountWithLocSets(s.NVregs, wc, nLocSets8, nLocSets16)
		index += nLocCombos * nIntfGraphs
	}

	// Offset within this width combo: enumerate loc combos before ours
	locCombo := encodeLocComboWithLocSets(s.NVregs, widthCombo, s.LocSetIndex, nLocSets8, nLocSets16)
	if locCombo < 0 {
		return -1, fmt.Errorf("invalid loc set index")
	}
	index += locCombo * nIntfGraphs

	// Offset within this loc combo: interference bitmask is the final index
	index += int(s.Interference)

	return index, nil
}

// countShapes returns the total number of shapes for a given nVregs.
func countShapes(nv int) int {
	return countShapesWithLocSets(nv, len(locSets8), len(locSets16))
}

func countShapesWithLocSets(nv, nLocSets8, nLocSets16 int) int {
	nEdges := nv * (nv - 1) / 2
	nIntfGraphs := 1 << nEdges
	total := 0
	nWidthCombos := 1 << nv
	for wc := 0; wc < nWidthCombos; wc++ {
		total += locComboCountWithLocSets(nv, wc, nLocSets8, nLocSets16) * nIntfGraphs
	}
	return total
}

// locComboCount returns how many loc set combinations exist for a given width combo.
func locComboCount(nv, widthCombo int) int {
	return locComboCountWithLocSets(nv, widthCombo, len(locSets8), len(locSets16))
}

func locComboCountWithLocSets(nv, widthCombo, nLocSets8, nLocSets16 int) int {
	count := 1
	for i := 0; i < nv; i++ {
		if widthCombo&(1<<i) != 0 {
			count *= nLocSets16
		} else {
			count *= nLocSets8
		}
	}
	return count
}

// encodeLocCombo encodes per-vreg loc set indices into a single combo index.
// Uses mixed-radix encoding: vreg[nv-1] varies fastest.
func encodeLocCombo(nv, widthCombo int, locSetIdx []int) int {
	return encodeLocComboWithLocSets(nv, widthCombo, locSetIdx, len(locSets8), len(locSets16))
}

func encodeLocComboWithLocSets(nv, widthCombo int, locSetIdx []int, nLocSets8, nLocSets16 int) int {
	combo := 0
	for i := 0; i < nv; i++ {
		var nSets int
		if widthCombo&(1<<i) != 0 {
			nSets = nLocSets16
		} else {
			nSets = nLocSets8
		}
		if locSetIdx[i] < 0 || locSetIdx[i] >= nSets {
			return -1
		}
		// Multiply by remaining dimensions
		mul := 1
		for j := i + 1; j < nv; j++ {
			if widthCombo&(1<<j) != 0 {
				mul *= nLocSets16
			} else {
				mul *= nLocSets8
			}
		}
		combo += locSetIdx[i] * mul
	}
	return combo
}

// InterferenceBitmask converts a list of edge pairs to the bitmask format.
// Edge (i,j) where i<j maps to bit index: sum_{a=0}^{i-1}(nv-1-a) + (j-i-1)
func InterferenceBitmask(nVregs int, edges [][2]int) uint32 {
	var mask uint32
	for _, e := range edges {
		i, j := e[0], e[1]
		if i > j {
			i, j = j, i
		}
		// Compute edge index
		idx := 0
		for a := 0; a < i; a++ {
			idx += nVregs - 1 - a
		}
		idx += j - i - 1
		mask |= 1 << idx
	}
	return mask
}
