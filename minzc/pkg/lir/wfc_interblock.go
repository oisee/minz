// wfc_interblock.go — Multi-block WFC constraint propagation.
//
// Extends the single-block WFC (wfc.go) with inter-block edge propagation.
// Block arguments carry LocSet constraints across CFG edges so that
// caller and callee naturally agree on registers without parallel copies.
//
// Dimension 2: inter-block constraint propagation
//   - Intra-block: existing forward+backward+vreg-consistency (unchanged)
//   - Inter-block: edge arg LocSets propagate to/from block param LocSets
//   - Fixpoint iteration handles loop back-edges (capped at 10 rounds)
//   - Global collapse in RPO order (predecessors before successors)
package lir

import "fmt"

// ProgWFC manages WFC state across all blocks of a Prog.
type ProgWFC struct {
	Desc     *MachineDesc
	Prog     *Prog
	States   map[string]*WFCState    // label → per-block WFC state
	ParamMap map[string][]BlockParam  // label → current param constraints
}

// NewProgWFC creates a ProgWFC from a Prog whose blocks already have
// isel'd instructions (Insts populated) and Params set.
func NewProgWFC(prog *Prog) *ProgWFC {
	pw := &ProgWFC{
		Desc:     prog.Desc,
		Prog:     prog,
		States:   make(map[string]*WFCState, len(prog.Blocks)),
		ParamMap: make(map[string][]BlockParam, len(prog.Blocks)),
	}

	for i := range prog.Blocks {
		b := &prog.Blocks[i]
		wfc := NewWFCStateWithParams(prog.Desc, b.Insts, b.Params)
		AddTermUses(wfc, &b.Term)
		pw.States[b.Label] = wfc
		// Copy params for tracking
		params := make([]BlockParam, len(b.Params))
		copy(params, b.Params)
		pw.ParamMap[b.Label] = params
	}

	return pw
}

// NewWFCStateWithParams creates a WFC state that includes block params
// as initial vreg constraints. Params define vregs available on block entry.
// Synthetic "param def" cells are prepended so the WFC collapse sees them
// as definitions and assigns distinct physical registers to each param.
func NewWFCStateWithParams(desc *MachineDesc, insts []Inst, params []BlockParam) *WFCState {
	// Prepend synthetic cells for params. These look like const definitions
	// (one cell per param) so the collapse assigns distinct physregs.
	paramCells := make([]WFCCell, len(params))
	for i, p := range params {
		allowed := p.Allowed
		if allowed.IsEmpty() {
			allowed = desc.LocsOfWidth(desc.WordSize)
		}
		paramCells[i] = WFCCell{
			Pat:     nil, // no real pattern — just a placeholder def
			DstLocs: allowed,
			VRegDst: p.VReg,
			VRegSrc: [2]int{-1, -1},
		}
	}

	wfc := NewWFCState(desc, insts)

	// Prepend param cells before instruction cells.
	allCells := make([]WFCCell, 0, len(paramCells)+len(wfc.Cells))
	allCells = append(allCells, paramCells...)
	allCells = append(allCells, wfc.Cells...)
	wfc.Cells = allCells

	// Pre-constrain: narrow instruction sources that use param vregs.
	for _, p := range params {
		if p.VReg < 0 {
			continue
		}
		for i := range wfc.Cells {
			c := &wfc.Cells[i]
			for s := 0; s < 2; s++ {
				if c.VRegSrc[s] == p.VReg && !p.Allowed.IsEmpty() {
					narrowed := c.SrcLocs[s].And(p.Allowed)
					if !narrowed.IsEmpty() {
						c.SrcLocs[s] = narrowed
					}
				}
			}
		}
	}

	return wfc
}

// AddTermUses appends synthetic "use" cells for vregs referenced in a block's
// terminator (cond, counter, args, ret vals). This ensures the WFC collapse
// keeps those vregs live until the end of the block.
func AddTermUses(wfc *WFCState, term *Term) {
	addUse := func(vreg int, allowed LocSet) {
		if vreg < 0 {
			return
		}
		if allowed.IsEmpty() {
			allowed = LocSet(0) // unconstrained
		}
		wfc.Cells = append(wfc.Cells, WFCCell{
			Pat:     nil, // synthetic use — no real instruction
			VRegDst: -1,
			VRegSrc: [2]int{vreg, -1},
			SrcLocs: [2]LocSet{allowed, 0},
		})
	}

	if term.Cond.VReg > 0 {
		addUse(term.Cond.VReg, term.Cond.Allowed)
	}
	if term.Counter.VReg > 0 {
		addUse(term.Counter.VReg, term.Counter.Allowed)
	}
	for _, edgeArgs := range term.Args {
		for _, arg := range edgeArgs {
			addUse(arg.VReg, arg.Allowed)
		}
	}
	for _, rv := range term.RetVals {
		addUse(rv.VReg, rv.Allowed)
	}
}

// Propagate runs intra-block + inter-block constraint propagation to fixpoint.
// Returns total iteration count.
func (pw *ProgWFC) Propagate() int {
	totalIters := 0

	for round := 0; round < 10; round++ {
		changed := false

		// Step 1: Intra-block propagation for each block
		for _, b := range pw.Prog.Blocks {
			wfc := pw.States[b.Label]
			iters := wfc.Propagate()
			if iters > 1 { // >1 means changes happened beyond first pass
				changed = true
			}
		}

		// Step 2: Inter-block edge propagation
		if pw.propagateEdges() {
			changed = true
		}

		totalIters++
		if !changed {
			break
		}
	}

	return totalIters
}

// propagateEdges propagates constraints along CFG edges:
//   - Forward: arg LocSet → target param LocSet (intersection)
//   - Backward: param LocSet → arg LocSet (intersection)
func (pw *ProgWFC) propagateEdges() bool {
	changed := false

	for bi := range pw.Prog.Blocks {
		b := &pw.Prog.Blocks[bi]
		term := &b.Term

		for edgeIdx, target := range term.Targets {
			if target == "" {
				continue
			}
			targetParams := pw.ParamMap[target]
			if len(targetParams) == 0 {
				continue
			}
			if edgeIdx >= len(term.Args) {
				continue
			}
			args := term.Args[edgeIdx]

			n := len(args)
			if n > len(targetParams) {
				n = len(targetParams)
			}

			for i := 0; i < n; i++ {
				argAllowed := args[i].Allowed
				paramAllowed := targetParams[i].Allowed

				if argAllowed.IsEmpty() || paramAllowed.IsEmpty() {
					continue
				}

				// Forward: narrow param to intersect with arg
				narrowedParam := paramAllowed.And(argAllowed)
				if narrowedParam != paramAllowed && !narrowedParam.IsEmpty() {
					pw.ParamMap[target][i].Allowed = narrowedParam
					// Also push into the target block's WFC state
					pw.constrainParamInBlock(target, targetParams[i].VReg, narrowedParam)
					changed = true
				}

				// Backward: narrow arg to intersect with param
				narrowedArg := argAllowed.And(paramAllowed)
				if narrowedArg != argAllowed && !narrowedArg.IsEmpty() {
					pw.Prog.Blocks[bi].Term.Args[edgeIdx][i].Allowed = narrowedArg
					// Also push into the source block's WFC state
					pw.constrainVRegInBlock(b.Label, args[i].VReg, narrowedArg)
					changed = true
				}
			}
		}
	}

	return changed
}

// constrainParamInBlock narrows the LocSet for a param vreg in a block's WFC state.
func (pw *ProgWFC) constrainParamInBlock(label string, vreg int, allowed LocSet) {
	wfc := pw.States[label]
	if wfc == nil || vreg < 0 {
		return
	}
	for i := range wfc.Cells {
		c := &wfc.Cells[i]
		for s := 0; s < 2; s++ {
			if c.VRegSrc[s] == vreg {
				narrowed := c.SrcLocs[s].And(allowed)
				if !narrowed.IsEmpty() {
					c.SrcLocs[s] = narrowed
				}
			}
		}
	}
}

// constrainVRegInBlock narrows the LocSet for a vreg's definition in a block's WFC state.
func (pw *ProgWFC) constrainVRegInBlock(label string, vreg int, allowed LocSet) {
	wfc := pw.States[label]
	if wfc == nil || vreg < 0 {
		return
	}
	for i := range wfc.Cells {
		c := &wfc.Cells[i]
		if c.VRegDst == vreg {
			narrowed := c.DstLocs.And(allowed)
			if !narrowed.IsEmpty() {
				c.DstLocs = narrowed
			}
		}
	}
}

// Collapse assigns physical locations in RPO order.
// Predecessors get collapsed before successors so that param assignments
// are known when processing successor blocks.
func (pw *ProgWFC) Collapse() error {
	order := pw.rpo()

	// Global vreg → phys tracking (shared across blocks for params).
	globalPhys := make(map[int]int)

	for _, label := range order {
		wfc := pw.States[label]
		if wfc == nil {
			continue
		}

		// Before collapsing, pin any param vregs that were already assigned
		// by a predecessor's edge args.
		params := pw.ParamMap[label]
		for _, p := range params {
			if phys, ok := globalPhys[p.VReg]; ok {
				pw.pinVRegInWFC(wfc, p.VReg, phys)
			}
		}

		// Also pin any vregs that are edge args to targets whose params
		// already have phys assignments (back-edge constraint propagation).
		bi := pw.blockIndex(label)
		if bi >= 0 {
			b := &pw.Prog.Blocks[bi]
			for edgeIdx, target := range b.Term.Targets {
				if target == "" || edgeIdx >= len(b.Term.Args) {
					continue
				}
				args := b.Term.Args[edgeIdx]
				targetParams := pw.ParamMap[target]
				n := len(args)
				if n > len(targetParams) {
					n = len(targetParams)
				}
				for i := 0; i < n; i++ {
					argVReg := args[i].VReg
					paramVReg := targetParams[i].VReg
					if argVReg < 0 || paramVReg < 0 {
						continue
					}
					// If target param already has a phys, pin arg vreg to it
					if paramPhys, ok := globalPhys[paramVReg]; ok {
						if _, argHas := globalPhys[argVReg]; !argHas {
							globalPhys[argVReg] = paramPhys
							pw.pinVRegInWFC(wfc, argVReg, paramPhys)
						}
					}
				}
			}
		}

		// Collapse this block
		if err := wfc.Collapse(); err != nil {
			return fmt.Errorf("block %s: %w", label, err)
		}

		// Record assignments: param vregs
		for i, p := range params {
			if p.VReg < 0 {
				continue
			}
			phys := pw.findVRegPhys(wfc, p.VReg)
			if phys >= 0 {
				pw.assignPhys(globalPhys, p.VReg, phys)
				pw.ParamMap[label][i].Phys = phys
			}
		}

		// Record assignments: all dst vregs defined in this block
		for i := range wfc.Cells {
			c := &wfc.Cells[i]
			if c.VRegDst >= 0 {
				phys := PhysOf(c.DstLocs)
				if phys >= 0 {
					pw.assignPhys(globalPhys, c.VRegDst, phys)
				}
			}
		}

		// Propagate assignments across outgoing edges (bidirectional).
		b := &pw.Prog.Blocks[pw.blockIndex(label)]
		for edgeIdx, target := range b.Term.Targets {
			if target == "" || edgeIdx >= len(b.Term.Args) {
				continue
			}
			args := b.Term.Args[edgeIdx]
			targetParams := pw.ParamMap[target]
			n := len(args)
			if n > len(targetParams) {
				n = len(targetParams)
			}
			for i := 0; i < n; i++ {
				argVReg := args[i].VReg
				paramVReg := targetParams[i].VReg
				if argVReg < 0 || paramVReg < 0 {
					continue
				}
				argPhys, argOk := globalPhys[argVReg]
				paramPhys, paramOk := globalPhys[paramVReg]

				if argOk && !paramOk {
					// Forward: arg → param
					globalPhys[paramVReg] = argPhys
				} else if !argOk && paramOk {
					// Backward (back-edge): param → arg
					globalPhys[argVReg] = paramPhys
				}
				// If both assigned and different, that's a parallel copy needed.
			}
		}
	}

	// Second pass: propagate back-edge constraints and re-collapse if needed.
	// After forward RPO collapse, some arg vregs may not match their target
	// params because the target was processed earlier (back-edge).
	for round := 0; round < 3; round++ {
		changed := false
		for _, label := range order {
			bi := pw.blockIndex(label)
			if bi < 0 {
				continue
			}
			b := &pw.Prog.Blocks[bi]
			wfc := pw.States[label]
			if wfc == nil {
				continue
			}

			needRecollapse := false
			for edgeIdx, target := range b.Term.Targets {
				if target == "" || edgeIdx >= len(b.Term.Args) {
					continue
				}
				args := b.Term.Args[edgeIdx]
				targetParams := pw.ParamMap[target]
				n := len(args)
				if n > len(targetParams) {
					n = len(targetParams)
				}
				for i := 0; i < n; i++ {
					argVReg := args[i].VReg
					paramVReg := targetParams[i].VReg
					if argVReg < 0 || paramVReg < 0 {
						continue
					}
					paramPhys, paramOk := globalPhys[paramVReg]
					argPhys, argOk := globalPhys[argVReg]
					if paramOk && (!argOk || argPhys != paramPhys) {
						// Pin arg vreg to param's phys
						globalPhys[argVReg] = paramPhys
						pw.pinVRegInWFC(wfc, argVReg, paramPhys)
						needRecollapse = true
						changed = true
					}
				}
			}

			if needRecollapse {
				// Re-collapse with new constraints
				if err := wfc.Collapse(); err != nil {
					return fmt.Errorf("re-collapse block %s: %w", label, err)
				}
				// Re-record assignments
				for i := range wfc.Cells {
					c := &wfc.Cells[i]
					if c.VRegDst >= 0 {
						phys := PhysOf(c.DstLocs)
						if phys >= 0 {
							pw.assignPhys(globalPhys, c.VRegDst, phys)
						}
					}
				}
			}
		}
		if !changed {
			break
		}
	}

	// Patch edge args with physical assignments
	for bi := range pw.Prog.Blocks {
		b := &pw.Prog.Blocks[bi]
		for edgeIdx := range b.Term.Args {
			for ai := range b.Term.Args[edgeIdx] {
				arg := &b.Term.Args[edgeIdx][ai]
				if arg.VReg >= 0 {
					if phys, ok := globalPhys[arg.VReg]; ok {
						arg.Phys = phys
						arg.Allowed = Singleton(phys)
					}
				}
			}
		}
		// Patch cond/counter operands
		if b.Term.Cond.VReg >= 0 {
			if phys, ok := globalPhys[b.Term.Cond.VReg]; ok {
				b.Term.Cond.Phys = phys
			}
		}
		if b.Term.Counter.VReg >= 0 {
			if phys, ok := globalPhys[b.Term.Counter.VReg]; ok {
				b.Term.Counter.Phys = phys
			}
		}
		// Patch return values
		for ri := range b.Term.RetVals {
			rv := &b.Term.RetVals[ri]
			if rv.VReg >= 0 {
				if phys, ok := globalPhys[rv.VReg]; ok {
					rv.Phys = phys
				}
			}
		}
	}

	// Sync ParamMap phys with globalPhys (second pass may have updated)
	for label, params := range pw.ParamMap {
		for i := range params {
			if params[i].VReg >= 0 {
				if phys, ok := globalPhys[params[i].VReg]; ok {
					pw.ParamMap[label][i].Phys = phys
				}
			}
		}
	}

	// Write back collapsed instructions to Prog blocks
	for i := range pw.Prog.Blocks {
		b := &pw.Prog.Blocks[i]
		wfc := pw.States[b.Label]
		if wfc == nil {
			continue
		}
		// Filter out synthetic cells (nil Pat) before writing back
		realInsts := make([]Inst, 0, len(wfc.Cells))
		for _, c := range wfc.Cells {
			if c.Pat != nil {
				realInsts = append(realInsts, Inst{
					Pat: c.Pat,
					Imm: c.Imm,
					Sym: c.Sym,
					Dst: Operand{VReg: c.VRegDst, Allowed: c.DstLocs, Phys: PhysOf(c.DstLocs)},
					Srcs: [2]Operand{
						{VReg: c.VRegSrc[0], Allowed: c.SrcLocs[0], Phys: PhysOf(c.SrcLocs[0])},
						{VReg: c.VRegSrc[1], Allowed: c.SrcLocs[1], Phys: PhysOf(c.SrcLocs[1])},
					},
				})
			}
		}
		b.Insts = realInsts
		// Write back param phys
		for pi := range b.Params {
			b.Params[pi] = pw.ParamMap[b.Label][pi]
		}
	}

	return nil
}

// findVRegPhys looks up the physical assignment for a vreg in a WFC state.
func (pw *ProgWFC) findVRegPhys(wfc *WFCState, vreg int) int {
	// Check sources first (params are consumed as sources)
	for i := range wfc.Cells {
		c := &wfc.Cells[i]
		for s := 0; s < 2; s++ {
			if c.VRegSrc[s] == vreg {
				phys := PhysOf(c.SrcLocs[s])
				if phys >= 0 {
					return phys
				}
			}
		}
		if c.VRegDst == vreg {
			phys := PhysOf(c.DstLocs)
			if phys >= 0 {
				return phys
			}
		}
	}
	return -1
}

// assignPhys records a phys assignment for a vreg.
func (pw *ProgWFC) assignPhys(globalPhys map[int]int, vreg int, phys int) {
	globalPhys[vreg] = phys
}

// pinVRegInWFC constrains a vreg to a specific physical register in a WFC state.
func (pw *ProgWFC) pinVRegInWFC(wfc *WFCState, vreg int, phys int) {
	loc := Singleton(phys)
	for i := range wfc.Cells {
		c := &wfc.Cells[i]
		for s := 0; s < 2; s++ {
			if c.VRegSrc[s] == vreg {
				c.SrcLocs[s] = loc
			}
		}
		if c.VRegDst == vreg {
			c.DstLocs = loc
		}
	}
}

// blockIndex returns the index of a block by label.
func (pw *ProgWFC) blockIndex(label string) int {
	for i, b := range pw.Prog.Blocks {
		if b.Label == label {
			return i
		}
	}
	return -1
}

// rpo returns block labels in reverse post-order.
func (pw *ProgWFC) rpo() []string {
	if len(pw.Prog.Blocks) == 0 {
		return nil
	}

	labelIdx := make(map[string]int, len(pw.Prog.Blocks))
	for i, b := range pw.Prog.Blocks {
		labelIdx[b.Label] = i
	}

	visited := make(map[string]bool)
	var order []string

	var dfs func(label string)
	dfs = func(label string) {
		if visited[label] {
			return
		}
		visited[label] = true

		idx, ok := labelIdx[label]
		if !ok {
			return
		}
		b := &pw.Prog.Blocks[idx]
		for _, target := range b.Term.Targets {
			if target != "" {
				dfs(target)
			}
		}
		order = append(order, label)
	}

	dfs(pw.Prog.Blocks[0].Label)

	// Include any unreachable blocks
	for _, b := range pw.Prog.Blocks {
		if !visited[b.Label] {
			order = append(order, b.Label)
		}
	}

	// Reverse for RPO
	for i, j := 0, len(order)-1; i < j; i, j = i+1, j-1 {
		order[i], order[j] = order[j], order[i]
	}
	return order
}

// ParallelCopyCount counts the number of edge crossings where
// arg.Phys != param.Phys (requiring a parallel copy move).
func (pw *ProgWFC) ParallelCopyCount() int {
	count := 0
	for bi := range pw.Prog.Blocks {
		b := &pw.Prog.Blocks[bi]
		for edgeIdx, target := range b.Term.Targets {
			if target == "" || edgeIdx >= len(b.Term.Args) {
				continue
			}
			params := pw.ParamMap[target]
			args := b.Term.Args[edgeIdx]
			n := len(args)
			if n > len(params) {
				n = len(params)
			}
			for i := 0; i < n; i++ {
				if args[i].Phys >= 0 && params[i].Phys >= 0 && args[i].Phys != params[i].Phys {
					count++
				}
			}
		}
	}
	return count
}
