// bridge.go — MIR2 → LIR conversion.
//
// Converts a MIR2 function into a sequence of MIROps (the LIR input format),
// then runs isel + WFC to produce allocated LIR instructions.
// This is the bridge between the existing compiler and the new LIR backend.
//
// Usage:
//
//	mir2Func := ... // from HIR lowering
//	lirResult, err := lir.LowerMIR2(mir2Func, lir.CISC)
//	// lirResult.Insts are allocated LIR instructions
//	// Execute on LIR-VM and compare with MIR2-VM for convergence
package lir

import (
	"fmt"

	"github.com/minz/minzc/pkg/mir2"
)

// LowerResult holds the output of MIR2→LIR lowering.
type LowerResult struct {
	Ops   []MIROp // translated MIR2 ops
	Insts []Inst  // allocated LIR instructions (after isel + WFC)
}

// LowerMIR2Block converts one MIR2 basic block into LIR MIROps.
// This is a straightforward 1:1 translation — isel handles the rest.
// The module parameter is needed for callee contract lookup (OpCall).
func LowerMIR2Block(b *mir2.Block, desc *MachineDesc, mod *mir2.Module, funcParamVRegs ...[]int) ([]MIROp, error) {
	var ops []MIROp

	for _, inst := range b.Insts {
		if inst.Op == mir2.OpCall || inst.Op == mir2.OpCallIndirect {
			callOps, err := translateCall(inst, desc, mod)
			if err != nil {
				return nil, fmt.Errorf("block %s: %w", b.Label, err)
			}
			ops = append(ops, callOps...)
			continue
		}
		// OpMul → runtime CALL to __mul8/__mul16.
		// ISLE combining already reduced constant multiplies (×2 → add, ×4 → shl)
		// at the MIROp level. Any OpMul that reaches here is variable×variable
		// or a const multiply that ISLE didn't handle.
		if inst.Op == mir2.OpMul {
			mulOps := translateMul(inst, desc)
			if mulOps != nil {
				ops = append(ops, mulOps...)
				continue
			}
		}
		op, err := translateInst(inst, desc)
		if err != nil {
			return nil, fmt.Errorf("block %s: %w", b.Label, err)
		}
		if op != nil {
			ops = append(ops, *op)
		}
	}

	// Fuse addr_of + store → store_global, addr_of + load → load_global.
	// LD (sym), HL is 16T vs addr_of→register (10T) + byte-store (22T) = 32T.
	ops = fuseGlobalAccess(ops)

	// Insert save-before-overwrite moves for Z80 (destructive accumulator ops).
	// Only for Z80 — other descriptors have orthogonal register files.
	if desc.Name == "z80" {
		ops = insertSaveBeforeOverwrite(ops, desc)
	}

	return ops, nil
}

// fuseGlobalAccess fuses addr_of + store/load sequences into direct global access.
// Before: OpConst{Sym:"p_name", dst=v1}; OpStore{src0=v1, src1=v2}
// After:  OpStoreGlobal{Sym:"p_name", src0=v2, src1=v2}
//
// LD (sym), HL is 16T vs addr_of→register (10T) + byte-store (22T) = 32T.
func fuseGlobalAccess(ops []MIROp) []MIROp {
	// Build map: vreg → op index for const with symbol (addr_of results).
	addrOfs := make(map[int]int) // vreg → index in ops
	for i, op := range ops {
		if op.Op == OpConst && op.Sym != "" {
			addrOfs[op.Dst] = i
		}
	}
	if len(addrOfs) == 0 {
		return ops
	}

	// Count uses for each vreg to detect single-use addr_of.
	useCount := make(map[int]int)
	for _, op := range ops {
		for _, s := range op.Src {
			if s > 0 {
				useCount[s]++
			}
		}
	}

	fused := make(map[int]bool) // indices to skip (eliminated addr_of)
	var result []MIROp
	for i, op := range ops {
		if fused[i] {
			continue
		}

		if op.Op == OpStore && op.Src[0] > 0 {
			if constIdx, ok := addrOfs[op.Src[0]]; ok && useCount[op.Src[0]] == 1 {
				constOp := ops[constIdx]
				fused[constIdx] = true
				result = append(result, MIROp{
					Op:    OpStoreGlobal,
					Dst:   -1,
					Src:   [2]int{op.Src[1], op.Src[1]},
					Imm:   constOp.Imm,
					Width: op.Width,
					Sym:   constOp.Sym,
				})
				continue
			}
		}

		if op.Op == OpLoad && op.Src[0] > 0 {
			if constIdx, ok := addrOfs[op.Src[0]]; ok && useCount[op.Src[0]] == 1 {
				constOp := ops[constIdx]
				fused[constIdx] = true
				result = append(result, MIROp{
					Op:    OpLoadGlobal,
					Dst:   op.Dst,
					Src:   [2]int{-1, -1},
					Imm:   constOp.Imm,
					Width: op.Width,
					Sym:   constOp.Sym,
				})
				continue
			}
		}

		result = append(result, op)
	}

	return result
}

// insertSaveBeforeOverwrite detects vregs that would be killed by a later
// instruction's destination and are still needed afterwards. For each such vreg,
// inserts a save move (OpMove) to a fresh vreg before the killing instruction,
// and rewrites subsequent uses to reference the saved vreg.
//
// Example: %r3 = add %r1, %r2; %r4 = sub %r1, %r2; %r5 = add %r3, %r4
// %r1 is used by both add and sub, but add's result overwrites %r1's location.
// → Insert: %r_save = move %r1 (before the add)
// → Rewrite: sub uses %r_save instead of %r1
// → Result: %r_save = move %r1; %r3 = add %r1, %r2; %r4 = sub %r_save, %r2; ...
func insertSaveBeforeOverwrite(ops []MIROp, desc *MachineDesc) []MIROp {
	nextVReg := 8000

	// For each ALU op, find src vregs that are used AFTER this op by a DIFFERENT op.
	// Those need to be saved before this op and renamed in subsequent uses.
	type saveInfo struct {
		beforeIdx int // insert save before this op index
		vreg      int // vreg to save
		saveVReg  int // fresh vreg to save into
	}
	var saves []saveInfo

	// Collect all vregs defined as dst of ALU ops (potential A-writes).
	aluDsts := make(map[int]int) // vreg → defining op index
	for i, op := range ops {
		if op.Dst >= 0 && op.Op != OpMove && op.Op != OpConst && op.Op != OpCall {
			aluDsts[op.Dst] = i
		}
	}

	for i, op := range ops {
		if op.Dst < 0 || op.Op == OpMove || op.Op == OpConst || op.Op == OpCall {
			continue
		}
		// This ALU op writes to its dst (likely A on Z80).
		// Find ALL vregs that:
		// 1. Are defined BEFORE this op (as src or dst of earlier ops)
		// 2. Are used AFTER this op
		// 3. Haven't already been saved for this insertion point
		savedHere := make(map[int]bool)

		// Check: any vreg defined before i and used after i.
		// This includes both params and results of earlier ALU ops.
		for j := 0; j < i; j++ {
			dstVReg := ops[j].Dst
			if dstVReg < 0 || savedHere[dstVReg] || dstVReg == op.Dst {
				continue
			}
			// Is dstVReg used after i?
			usedLater := false
			for k := i + 1; k < len(ops); k++ {
				for ss := 0; ss < 2; ss++ {
					if ops[k].Src[ss] == dstVReg {
						usedLater = true
						break
					}
				}
				if usedLater {
					break
				}
			}
			if !usedLater {
				continue
			}
			// Is dstVReg at risk of being overwritten?
			// Conservative: any previous ALU result or param could be in A.
			// On Z80, ALU ops write to A, so any previous vreg that
			// WFC might place in A would be overwritten.
			// We can't know at MIROp level, so save conservatively.
			_, isALU := aluDsts[dstVReg]
			isSrc := false
			for s := 0; s < 2; s++ {
				if op.Src[s] == dstVReg {
					isSrc = true
				}
			}
			if isALU || isSrc {
				sv := nextVReg
				nextVReg++
				saves = append(saves, saveInfo{beforeIdx: i, vreg: dstVReg, saveVReg: sv})
				savedHere[dstVReg] = true
			}
		}

		// Intra-instruction conflict: if this ALU op has 2 different source vregs
		// that could both be in A (both are ALU results), save src1 so the setup
		// move for src0 doesn't overwrite it.
		if op.Src[0] >= 0 && op.Src[1] >= 0 && op.Src[0] != op.Src[1] {
			_, src0IsALU := aluDsts[op.Src[0]]
			_, src1IsALU := aluDsts[op.Src[1]]
			if (src0IsALU || savedHere[op.Src[0]]) && src1IsALU && !savedHere[op.Src[1]] {
				sv := nextVReg
				nextVReg++
				saves = append(saves, saveInfo{beforeIdx: i, vreg: op.Src[1], saveVReg: sv})
				savedHere[op.Src[1]] = true
			}
		}

		// Also save src vregs of THIS op that are used later (original logic).
		for s := 0; s < 2; s++ {
			srcVReg := op.Src[s]
			if srcVReg < 0 || savedHere[srcVReg] {
				continue
			}
			for j := i + 1; j < len(ops); j++ {
				for ss := 0; ss < 2; ss++ {
					if ops[j].Src[ss] == srcVReg {
						sv := nextVReg
						nextVReg++
						saves = append(saves, saveInfo{beforeIdx: i, vreg: srcVReg, saveVReg: sv})
						savedHere[srcVReg] = true
						goto nextSrc
					}
				}
			}
		nextSrc:
		}
	}

	if len(saves) == 0 {
		return ops
	}

	// Build rename map: for each save, rename vreg → saveVReg in ops AFTER beforeIdx.
	// Process saves in order (they may chain).
	// Insert save moves and build the renamed output.
	// Group saves by insertion point.
	type insertionGroup struct {
		saves []saveInfo
	}
	groups := make(map[int][]saveInfo)
	for _, s := range saves {
		groups[s.beforeIdx] = append(groups[s.beforeIdx], s)
	}

	// Build rename map: active renames at each point.
	// A rename is active starting from the instruction AFTER the destructive op.
	activeRenames := make(map[int]int)
	var result []MIROp

	for i, op := range ops {
		// Insert saves before this op.
		if svs, ok := groups[i]; ok {
			for _, sv := range svs {
				// The save uses the ORIGINAL vreg (before any rename from earlier saves)
				srcVReg := sv.vreg
				if renamed, ok2 := activeRenames[srcVReg]; ok2 {
					srcVReg = renamed
				}
				// DstAllowed excludes the accumulator — the save must go
			// to a different register so it survives the destructive ALU op.
			saveDst := desc.LocsOfWidth(op.Width)
			if saveDst.IsEmpty() {
				saveDst = desc.LocsOfWidth(8)
			}
			// For 8-bit ops, exclude A (accumulator, ALU destination).
			// For 16-bit ops, exclude HL (16-bit ALU destination).
			if op.Width <= 8 {
				if aIdx := desc.LocByName("A"); aIdx >= 0 {
					saveDst = saveDst.Clear(aIdx)
				}
			} else {
				if hlIdx := desc.LocByName("HL"); hlIdx >= 0 {
					saveDst = saveDst.Clear(hlIdx)
				}
			}
				result = append(result, MIROp{
					Op:         OpMove,
					Dst:        sv.saveVReg,
					Src:        [2]int{srcVReg, -1},
					Width:      op.Width,
					DstAllowed: saveDst,
				})
			}
		}

		// Apply renames to this op's sources.
		// BUT: the destructive op itself should use the ORIGINAL vregs,
		// not the saves. Renames apply starting from the NEXT op.
		result = append(result, op)

		// Activate renames for saves at this index.
		if svs, ok := groups[i]; ok {
			for _, sv := range svs {
				activeRenames[sv.vreg] = sv.saveVReg
			}
		}
	}

	// Apply renames: for each save, rename usages of the original vreg
	// in ops that correspond to original indices AFTER beforeIdx.
	// We need to map result positions to original op indices.
	origIdx := make([]int, len(result))
	oi := 0
	for ri, op := range result {
		if op.Op == OpMove && op.Dst >= 8000 && op.Dst < 9000 {
			origIdx[ri] = -1 // inserted save, not an original op
		} else {
			origIdx[ri] = oi
			oi++
		}
	}

	for _, sv := range saves {
		for ri := range result {
			oi := origIdx[ri]
			if oi < 0 {
				continue // skip inserted save moves
			}
			if oi < sv.beforeIdx {
				continue // skip ops before the destructive instruction
			}
			// For the instruction AT beforeIdx: only rename src1 (not src0).
			// src0 is the "primary" operand that stays in the original register.
			// For instructions AFTER: rename all sources.
			op := &result[ri]
			if oi == sv.beforeIdx {
				// Only rename src1 (the non-setup operand)
				if op.Src[1] == sv.vreg {
					op.Src[1] = sv.saveVReg
				}
			} else {
				for s := 0; s < 2; s++ {
					if op.Src[s] == sv.vreg {
						op.Src[s] = sv.saveVReg
					}
				}
			}
		}
	}

	return result
}

// FuncContractVRegs extracts virtual register IDs from a function's contract params.
func FuncContractVRegs(f *mir2.Func) []int {
	var vregs []int
	for _, cp := range f.Contract.Params {
		vregs = append(vregs, int(cp.Reg))
	}
	return vregs
}

// insertCallSpills scans MIROps for OpCall and inserts save/restore moves
// for any vreg that is defined before the call and used after it.
// This ensures values survive across calls that clobber all registers.
func insertCallSpills(ops []MIROp, desc *MachineDesc, paramVRegs []int) []MIROp {
	// Find call positions.
	hasCall := false
	for _, op := range ops {
		if op.Op == OpCall {
			hasCall = true
			break
		}
	}
	if !hasCall {
		return ops
	}

	// For each call, find vregs that are:
	// - defined (as Dst) before the call OR are function params
	// - used (as Src) after the call
	// These need to be spilled/restored.
	nextSpillVReg := 9000 // synthetic vreg IDs for spill temporaries

	var result []MIROp
	spillMap := make(map[int]int) // original vreg → restored vreg (for renaming)

	for i, op := range ops {
		if op.Op == OpCall {
			// Find vregs live across this call.
			defsBefore := make(map[int]bool)
			// Include function params as pre-defined.
			for _, pv := range paramVRegs {
				defsBefore[pv] = true
			}
			for j := 0; j < i; j++ {
				if ops[j].Dst >= 0 {
					defsBefore[ops[j].Dst] = true
				}
			}

			usesAfter := make(map[int]bool)
			for j := i + 1; j < len(ops); j++ {
				for s := 0; s < 2; s++ {
					if ops[j].Src[s] >= 0 {
						usesAfter[ops[j].Src[s]] = true
					}
				}
			}

			// Vregs that need saving: defined before AND used after.
			// Exclude the call's own dst (it's defined BY the call).
			var toSave []int
			for vreg := range defsBefore {
				if usesAfter[vreg] && vreg != op.Dst {
					toSave = append(toSave, vreg)
				}
			}

			// Emit save moves: vreg → spill (before call)
			for _, vreg := range toSave {
				spillVReg := nextSpillVReg
				nextSpillVReg++
				result = append(result, MIROp{
					Op:    OpMove,
					Dst:   spillVReg,
					Src:   [2]int{vreg, -1},
					Width: 8,
					DstAllowed: desc.SpillLocs(),
				})
				spillMap[vreg] = spillVReg
			}

			// Emit the call itself
			result = append(result, applySpillRenames(op, spillMap))

			// Emit restore moves: spill → new vreg (after call)
			for _, vreg := range toSave {
				spillVReg := spillMap[vreg]
				restoredVReg := nextSpillVReg
				nextSpillVReg++
				result = append(result, MIROp{
					Op:    OpMove,
					Dst:   restoredVReg,
					Src:   [2]int{spillVReg, -1},
					Width: 8,
				})
				// Update spillMap: subsequent uses of `vreg` should use `restoredVReg`
				spillMap[vreg] = restoredVReg
			}
		} else {
			result = append(result, applySpillRenames(op, spillMap))
		}
	}

	return result
}

// applySpillRenames replaces src vreg references using the spillMap.
// After a save/restore pair, subsequent uses of the original vreg
// should reference the restored vreg instead.
func applySpillRenames(op MIROp, spillMap map[int]int) MIROp {
	for s := 0; s < 2; s++ {
		if restored, ok := spillMap[op.Src[s]]; ok {
			op.Src[s] = restored
		}
	}
	return op
}

// LowerMIR2Func converts a full MIR2 function (all blocks, straight-line)
// into LIR and runs isel + WFC allocation.
func LowerMIR2Func(f *mir2.Func, desc *MachineDesc) (*LowerResult, error) {
	var allOps []MIROp

	fpv := FuncContractVRegs(f)
	for _, b := range f.Blocks {
		ops, err := LowerMIR2Block(b, desc, nil, fpv)
		if err != nil {
			return nil, err
		}
		allOps = append(allOps, ops...)
	}

	if len(allOps) == 0 {
		return &LowerResult{}, nil
	}

	// Extract function params for isel+WFC.
	params := ContractParamsToBlockParams(f, desc)

	// Instruction selection with param pre-seeding
	sel, err := SelectBlockInstructions(desc, allOps, params)
	if err != nil {
		return nil, fmt.Errorf("isel: %w", err)
	}

	// WFC constraint propagation + collapse with param cells
	wfc := NewWFCStateWithParams(desc, sel.Insts, params)
	wfc.Propagate()
	if err := wfc.Collapse(); err != nil {
		return nil, fmt.Errorf("wfc: %w", err)
	}

	return &LowerResult{
		Ops:   allOps,
		Insts: wfc.ToInsts(),
	}, nil
}

// LowerMIR2Prog converts a full MIR2 function into a multi-block LIR Prog,
// preserving block structure, params, terminators, and edge arguments.
// This is the structured alternative to LowerMIR2Func (which flattens).
func LowerMIR2Prog(f *mir2.Func, desc *MachineDesc, mod *mir2.Module) (*Prog, error) {
	prog := &Prog{
		Name:   f.Name,
		Blocks: make([]Block, 0, len(f.Blocks)),
		Desc:   desc,
	}

	for _, mb := range f.Blocks {
		b := Block{Label: mb.Label}

		// Translate block params
		for _, mp := range mb.Params {
			width := 8
			if mp.Ty != nil {
				if w := mp.Ty.Width(); w > 0 {
					width = w
				}
			}
			if width < 8 {
				width = 8
			}
			b.Params = append(b.Params, BlockParam{
				VReg:    int(mp.Dst),
				Allowed: regClassToLocSet(desc, mp.Class, width),
				Phys:    -1,
			})
		}

		// Translate instructions
		ops, err := LowerMIR2Block(mb, desc, mod, FuncContractVRegs(f))
		if err != nil {
			return nil, err
		}
		// Store ops temporarily — isel happens later per-block
		b.Insts = nil // will be filled after isel
		// Stash MIROps in a side map — but for now we embed them as
		// un-selected instructions (no pattern) so we can attach them later.
		// Actually, we'll return a Prog with a parallel ops slice. But
		// Prog doesn't have that. Let's use a simpler approach: store ops
		// in the block's Insts as "raw" Insts with MIROp encoded.
		// Better: just do isel inline.
		_ = ops

		// Translate terminator
		b.Term, err = translateTerm(mb.Term, desc)
		if err != nil {
			return nil, fmt.Errorf("block %s term: %w", mb.Label, err)
		}

		prog.Blocks = append(prog.Blocks, b)
	}

	return prog, nil
}

// LowerMIR2ProgWithOps converts a MIR2 function into a Prog and also returns
// per-block MIROps for later isel. The Prog.Blocks[i].Insts are initially empty.
func LowerMIR2ProgWithOps(f *mir2.Func, desc *MachineDesc, mod *mir2.Module) (*Prog, [][]MIROp, error) {
	prog := &Prog{
		Name:   f.Name,
		Blocks: make([]Block, 0, len(f.Blocks)),
		Desc:   desc,
	}

	blockOps := make([][]MIROp, 0, len(f.Blocks))

	for _, mb := range f.Blocks {
		b := Block{Label: mb.Label}

		// Translate block params
		for _, mp := range mb.Params {
			width := 8
			if mp.Ty != nil {
				if w := mp.Ty.Width(); w > 0 {
					width = w
				}
			}
			if width < 8 {
				width = 8
			}
			b.Params = append(b.Params, BlockParam{
				VReg:    int(mp.Dst),
				Allowed: regClassToLocSet(desc, mp.Class, width),
				Phys:    -1,
			})
		}

		// Translate instructions to MIROps
		ops, err := LowerMIR2Block(mb, desc, mod, FuncContractVRegs(f))
		if err != nil {
			return nil, nil, err
		}
		blockOps = append(blockOps, ops)

		// Translate terminator
		var termErr error
		b.Term, termErr = translateTerm(mb.Term, desc)
		if termErr != nil {
			return nil, nil, fmt.Errorf("block %s term: %w", mb.Label, termErr)
		}

		prog.Blocks = append(prog.Blocks, b)
	}

	return prog, blockOps, nil
}

// LowerMIR2ProgWithEGraph is like LowerMIR2ProgWithOps but uses the e-graph
// bridge for multi-variant lowering. Each block gets an EGraph alongside
// the flattened (cheapest) MIROps.
func LowerMIR2ProgWithEGraph(f *mir2.Func, desc *MachineDesc, mod *mir2.Module) (*Prog, [][]MIROp, []*EGraph, error) {
	prog := &Prog{
		Name:   f.Name,
		Blocks: make([]Block, 0, len(f.Blocks)),
		Desc:   desc,
	}

	blockOps := make([][]MIROp, 0, len(f.Blocks))
	blockGraphs := make([]*EGraph, 0, len(f.Blocks))

	for _, mb := range f.Blocks {
		b := Block{Label: mb.Label}

		for _, mp := range mb.Params {
			width := 8
			if mp.Ty != nil {
				if w := mp.Ty.Width(); w > 0 {
					width = w
				}
			}
			if width < 8 {
				width = 8
			}
			b.Params = append(b.Params, BlockParam{
				VReg:    int(mp.Dst),
				Allowed: regClassToLocSet(desc, mp.Class, width),
				Phys:    -1,
			})
		}

		eg, ops, err := LowerMIR2BlockEGraph(mb, desc, mod)
		if err != nil {
			return nil, nil, nil, err
		}
		blockOps = append(blockOps, ops)
		blockGraphs = append(blockGraphs, eg)

		var termErr error
		b.Term, termErr = translateTerm(mb.Term, desc)
		if termErr != nil {
			return nil, nil, nil, fmt.Errorf("block %s term: %w", mb.Label, termErr)
		}

		prog.Blocks = append(prog.Blocks, b)
	}

	return prog, blockOps, blockGraphs, nil
}

// ContractParamsToBlockParams converts a MIR2 function's Contract.Params
// into LIR BlockParam entries. This allows the flat codegen path to seed
// WFC with the same param constraints that the multi-block path uses.
func ContractParamsToBlockParams(f *mir2.Func, desc *MachineDesc) []BlockParam {
	var params []BlockParam
	for _, cp := range f.Contract.Params {
		width := 8
		if cp.Ty != nil {
			if w := cp.Ty.Width(); w > 0 {
				width = w
			}
		}
		if width < 8 {
			width = 8
		}
		params = append(params, BlockParam{
			VReg:    int(cp.Reg),
			Allowed: regClassToLocSet(desc, cp.Class, width),
			Phys:    -1,
		})
	}
	return params
}

// regClassToLocSet maps a MIR2 register class + width to a LIR LocSet.
func regClassToLocSet(desc *MachineDesc, cls mir2.RegClass, width int) LocSet {
	// For non-Z80 descs, just return all locs of matching width.
	if desc.Name != "z80" {
		s := desc.LocsOfWidth(width)
		if !s.IsEmpty() {
			return s
		}
		return desc.LocsOfWidth(desc.WordSize)
	}

	// Z80-specific class → loc mapping
	switch cls {
	case mir2.ClassAcc:
		return desc.LocSetByNames("A")
	case mir2.ClassCounter:
		return desc.LocSetByNames("B")
	case mir2.ClassPointer:
		return desc.LocSetByNames("HL")
	case mir2.ClassIndex:
		return desc.LocSetByNames("DE")
	case mir2.ClassPair:
		return desc.LocSetByNames("HL", "DE", "BC")
	case mir2.ClassIX:
		return desc.LocSetByNames("IX")
	case mir2.ClassIY:
		return desc.LocSetByNames("IY")
	case mir2.ClassFlag:
		return desc.LocSetByNames("F")
	case mir2.ClassGeneral:
		if width >= 16 {
			return desc.LocSetByNames("HL", "DE", "BC")
		}
		return desc.LocSetByNames("A", "B", "C", "D", "E", "H", "L")
	case mir2.ClassMem:
		s := LocSet(0)
		for i, loc := range desc.Locs {
			if loc.Kind == LocMem {
				s = s.Set(i)
			}
		}
		return s
	default:
		// Fallback: all locs of matching width
		s := desc.LocsOfWidth(width)
		if !s.IsEmpty() {
			return s
		}
		return desc.LocsOfWidth(desc.WordSize)
	}
}

// translateTerm converts a MIR2 terminator to a LIR Term.
func translateTerm(t mir2.Term, desc *MachineDesc) (Term, error) {
	if t == nil {
		return Term{Kind: TermNone}, nil
	}

	regToOp := func(r mir2.Reg) Operand {
		if r == mir2.NoReg {
			return Operand{VReg: -1, Phys: -1}
		}
		return Operand{VReg: int(r), Allowed: desc.LocsOfWidth(8), Phys: -1}
	}

	regsToOps := func(regs []mir2.Reg) []Operand {
		ops := make([]Operand, len(regs))
		for i, r := range regs {
			ops[i] = regToOp(r)
		}
		return ops
	}

	switch tt := t.(type) {
	case *mir2.TermJmp:
		term := Term{
			Kind:    TermJump,
			Targets: []string{tt.Target},
		}
		if len(tt.Args) > 0 {
			term.Args = [][]Operand{regsToOps(tt.Args)}
		}
		return term, nil

	case *mir2.TermBrIf:
		term := Term{
			Kind:    TermBranch,
			Cond:    regToOp(tt.Cond),
			Targets: []string{tt.Then, tt.Else},
			Args:    make([][]Operand, 2),
		}
		if len(tt.ThenArgs) > 0 {
			term.Args[0] = regsToOps(tt.ThenArgs)
		}
		if len(tt.ElseArgs) > 0 {
			term.Args[1] = regsToOps(tt.ElseArgs)
		}
		return term, nil

	case *mir2.TermBrIf2:
		// Three-way branch: split into cmp block → two conditional edges.
		// For now, emit as a TermBranch with Lt as "then" and a fallthrough
		// to handle Eq/Gt. The pipeline will handle this via block splitting.
		// Simplification: encode as TermBranch3 (Lt vs not-Lt), then the
		// not-Lt block branches on Eq vs Gt.
		// Actually, per the plan: TermBrIf2 → two TermBranch blocks.
		// But that's a block-level transform, not a term-level one.
		// At the term level, we can't create new blocks. Return a special
		// encoding that the pipeline splits later.
		// For now: treat as TermBranch comparing Lhs < Rhs.
		// The bridge caller will handle the split.
		return Term{
			Kind: TermBranch,
			Cond: regToOp(tt.Lhs),
			Targets: []string{tt.Lt, tt.Gt},
			Args: [][]Operand{regsToOps(tt.LtArgs), regsToOps(tt.GtArgs)},
		}, nil

	case *mir2.TermDJNZ:
		term := Term{
			Kind:    TermDJNZ,
			Counter: regToOp(tt.Counter),
			Targets: []string{tt.Body, tt.Exit},
			Args:    make([][]Operand, 2),
		}
		if len(tt.BodyArgs) > 0 {
			term.Args[0] = regsToOps(tt.BodyArgs)
		}
		if len(tt.ExitArgs) > 0 {
			term.Args[1] = regsToOps(tt.ExitArgs)
		}
		return term, nil

	case *mir2.TermRet:
		term := Term{
			Kind:    TermReturn,
			RetVals: regsToOps(tt.Vals),
		}
		return term, nil

	case *mir2.TermCondRet:
		// Conditional return: treat as branch to then-block with return as fallthrough.
		// Simplification: if cond==0 return, else jump to Then.
		term := Term{
			Kind:    TermBranch,
			Cond:    regToOp(tt.Cond),
			Targets: []string{tt.Then, ""},
			Args:    make([][]Operand, 2),
		}
		if len(tt.ThenArgs) > 0 {
			term.Args[0] = regsToOps(tt.ThenArgs)
		}
		return term, nil

	case *mir2.TermUnreachable:
		return Term{Kind: TermReturn}, nil

	default:
		return Term{Kind: TermNone}, nil
	}
}

// mulHasConstOperand checks if a multiply instruction has at least one
// constant operand (which ISLE combining can reduce to shifts+adds).
func mulHasConstOperand(inst *mir2.Inst, block *mir2.Block) bool {
	for s := 0; s < 2; s++ {
		src := inst.Src[s]
		if src == mir2.NoReg {
			continue
		}
		// Look for a const definition of this vreg in the same block.
		for _, other := range block.Insts {
			if other.Dst == src && other.Op == mir2.OpConst {
				return true
			}
		}
	}
	return false
}

// translateMul converts a non-constant OpMul into a CALL to a runtime
// multiply routine (__mul8 or __mul16). Returns nil if the multiply
// might be reducible by ISLE (has a constant operand).
func translateMul(inst *mir2.Inst, desc *MachineDesc) []MIROp {
	// Check if either source is a constant — ISLE combining will handle those.
	// We only need runtime CALL for variable × variable.
	// At bridge level we can't easily check if src is const, so we always
	// emit the CALL. ISLE combining runs BEFORE isel and will have already
	// reduced const multiplies to shifts/adds — those won't reach here
	// because the MIR2 OpMul will have been rewritten.
	// Actually, ISLE works on MIROps not MIR2 — so the OpMul MIROp is
	// what ISLE sees. If ISLE reduces it, the MIROp changes to OpAdd/OpShl.
	// If ISLE doesn't reduce it (variable×variable), it stays OpMul and
	// isel fails. So we should always convert OpMul to a CALL here,
	// and let ISLE handle the const cases upstream.

	width := 8
	if inst.Ty != nil {
		if w := inst.Ty.Width(); w > 0 {
			width = w
		}
	}
	if width < 8 {
		width = 8
	}

	var ops []MIROp

	if width <= 8 {
		// __mul8(a: u8 = A, b: u8 = B) -> u8 = A
		// Arg 0: src0 → A
		ops = append(ops, MIROp{
			Op:         OpMove,
			Dst:        7000, // synthetic vreg for arg0
			Src:        [2]int{int(inst.Src[0]), -1},
			Width:      8,
			DstAllowed: desc.LocSetByNames("A"),
		})
		// Arg 1: src1 → B (or any non-A GPR)
		nonA := desc.LocsOfWidth(8)
		if aIdx := desc.LocByName("A"); aIdx >= 0 {
			nonA = nonA.Clear(aIdx)
		}
		ops = append(ops, MIROp{
			Op:         OpMove,
			Dst:        7001,
			Src:        [2]int{int(inst.Src[1]), -1},
			Width:      8,
			DstAllowed: nonA,
		})
		// CALL __mul8
		ops = append(ops, MIROp{
			Op:    OpCall,
			Dst:   int(inst.Dst),
			Src:   [2]int{-1, -1},
			Width: 8,
			Sym:   "__mul8",
		})
	} else {
		// __mul16(a: u16 = HL, b: u16 = DE) -> u16 = HL
		ops = append(ops, MIROp{
			Op:         OpMove,
			Dst:        7000,
			Src:        [2]int{int(inst.Src[0]), -1},
			Width:      16,
			DstAllowed: desc.LocSetByNames("HL"),
		})
		ops = append(ops, MIROp{
			Op:         OpMove,
			Dst:        7001,
			Src:        [2]int{int(inst.Src[1]), -1},
			Width:      16,
			DstAllowed: desc.LocSetByNames("DE"),
		})
		ops = append(ops, MIROp{
			Op:    OpCall,
			Dst:   int(inst.Dst),
			Src:   [2]int{-1, -1},
			Width: 16,
			Sym:   "__mul16",
		})
	}

	return ops
}

// translateCall converts an OpCall/OpCallIndirect into a sequence of LIR MIROps:
// argument setup moves (one per arg) + the call itself.
// Returns nil, nil if the call can't be lowered (e.g. indirect call, missing module).
func translateCall(inst *mir2.Inst, desc *MachineDesc, mod *mir2.Module) ([]MIROp, error) {
	if inst.Op == mir2.OpCallIndirect {
		return translateCallIndirect(inst, desc)
	}
	if mod == nil {
		return nil, nil // can't look up callee without module
	}

	callee := mod.FuncByName(inst.Sym)
	if callee == nil {
		return nil, nil // extern/unknown callee — skip
	}

	var ops []MIROp

	// Emit argument setup: move each arg vreg into its param-class-constrained vreg.
	// The isel will pick the right move pattern, and WFC will collapse to the
	// physical register matching the param class.
	for i, argReg := range inst.Args {
		if i >= len(callee.Contract.Params) {
			break
		}
		cp := callee.Contract.Params[i]
		width := 8
		if cp.Ty != nil {
			if w := cp.Ty.Width(); w > 0 {
				width = w
			}
		}
		if width < 8 {
			width = 8
		}
		// Emit: move argReg → constrained vreg matching callee param class.
		// DstAllowed tells isel to narrow the move destination to the
		// callee's expected register class.
		ops = append(ops, MIROp{
			Op:         OpMove,
			Dst:        int(cp.Reg),
			Src:        [2]int{int(argReg), -1},
			Width:      width,
			DstAllowed: regClassToLocSet(desc, cp.Class, width),
		})
	}

	// Emit the call itself.
	width := 0 // void
	if inst.Ty != nil {
		w := inst.Ty.Width()
		if w > 0 {
			width = w
		}
	}
	if width < 8 && width > 0 {
		width = 8
	}

	callOp := MIROp{
		Op:    OpCall,
		Dst:   int(inst.Dst),
		Src:   [2]int{-1, -1},
		Width: width,
		Sym:   inst.Sym,
	}
	if inst.Dst == mir2.NoReg {
		callOp.Dst = -1
	}

	ops = append(ops, callOp)
	return ops, nil
}

// translateCallIndirect converts OpCallIndirect to a CALL via trampoline.
// On Z80: move function pointer to HL, move args to standard ABI regs,
// then CALL __call_hl (which does JP (HL)).
func translateCallIndirect(inst *mir2.Inst, desc *MachineDesc) ([]MIROp, error) {
	var ops []MIROp

	// Move function pointer (Src[0]) to HL for indirect call.
	if inst.Src[0] != mir2.NoReg {
		ops = append(ops, MIROp{
			Op:         OpMove,
			Dst:        7010, // synthetic vreg for fn ptr
			Src:        [2]int{int(inst.Src[0]), -1},
			Width:      16,
			DstAllowed: desc.LocSetByNames("HL"),
		})
	}

	// Move args to standard ABI positions.
	// Standard Z80 indirect call ABI: arg0=A (u8) or HL (u16), arg1=C/DE, arg2=B.
	abiRegs8 := []string{"A", "C", "B"}
	abiRegs16 := []string{"HL", "DE", "BC"}
	for i, argReg := range inst.Args {
		width := 8
		if inst.Ty != nil && inst.Ty.Width() > 8 {
			width = 16
		}
		var dstAllowed LocSet
		if width <= 8 && i < len(abiRegs8) {
			dstAllowed = desc.LocSetByNames(abiRegs8[i])
		} else if i < len(abiRegs16) {
			dstAllowed = desc.LocSetByNames(abiRegs16[i])
		}
		if dstAllowed.IsEmpty() {
			continue
		}
		ops = append(ops, MIROp{
			Op:         OpMove,
			Dst:        7020 + i,
			Src:        [2]int{int(argReg), -1},
			Width:      width,
			DstAllowed: dstAllowed,
		})
	}

	// Emit CALL __call_hl
	width := 8
	if inst.Ty != nil {
		if w := inst.Ty.Width(); w > 0 {
			width = w
		}
	}
	if width < 8 {
		width = 8
	}

	callOp := MIROp{
		Op:    OpCall,
		Dst:   int(inst.Dst),
		Src:   [2]int{-1, -1},
		Width: width,
		Sym:   "__call_hl",
	}
	if inst.Dst == mir2.NoReg {
		callOp.Dst = -1
	}
	ops = append(ops, callOp)

	return ops, nil
}

// translateInst converts one MIR2 instruction to a LIR MIROp.
func translateInst(inst *mir2.Inst, desc *MachineDesc) (*MIROp, error) {
	if inst.Dst == mir2.NoReg && inst.Op != mir2.OpStore {
		return nil, nil // skip side-effect-free instructions with no result
	}

	width := 8
	if inst.Ty != nil {
		w := inst.Ty.Width()
		if w > 0 {
			width = w
		}
	}
	// Several ops produce or consume booleans (width=1) but the actual
	// operation uses 8-bit registers. Promote narrow widths to 8.
	if width < 8 {
		width = 8
	}
	// Cap width to max register width. Struct types may report their full
	// byte size (e.g. Arena = 32 bits) but the register holds a pointer (16 bits).
	// On Z80/CISC, max register width is 16.
	maxWidth := desc.WordSize
	if maxWidth < 16 {
		maxWidth = 16
	}
	if width > maxWidth {
		width = maxWidth
	}

	// Mask immediate to width to prevent overflow (CP 4294967295 → CP 255).
	imm := inst.Imm
	if width <= 8 {
		imm &= 0xFF
	} else if width <= 16 {
		imm &= 0xFFFF
	}

	op := &MIROp{
		Dst:   int(inst.Dst),
		Src:   [2]int{int(inst.Src[0]), int(inst.Src[1])},
		Imm:   imm,
		Width: width,
	}

	// Fix NoReg → -1
	if inst.Dst == mir2.NoReg {
		op.Dst = -1
	}
	if inst.Src[0] == mir2.NoReg {
		op.Src[0] = -1
	}
	if inst.Src[1] == mir2.NoReg {
		op.Src[1] = -1
	}

	switch inst.Op {
	case mir2.OpConst:
		op.Op = OpConst
	case mir2.OpMove:
		op.Op = OpMove
	case mir2.OpAdd:
		op.Op = OpAdd
	case mir2.OpSub:
		op.Op = OpSub
	case mir2.OpMul:
		op.Op = OpMul
	case mir2.OpAnd:
		op.Op = OpAnd
	case mir2.OpOr:
		op.Op = OpOr
	case mir2.OpXor:
		op.Op = OpXor
	case mir2.OpShl:
		op.Op = OpShl
	case mir2.OpShr, mir2.OpSar:
		op.Op = OpShr
	case mir2.OpCmp:
		// CmpSubCarry: carry flag already set by preceding SUB — no instruction needed.
		if inst.Cond == mir2.CmpSubCarry || inst.Cond == mir2.CmpSubCarryNot {
			return nil, nil // flag already in F register
		}
		op.Op = OpCmp
		// CMP result is bool (width=1 → 8), but operands may be 16-bit.
		if inst.SrcTy != nil {
			w := inst.SrcTy.Width()
			if w > op.Width {
				op.Width = w
			}
		}
	case mir2.OpLoad:
		op.Op = OpLoad
	case mir2.OpStore:
		op.Op = OpStore
		op.Dst = -1
	case mir2.OpNeg:
		// neg(x) → sub(0, x): emit const 0 + sub
		// For now, skip — isel doesn't have a neg pattern yet
		return nil, nil
	case mir2.OpNot:
		// bitwise complement — skip for now
		return nil, nil
	case mir2.OpTrunc:
		// Truncation u16→u8: extract low byte. On Z80 this means the src
		// must be in a pair (HL/DE/BC) and the result takes the low byte (L/E/C).
		// We model this as OpMove with width=8 and SrcAllowed constrained
		// to 8-bit regs (the low byte will be selected by the pattern).
		op.Op = OpMove
		op.Width = 8
		op.Src[1] = -1
		// Constrain: dst must be in 8-bit GPR
		op.DstAllowed = desc.LocsOfWidth(8)
	case mir2.OpExt, mir2.OpSext:
		// Widening: 8→16 bit. Keep width from the dest type (16).
		op.Op = OpMove
		op.Src[1] = -1
	case mir2.OpField, mir2.OpPtrBump:
		// Pointer offset — treat as add with immediate
		op.Op = OpAdd
		// Imm is the byte offset; src[1] is unused, use const
		op.Src[1] = -1
	case mir2.OpPtrAdd:
		op.Op = OpAdd
	case mir2.OpAddrOf:
		// Address of global — treat as const with symbol address.
		// The symbol name from MIR2 (inst.Sym) becomes the Sym field
		// so isel emits "LD rr, symbol" instead of "LD rr, 0".
		op.Op = OpConst
		op.Src = [2]int{-1, -1}
		op.Sym = SanitizeAsmLabel(inst.Sym)
		op.Width = 16 // addresses are always 16-bit on Z80
	case mir2.OpCall, mir2.OpCallIndirect:
		// Calls: skip for now (need calling convention support)
		return nil, nil
	case mir2.OpPush, mir2.OpPop:
		// Stack ops: skip (handled by regalloc)
		return nil, nil
	case mir2.OpPatchSlot, mir2.OpLoadPatched, mir2.OpPatch:
		// SMC: skip (target-specific)
		return nil, nil
	case mir2.OpAsm:
		// Inline assembly: skip
		return nil, nil
	case mir2.OpAlloca:
		// Stack alloc: skip
		return nil, nil
	default:
		return nil, nil
	}

	return op, nil
}

// RunConvergence executes the same MIR2 function on MIR2-VM and LIR-VM,
// comparing results. Returns error on divergence.
func RunConvergence(f *mir2.Func, m *mir2.Module, desc *MachineDesc, args []mir2.Value) error {
	// MIR2-VM execution
	vm := mir2.NewVM(m)
	mir2Result, err := vm.Call(f.Name, args)
	if err != nil {
		return fmt.Errorf("mir2-vm: %w", err)
	}

	// LIR lowering + execution
	lirResult, err := LowerMIR2Func(f, desc)
	if err != nil {
		return fmt.Errorf("lir lower: %w", err)
	}

	lirVM := NewVM(desc)
	for i := range lirResult.Insts {
		if err := lirVM.ExecInst(&lirResult.Insts[i]); err != nil {
			return fmt.Errorf("lir-vm inst %d: %w", i, err)
		}
	}

	// Compare: MIR2 return value vs LIR last instruction's destination
	if len(mir2Result) == 0 {
		return nil // void function, nothing to compare
	}
	mir2Val := mir2Result[0].I

	if len(lirResult.Insts) == 0 {
		return fmt.Errorf("lir produced no instructions")
	}
	lastInst := lirResult.Insts[len(lirResult.Insts)-1]
	if lastInst.Dst.Phys < 0 {
		return fmt.Errorf("lir last instruction has no physical destination")
	}
	lirVal := int64(lirVM.Get(lastInst.Dst.Phys))

	if mir2Val != lirVal {
		return fmt.Errorf("DIVERGENCE: mir2-vm=%d, lir-vm(%s)=%d",
			mir2Val, desc.Name, lirVal)
	}

	return nil
}
