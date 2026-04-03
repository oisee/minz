package mir2

import "sort"

// SolverFriendlyShapeFacts is a conservative MIR2-side summary of structural
// motifs that often correlate with solver-unfriendly code shape.
//
// The goal is not full loop or alias analysis. It is a practical signal layer
// for future Grace transforms such as indexed-loop -> pointer-walk conversion
// and row-helper extraction.
type SolverFriendlyShapeFacts struct {
	Loops             []LoopRegionFact
	IndexedAccesses   []IndexedAccessFact
	RepeatedAddrTerms []RepeatedAddrTermFact
}

// LoopRegionFact marks a conservative loop region inferred from a backedge in
// block layout order.
type LoopRegionFact struct {
	Header string
	Latch  string
	Blocks []string
}

// IndexedAccessFact records a memory access whose pointer comes from OpPtrAdd.
// These are the first candidates for pointer-walk conversion.
type IndexedAccessFact struct {
	BlockLabel string
	AccessOp   Op
	Width      int
	PtrReg     Reg
	BaseReg    Reg
	OffsetReg  Reg
	PtrKey     string
	BaseKey    string
	OffsetKey  string
	InLoop     bool
}

// RepeatedAddrTermFact records repeated indexed-address pressure inside a
// single block. Kind is either:
//   - "same_term": the exact ptr_add(base, offset) term repeats
//   - "same_base": multiple indexed accesses share the same base root
type RepeatedAddrTermFact struct {
	BlockLabel string
	Kind       string
	Key        string
	Count      int
	InLoop     bool
}

// CollectSolverFriendlyShapeFacts extracts conservative structural signals
// from a MIR2 function that can later feed Grace profitability and transform
// selection.
func CollectSolverFriendlyShapeFacts(f *Func) SolverFriendlyShapeFacts {
	var facts SolverFriendlyShapeFacts
	if f == nil || len(f.Blocks) == 0 {
		return facts
	}

	blockIndex := make(map[string]int, len(f.Blocks))
	for i, b := range f.Blocks {
		blockIndex[b.Label] = i
	}

	loopBlocks := make(map[string]bool)
	for _, loop := range findConservativeLoopRegions(f, blockIndex) {
		facts.Loops = append(facts.Loops, loop)
		for _, label := range loop.Blocks {
			loopBlocks[label] = true
		}
	}

	for _, b := range f.Blocks {
		defInst := make(map[Reg]*Inst, len(b.Insts))
		for _, inst := range b.Insts {
			if inst.Dst != NoReg {
				defInst[inst.Dst] = inst
			}
		}

		sameTermCounts := make(map[string]int)
		sameBaseCounts := make(map[string]int)

		for _, inst := range b.Insts {
			var ptrReg Reg
			switch inst.Op {
			case OpLoad:
				ptrReg = inst.Src[0]
			case OpStore:
				ptrReg = inst.Src[0]
			default:
				continue
			}

			ptrInst, ok := defInst[ptrReg]
			if !ok || ptrInst.Op != OpPtrAdd {
				continue
			}

			ptrKey := valueKey(ptrReg, f)
			baseKey := valueKey(ptrInst.Src[0], f)
			offsetKey := valueKey(ptrInst.Src[1], f)
			if ptrKey == "" || baseKey == "" {
				continue
			}

			facts.IndexedAccesses = append(facts.IndexedAccesses, IndexedAccessFact{
				BlockLabel: b.Label,
				AccessOp:   inst.Op,
				Width:      inst.Ty.Width(),
				PtrReg:     ptrReg,
				BaseReg:    ptrInst.Src[0],
				OffsetReg:  ptrInst.Src[1],
				PtrKey:     ptrKey,
				BaseKey:    baseKey,
				OffsetKey:  offsetKey,
				InLoop:     loopBlocks[b.Label],
			})
			sameTermCounts[ptrKey]++
			sameBaseCounts[baseKey]++
		}

		for key, count := range sameTermCounts {
			if count >= 2 {
				facts.RepeatedAddrTerms = append(facts.RepeatedAddrTerms, RepeatedAddrTermFact{
					BlockLabel: b.Label,
					Kind:       "same_term",
					Key:        key,
					Count:      count,
					InLoop:     loopBlocks[b.Label],
				})
			}
		}
		for key, count := range sameBaseCounts {
			if count >= 2 {
				facts.RepeatedAddrTerms = append(facts.RepeatedAddrTerms, RepeatedAddrTermFact{
					BlockLabel: b.Label,
					Kind:       "same_base",
					Key:        key,
					Count:      count,
					InLoop:     loopBlocks[b.Label],
				})
			}
		}
	}

	sort.Slice(facts.Loops, func(i, j int) bool {
		if facts.Loops[i].Header != facts.Loops[j].Header {
			return facts.Loops[i].Header < facts.Loops[j].Header
		}
		return facts.Loops[i].Latch < facts.Loops[j].Latch
	})
	sort.Slice(facts.IndexedAccesses, func(i, j int) bool {
		if facts.IndexedAccesses[i].BlockLabel != facts.IndexedAccesses[j].BlockLabel {
			return facts.IndexedAccesses[i].BlockLabel < facts.IndexedAccesses[j].BlockLabel
		}
		if facts.IndexedAccesses[i].PtrKey != facts.IndexedAccesses[j].PtrKey {
			return facts.IndexedAccesses[i].PtrKey < facts.IndexedAccesses[j].PtrKey
		}
		return facts.IndexedAccesses[i].AccessOp < facts.IndexedAccesses[j].AccessOp
	})
	sort.Slice(facts.RepeatedAddrTerms, func(i, j int) bool {
		if facts.RepeatedAddrTerms[i].BlockLabel != facts.RepeatedAddrTerms[j].BlockLabel {
			return facts.RepeatedAddrTerms[i].BlockLabel < facts.RepeatedAddrTerms[j].BlockLabel
		}
		if facts.RepeatedAddrTerms[i].Kind != facts.RepeatedAddrTerms[j].Kind {
			return facts.RepeatedAddrTerms[i].Kind < facts.RepeatedAddrTerms[j].Kind
		}
		return facts.RepeatedAddrTerms[i].Key < facts.RepeatedAddrTerms[j].Key
	})

	return facts
}

func findConservativeLoopRegions(f *Func, blockIndex map[string]int) []LoopRegionFact {
	var loops []LoopRegionFact
	seen := make(map[string]bool)
	for i, b := range f.Blocks {
		if b.Term == nil {
			continue
		}
		for _, succ := range b.Term.Successors() {
			j, ok := blockIndex[succ]
			if !ok || j > i {
				continue
			}
			key := succ + "->" + b.Label
			if seen[key] {
				continue
			}
			seen[key] = true

			blocks := make([]string, 0, i-j+1)
			for k := j; k <= i; k++ {
				blocks = append(blocks, f.Blocks[k].Label)
			}
			loops = append(loops, LoopRegionFact{
				Header: succ,
				Latch:  b.Label,
				Blocks: blocks,
			})
		}
	}
	return loops
}
