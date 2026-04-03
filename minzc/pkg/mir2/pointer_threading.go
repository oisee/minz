// pointer_threading.go — True pointer threading through block args.
//
// Rewrites simple indexed loops that rebuild ptr_add(base, ext(index)) each
// iteration into loops that carry a walking pointer as a block argument.
// The hot loop body then uses the carried pointer directly for memory ops
// instead of reconstructing the address from base+index.
//
// This is a structural MIR2 rewrite — it modifies block params, terminators,
// and instruction sequences. It is NOT a local CSE pass.
//
// Conservative: only fires on simple single-body loops with constant stride,
// static base, and straightforward index threading.
//
// Called from RunGracePasses after declarative Grace rules and ptr-add CSE.
package mir2

// PointerThreadingStats tracks transform activity.
type PointerThreadingStats struct {
	FuncsVisited    int
	LoopsThreaded   int
	PtrAddsRemoved  int
}

// ApplyPointerThreading scans f for eligible indexed loops and rewrites them
// to carry a walking pointer through block args. Returns true if any rewrite fired.
func ApplyPointerThreading(f *Func, stats *PointerThreadingStats) bool {
	if f == nil || len(f.Blocks) == 0 {
		return false
	}
	if stats != nil {
		stats.FuncsVisited++
	}

	facts := CollectSolverFriendlyShapeFacts(f)
	changed := false

	for _, loop := range facts.Loops {
		if threadPointerInLoop(f, &loop, &facts, stats) {
			changed = true
		}
	}
	return changed
}

// threadPointerInLoop attempts to rewrite one loop to carry a walking pointer.
//
// Eligible shape:
//   - header has exactly one body successor (simple loop)
//   - body block jumps back to header (latch = body)
//   - body contains OpPtrAdd(base, ext(indexParam)) where indexParam is a
//     header block arg and base is loop-invariant (OpAddrOf or func param)
//   - index is updated by a constant stride (add/sub const) in the body
//   - the latch passes the updated index back to header
func threadPointerInLoop(f *Func, loop *LoopRegionFact, facts *SolverFriendlyShapeFacts, stats *PointerThreadingStats) bool {
	header := f.BlockByLabel(loop.Header)
	if header == nil || len(loop.Blocks) < 2 {
		return false
	}

	// Find body block: must be the latch that jumps back to header.
	latchBlock := f.BlockByLabel(loop.Latch)
	if latchBlock == nil {
		return false
	}
	latchJmp, ok := latchBlock.Term.(*TermJmp)
	if !ok || latchJmp.Target != loop.Header {
		return false
	}

	// Header must have block params (loop-carried values).
	if len(header.Params) == 0 {
		return false
	}

	// Build def map for the body block.
	bodyDefs := make(map[Reg]*Inst)
	for _, inst := range latchBlock.Insts {
		if inst.Dst != NoReg {
			bodyDefs[inst.Dst] = inst
		}
	}

	// For each header param, check if it's an index used in ptr_add(base, ext(param)).
	for paramIdx, param := range header.Params {
		candidate := findThreadingCandidate(f, header, latchBlock, bodyDefs, param, paramIdx, latchJmp)
		if candidate == nil {
			continue
		}

		// Apply the rewrite.
		applyThreading(f, header, latchBlock, latchJmp, candidate)
		if stats != nil {
			stats.LoopsThreaded++
			stats.PtrAddsRemoved += candidate.ptrAddsToRemove
		}
		return true // one rewrite per loop per pass (conservative)
	}

	return false
}

// ptrAddEntry describes one ptr_add to rewrite, with its constant delta
// from the base walked pointer (0 = exact match, N = offset by N bytes).
type ptrAddEntry struct {
	inst  *Inst
	delta int64 // constant byte offset from base pointer (0 = no offset)
}

// threadingCandidate holds all the info needed to apply one pointer threading rewrite.
type threadingCandidate struct {
	paramIdx      int    // index of the loop-carried index param in header.Params
	indexReg      Reg    // the header param reg holding the index
	baseReg       Reg    // the loop-invariant base pointer reg
	baseTy        Ty     // type of base
	baseCls       RegClass
	strideImm     int64  // constant stride value
	strideOp      Op     // OpAdd or OpSub
	ptrAddEntries []ptrAddEntry // ptr_add instructions to rewrite, with deltas
	ptrAddsToRemove int
	// Where in latchJmp.Args the updated index is passed back
	latchArgIdx   int
	// The reg holding the updated index value in the latch
	updatedIndexReg Reg
	// Byte width of memory accesses through the ptr_add (1 for u8, 2 for u16).
	accessWidthBytes int
}

func findThreadingCandidate(f *Func, header, body *Block, bodyDefs map[Reg]*Inst, param BlockParam, paramIdx int, latchJmp *TermJmp) *threadingCandidate {
	indexReg := param.Dst

	// Find ptr_add instructions in body that use ext(indexReg) or
	// ext(indexReg)+const as offset, with the same loop-invariant base.
	var entries []ptrAddEntry
	var baseReg Reg
	var baseTy Ty
	var baseCls RegClass

	for _, inst := range body.Insts {
		if inst.Op != OpPtrAdd {
			continue
		}

		// Check if base is loop-invariant (OpAddrOf in body).
		bReg := inst.Src[0]
		if !isLoopInvariantBase(bReg, body, bodyDefs, f) {
			continue
		}

		// Check if offset derives from indexReg, possibly with a constant delta.
		offsetReg := inst.Src[1]
		delta, ok := offsetDeltaFromIndex(offsetReg, indexReg, bodyDefs, header)
		if !ok {
			continue
		}

		if len(entries) == 0 {
			baseReg = bReg
			if bInst := findDefInBlock(bReg, body); bInst != nil {
				baseTy = bInst.Ty
				baseCls = bInst.Cls
			} else {
				baseTy = TyPtr
				baseCls = ClassPointer
			}
		} else if bReg != baseReg {
			// Multiple different bases — skip entirely.
			return nil
		}

		entries = append(entries, ptrAddEntry{inst: inst, delta: delta})
	}

	if len(entries) == 0 {
		return nil
	}

	// Determine access width from loads/stores consuming the ptr_add results.
	accessWidthBytes := 0
	for _, e := range entries {
		for _, inst := range body.Insts {
			if (inst.Op == OpLoad || inst.Op == OpStore) && inst.Src[0] == e.inst.Dst {
				w := 1
				if inst.Ty != nil {
					wb := inst.Ty.Width() / 8
					if wb > 0 {
						w = wb
					}
				}
				if accessWidthBytes == 0 {
					accessWidthBytes = w
				} else if accessWidthBytes != w {
					return nil // conflicting widths — reject
				}
			}
		}
	}
	if accessWidthBytes == 0 {
		accessWidthBytes = 1
	}

	// Find constant stride: the index update pattern.
	// Look for the value passed back to header for this param.
	if paramIdx >= len(latchJmp.Args) {
		return nil
	}
	updatedIdx := latchJmp.Args[paramIdx]

	strideOp, strideImm, ok := findConstantStride(updatedIdx, indexReg, bodyDefs, header)
	if !ok {
		return nil
	}

	return &threadingCandidate{
		paramIdx:         paramIdx,
		indexReg:         indexReg,
		baseReg:          baseReg,
		baseTy:           baseTy,
		baseCls:          baseCls,
		strideImm:        strideImm,
		strideOp:         strideOp,
		ptrAddEntries:    entries,
		ptrAddsToRemove:  len(entries),
		latchArgIdx:      paramIdx,
		updatedIndexReg:  updatedIdx,
		accessWidthBytes: accessWidthBytes,
	}
}

// offsetDeltaFromIndex checks if offsetReg derives from indexReg with an
// optional constant delta. Returns (delta, true) or (0, false).
//
// Accepted patterns:
//   - offsetReg == indexReg                    → delta=0
//   - offsetReg = ext(indexReg)                → delta=0
//   - offsetReg = ext(indexReg) + const        → delta=const
//   - offsetReg = add(ext(indexReg), const)     → delta=const
func offsetDeltaFromIndex(offsetReg, indexReg Reg, bodyDefs map[Reg]*Inst, header *Block) (int64, bool) {
	// Direct match.
	if offsetReg == indexReg {
		return 0, true
	}

	// Resolve the defining instruction (body or header).
	inst := resolveOffsetDef(offsetReg, bodyDefs, header)
	if inst == nil {
		return 0, false
	}

	// ext(indexReg) → delta=0
	if (inst.Op == OpExt || inst.Op == OpSext) && inst.Src[0] == indexReg {
		return 0, true
	}

	// add(X, const) where X derives from indexReg → delta=const
	if inst.Op == OpAdd {
		// Check: src[0] derives from index, src[1] is const
		if d, ok := offsetDeltaFromIndex(inst.Src[0], indexReg, bodyDefs, header); ok {
			if cInst := resolveOffsetDef(inst.Src[1], bodyDefs, header); cInst != nil && cInst.Op == OpConst {
				return d + cInst.Imm, true
			}
		}
		// Check commuted: src[1] derives from index, src[0] is const
		if d, ok := offsetDeltaFromIndex(inst.Src[1], indexReg, bodyDefs, header); ok {
			if cInst := resolveOffsetDef(inst.Src[0], bodyDefs, header); cInst != nil && cInst.Op == OpConst {
				return d + cInst.Imm, true
			}
		}
	}

	return 0, false
}

// resolveOffsetDef finds the defining instruction for a reg in body or header.
func resolveOffsetDef(reg Reg, bodyDefs map[Reg]*Inst, header *Block) *Inst {
	if inst, ok := bodyDefs[reg]; ok {
		return inst
	}
	for _, inst := range header.Insts {
		if inst.Dst == reg {
			return inst
		}
	}
	return nil
}

// isLoopInvariantBase checks if reg is defined by OpAddrOf in the body block.
// For this first conservative pass, we only accept OpAddrOf bases because
// initial-pointer construction recreates the base by symbol lookup.
func isLoopInvariantBase(reg Reg, body *Block, bodyDefs map[Reg]*Inst, f *Func) bool {
	if inst, ok := bodyDefs[reg]; ok {
		return inst.Op == OpAddrOf
	}
	// Not defined in body — could be func param or entry-defined.
	// Reject for now: we can't recreate it via OpAddrOf at entry edges.
	return false
}

// findConstantStride checks if updatedIdx = indexReg +/- const.
func findConstantStride(updatedIdx, indexReg Reg, bodyDefs map[Reg]*Inst, header *Block) (Op, int64, bool) {
	inst, ok := bodyDefs[updatedIdx]
	if !ok {
		return 0, 0, false
	}

	if inst.Op != OpAdd && inst.Op != OpSub {
		return 0, 0, false
	}

	// Pattern: updatedIdx = indexReg op constReg
	if inst.Src[0] != indexReg {
		return 0, 0, false
	}

	// Check if Src[1] is a constant.
	strideReg := inst.Src[1]
	if strideInst, ok := bodyDefs[strideReg]; ok && strideInst.Op == OpConst {
		return inst.Op, strideInst.Imm, true
	}

	return 0, 0, false
}

func findDefInBlock(reg Reg, b *Block) *Inst {
	for _, inst := range b.Insts {
		if inst.Dst == reg {
			return inst
		}
	}
	return nil
}

// applyThreading performs the actual structural rewrite:
// 1. Add a new block param to header for the walking pointer.
// 2. Compute initial pointer value (base + ext(initial_index)) at each entry edge.
// 3. Replace ptr_add instructions in body with uses of the carried pointer.
// 4. Add pointer advance (ptr + stride) at latch and pass to header.
// 5. Update all edges that jump to header with the new arg.
func applyThreading(f *Func, header, body *Block, latchJmp *TermJmp, c *threadingCandidate) {
	// 1. Allocate new reg for the walking pointer param in header.
	walkPtrReg := f.AllocReg()
	header.Params = append(header.Params, BlockParam{
		Dst:   walkPtrReg,
		Ty:    TyPtr,
		Class: ClassPointer,
	})
	newParamIdx := len(header.Params) - 1

	// 2. For every predecessor edge jumping to header, compute initial ptr.
	// We need to add an arg for the new param on every edge to header.
	for _, b := range f.Blocks {
		if b.Term == nil {
			continue
		}
		b.Term.ForEachEdge(func(target string, args []Reg, _ int) {
			if target != header.Label {
				return
			}
			// This edge needs a new arg: base + ext(index_at_this_edge).
			// For the latch edge, we'll handle separately (advance pointer).
			// For entry edges, compute base + ext(initial_index).
		})

		switch tt := b.Term.(type) {
		case *TermJmp:
			if tt.Target == header.Label && b.Label != body.Label {
				// Entry edge: compute initial pointer.
				initPtr := computeInitialPointer(f, b, c)
				tt.Args = append(tt.Args, initPtr)
			}
		case *TermBrIf:
			if tt.Then == header.Label {
				initPtr := computeInitialPointerForBrIf(f, b, c, true)
				tt.ThenArgs = append(tt.ThenArgs, initPtr)
			}
			if tt.Else == header.Label {
				initPtr := computeInitialPointerForBrIf(f, b, c, false)
				tt.ElseArgs = append(tt.ElseArgs, initPtr)
			}
		}
	}

	// 3. Thread walkPtrReg through to body block.
	// If header has a br_if to body, add the walkPtrReg as a forwarded arg.
	// Add a new block param to body for the walk pointer.
	bodyWalkReg := f.AllocReg()
	body.Params = append(body.Params, BlockParam{
		Dst:   bodyWalkReg,
		Ty:    TyPtr,
		Class: ClassPointer,
	})

	// Update header's terminator to pass walkPtrReg to body.
	switch tt := header.Term.(type) {
	case *TermBrIf:
		if tt.Then == body.Label {
			tt.ThenArgs = append(tt.ThenArgs, walkPtrReg)
		}
		if tt.Else == body.Label {
			tt.ElseArgs = append(tt.ElseArgs, walkPtrReg)
		}
	case *TermJmp:
		if tt.Target == body.Label {
			tt.Args = append(tt.Args, walkPtrReg)
		}
	}

	// 4. Replace ptr_add instructions in body.
	// For delta=0 entries: result is bodyWalkReg directly.
	// For delta!=0 entries: rewrite to ptr_add(bodyWalkReg, delta_const).
	for _, entry := range c.ptrAddEntries {
		oldDst := entry.inst.Dst
		if entry.delta == 0 {
			// Exact match — replace all uses with bodyWalkReg.
			for _, inst := range body.Insts {
				replaceSrc(inst, oldDst, bodyWalkReg)
			}
			if body.Term != nil {
				replaceTermReg(body.Term, oldDst, bodyWalkReg)
			}
			// Dead-ify the original ptr_add.
			entry.inst.Op = OpMove
			entry.inst.Src = [2]Reg{bodyWalkReg, NoReg}
		} else {
			// Offset access — rewrite ptr_add to use walked pointer + delta.
			deltaReg := f.AllocReg()
			// Insert a const instruction for the delta right before this ptr_add.
			deltaConst := &Inst{
				Op:  OpConst,
				Dst: deltaReg,
				Imm: entry.delta,
				Ty:  TyU16,
			}
			// Find position of this ptr_add and insert delta const before it.
			for i, inst := range body.Insts {
				if inst == entry.inst {
					body.Insts = append(body.Insts[:i+1], body.Insts[i:]...)
					body.Insts[i] = deltaConst
					break
				}
			}
			// Rewrite the ptr_add to use bodyWalkReg + deltaReg.
			entry.inst.Src = [2]Reg{bodyWalkReg, deltaReg}
		}
	}

	// 5. Advance pointer at latch: new_ptr = bodyWalkReg +/- stride.
	// When the index is decremented (OpSub), the pointer must also decrease.
	// Note: stride is in the same units as the original index (byte offsets).
	// The original ptr_add(base, ext(index)) already treats index as bytes.
	// accessWidthBytes is recorded for future width-scaled optimizations.
	strideReg := f.AllocReg()
	advancedPtr := f.AllocReg()

	ptrStride := c.strideImm
	if c.strideOp == OpSub {
		ptrStride = -ptrStride // countdown loop → pointer moves backward
	}

	strideConst := &Inst{
		Op:  OpConst,
		Dst: strideReg,
		Imm: ptrStride,
		Ty:  TyU16,
	}

	advanceInst := &Inst{
		Op:  OpPtrAdd,
		Dst: advancedPtr,
		Src: [2]Reg{bodyWalkReg, strideReg},
		Ty:  TyPtr,
		Cls: ClassPointer,
	}

	// Insert before the terminator (append to body.Insts).
	body.Insts = append(body.Insts, strideConst, advanceInst)

	// 6. Update latch jump to pass advanced pointer to header's new param.
	latchJmp.Args = append(latchJmp.Args, advancedPtr)
	_ = newParamIdx // used implicitly via append position matching
}

// computeInitialPointer inserts instructions before b's terminator to compute
// base + ext(initial_index) and returns the result register.
func computeInitialPointer(f *Func, b *Block, c *threadingCandidate) Reg {
	// Get the initial index value from the existing jump args.
	jmp := b.Term.(*TermJmp)
	initialIdx := jmp.Args[c.paramIdx]

	// addr_of base
	baseReg := f.AllocReg()
	b.Insts = append(b.Insts, &Inst{
		Op:  OpAddrOf,
		Dst: baseReg,
		Sym: findAddrOfSym(c.baseReg, f),
		Ty:  TyPtr,
		Cls: ClassPointer,
	})

	// ext index to u16 if needed
	wideIdx := initialIdx
	if c.baseTy == TyPtr { // always widen for pointer arithmetic
		wideIdx = f.AllocReg()
		b.Insts = append(b.Insts, &Inst{
			Op:    OpExt,
			Dst:   wideIdx,
			Src:   [2]Reg{initialIdx},
			SrcTy: TyU8,
			Ty:    TyU16,
		})
	}

	// ptr_add
	ptrReg := f.AllocReg()
	b.Insts = append(b.Insts, &Inst{
		Op:  OpPtrAdd,
		Dst: ptrReg,
		Src: [2]Reg{baseReg, wideIdx},
		Ty:  TyPtr,
		Cls: ClassPointer,
	})

	return ptrReg
}

// computeInitialPointerForBrIf handles the case where an entry to the loop
// header comes from a br_if edge. We insert computation in the source block.
func computeInitialPointerForBrIf(f *Func, b *Block, c *threadingCandidate, isThen bool) Reg {
	var args []Reg
	if isThen {
		args = b.Term.(*TermBrIf).ThenArgs
	} else {
		args = b.Term.(*TermBrIf).ElseArgs
	}

	if c.paramIdx >= len(args) {
		// No existing args — this edge doesn't pass an index.
		// Return a placeholder (should not happen in well-formed loops).
		return NoReg
	}

	initialIdx := args[c.paramIdx]

	baseReg := f.AllocReg()
	b.Insts = append(b.Insts, &Inst{
		Op:  OpAddrOf,
		Dst: baseReg,
		Sym: findAddrOfSym(c.baseReg, f),
		Ty:  TyPtr,
		Cls: ClassPointer,
	})

	wideIdx := f.AllocReg()
	b.Insts = append(b.Insts, &Inst{
		Op:    OpExt,
		Dst:   wideIdx,
		Src:   [2]Reg{initialIdx},
		SrcTy: TyU8,
		Ty:    TyU16,
	})

	ptrReg := f.AllocReg()
	b.Insts = append(b.Insts, &Inst{
		Op:  OpPtrAdd,
		Dst: ptrReg,
		Src: [2]Reg{baseReg, wideIdx},
		Ty:  TyPtr,
		Cls: ClassPointer,
	})

	return ptrReg
}

// findAddrOfSym finds the symbol name for an OpAddrOf instruction defining reg.
func findAddrOfSym(reg Reg, f *Func) string {
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Dst == reg && inst.Op == OpAddrOf {
				return inst.Sym
			}
		}
	}
	return ""
}
