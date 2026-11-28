package mir2

import "fmt"

type SingleCallInlineStats struct {
	DisableExportCopies bool
	KeepCallers         map[string]bool
	InlinedCallers      map[string]bool
	ExportCopies        int
	// ReportSpeedCandidates fills SpeedCandidates: the small loop-free
	// exported leaves (ExportSpeedCopyMaxInsts, ExportSpeedCopyMaxBlocks)
	// whose copy into their single caller passes the speed pre-filter
	// (speedCopyCallerOK). Reporting copies nothing.
	ReportSpeedCandidates bool
	SpeedCandidates       map[string]string // callee -> caller
	// SpeedCopyAllow lists the speed candidates to copy (the pipeline's
	// decision, export_speed_copy.go). A copy always grows the module, since
	// the callable definition is retained, so nothing else may copy them.
	SpeedCopyAllow map[string]bool
	// SpeedCopies records each speed copy made: callee name -> caller name.
	SpeedCopies map[string]string
	// DisableNarrowCopies refuses the parameterless straight-line export
	// copies, so a speed-copy compile reproduces a reference product whose
	// byte gate refused them.
	DisableNarrowCopies bool
}

// Bounds for a speed-only export copy: the copied body is at most this many
// MIR instructions (terminators excluded, counted on the lowered MIR the
// single-call inliner sees, before constant folding) in at most this many
// blocks.
const (
	ExportSpeedCopyMaxInsts  = 16
	ExportSpeedCopyMaxBlocks = 8
)

// acyclicBlocks reports whether f's CFG has no cycle (a loop-free leaf runs
// each block at most once per call, so its copy executes no more often).
func acyclicBlocks(f *Func) bool {
	state := map[string]int{}
	var visit func(label string) bool
	visit = func(label string) bool {
		switch state[label] {
		case 1:
			return false
		case 2:
			return true
		}
		state[label] = 1
		if b := f.BlockByLabel(label); b != nil && b.Term != nil {
			for _, s := range b.Term.Successors() {
				if !visit(s) {
					return false
				}
			}
		}
		state[label] = 2
		return true
	}
	return visit(f.Entry().Label)
}

// speedCopyCallerOK is the speed pre-filter, decided on the MIR before any
// copy: the pipeline accepts a speed copy only when the static worst-path
// model scores the caller after the copy (export_speed_copy.go), and that
// model refuses any other call (it cannot cost a callee it does not see) and
// any loop without a proved trip count. So the caller must contain exactly
// one call instruction, the copied site, and every cycle of its CFG must
// have a static loop fact with a known trip count. A caller that is itself
// inlined into another function would carry the copy elsewhere (the model
// would score a function that does not run), so it must not be eligible
// for single-call inlining either (checked by the caller of this helper).
func speedCopyCallerOK(caller *Func) bool {
	calls := 0
	for _, b := range caller.Blocks {
		for _, x := range b.Insts {
			if x.Op == OpCall || x.Op == OpCallCond || x.Op == OpCallIndirect {
				calls++
			}
			if x.Op == OpAsm || x.Op == OpPatch || x.Op == OpPatchSlot || x.Op == OpLoadPatched {
				return false
			}
		}
	}
	if calls != 1 {
		return false
	}
	state := map[string]int{}
	ok := true
	var visit func(label string)
	visit = func(label string) {
		state[label] = 1
		if b := caller.BlockByLabel(label); b != nil && b.Term != nil {
			for _, s := range b.Term.Successors() {
				switch state[s] {
				case 1: // back edge: s heads a cycle
					if fact, known := caller.StaticLoops[s]; !known || fact.Trips < 0 {
						ok = false
					}
				case 0:
					visit(s)
				}
			}
		}
		state[label] = 2
	}
	if len(caller.Blocks) > 0 {
		visit(caller.Entry().Label)
	}
	return ok
}

// InlineSingleCallSites removes the size limit for private single-use bodies.
// Exported straight-line, parameterless leaf helpers may be copied, but their
// callable definitions are retained by the ordinary export/pruning roots.
// A small loop-free exported leaf passing the speed pre-filter is reported in
// SpeedCandidates and copied only when SpeedCopyAllow names it; such copies
// are reported in SpeedCopies (see SingleCallInlineStats).
func InlineSingleCallSites(m *Module, exported map[string]bool, stats ...*SingleCallInlineStats) bool {
	counts := map[string]int{}
	callerOf := map[string]*Func{}
	address := map[string]bool{}
	opaque := false
	for _, g := range m.Globals {
		for _, r := range g.Relocs {
			address[r.Symbol] = true
		}
		for _, s := range g.VtableSyms {
			address[s] = true
		}
	}
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, x := range b.Insts {
				if x.Op == OpCall || x.Op == OpCallCond {
					counts[x.Sym]++
					callerOf[x.Sym] = f
				}
				if x.Op == OpAddrOf {
					address[x.Sym] = true
				}
				if x.Op == OpAsm {
					opaque = true
				}
			}
		}
	}
	if opaque {
		return false
	} // opaque assembly can add static callers/addresses
	recursive := func(start string) bool {
		seen := map[string]bool{}
		var visit func(string) bool
		visit = func(name string) bool {
			if seen[name] {
				return false
			}
			seen[name] = true
			f := m.FuncByName(name)
			if f == nil {
				return false
			}
			for _, b := range f.Blocks {
				for _, x := range b.Insts {
					if x.Op == OpCall || x.Op == OpCallCond {
						if x.Sym == start || visit(x.Sym) {
							return true
						}
					}
				}
			}
			return false
		}
		return visit(start)
	}
	eligible := map[string]*Func{}
	speedShape := map[string]bool{}
	for _, f := range m.Funcs {
		// Absence from the pruning roots is not evidence of private linkage.
		// Runtime helpers may be Internal yet callable by a provider's name.
		if f.Attrs.ReferencedByName || !f.Attrs.Private && !exported[f.Name] {
			continue
		}
		if counts[f.Name] != 1 || address[f.Name] || f.Attrs.FixedIndirectABI || f.Attrs.IsExtern || f.Attrs.IsComptime || f.Attrs.IsRecursive || f.Attrs.FallibleCY || len(f.Contract.StructReturnBuffers) > 0 || len(f.Blocks) == 0 || len(f.Entry().Params) > 0 || recursive(f.Name) {
			continue
		}
		safe := true
		for _, p := range f.Contract.Params {
			if p.ClassHard || p.Class == ClassFlag {
				safe = false
			}
		}
		for _, r := range f.Contract.Returns {
			if r.Class == ClassFlag {
				safe = false
			}
		}
		for _, b := range f.Blocks {
			for _, x := range b.Insts {
				if x.SequentialBytes || x.ByteLoop != nil || x.FallibleCY || x.ClsHard {
					safe = false
				}
				switch x.Op {
				case OpAlloca, OpAsm, OpPatch, OpPatchSlot, OpLoadPatched, OpPush, OpPop:
					safe = false
				}
			}
			switch b.Term.(type) {
			case *TermJmp, *TermBrIf, *TermBrIf2, *TermDJNZ, *TermCondRet, *TermRet, *TermUnreachable:
			default:
				safe = false
			}
		}
		if exported[f.Name] {
			if len(stats) > 0 && stats[0] != nil && stats[0].DisableExportCopies {
				continue
			}
			narrow := len(f.Blocks) == 1 && len(f.Contract.Params) == 0
			insts := 0
			for _, b := range f.Blocks {
				insts += len(b.Insts)
				for _, x := range b.Insts {
					if x.Op == OpCall || x.Op == OpCallCond || x.Op == OpCallIndirect || x.Op == OpStore || x.Volatile {
						safe = false
					}
				}
			}
			if narrow && len(stats) > 0 && stats[0] != nil && stats[0].DisableNarrowCopies {
				safe = false
			}
			if !narrow {
				if !safe || len(stats) == 0 || stats[0] == nil ||
					len(f.Blocks) > ExportSpeedCopyMaxBlocks || insts > ExportSpeedCopyMaxInsts || len(f.StaticLoops) > 0 || !acyclicBlocks(f) {
					safe = false
				} else {
					speedShape[f.Name] = true
					safe = stats[0].SpeedCopyAllow[f.Name]
				}
			}
		}
		if safe {
			eligible[f.Name] = f
		}
	}
	// The speed pre-filter needs the complete eligibility: a caller that is
	// itself inlined would move the copy into another function.
	for _, name := range sortedNames(speedShape) {
		caller := callerOf[name]
		pass := caller != nil && caller.Name != name && eligible[caller.Name] == nil && !speedShape[caller.Name] &&
			!(stats[0].KeepCallers[caller.Name]) && speedCopyCallerOK(caller)
		if !pass {
			delete(eligible, name)
			continue
		}
		if stats[0].ReportSpeedCandidates {
			if stats[0].SpeedCandidates == nil {
				stats[0].SpeedCandidates = map[string]string{}
			}
			stats[0].SpeedCandidates[name] = caller.Name
		}
	}
	changed := false
	// Re-scan after each splice: a private single-use call chain can be expanded
	// without relying on declaration order. Recursion was excluded above.
	for {
		progress := false
		for _, caller := range m.Funcs {
			if len(stats) > 0 && stats[0] != nil && stats[0].KeepCallers[caller.Name] {
				continue
			}
			for _, block := range caller.Blocks {
				for index, call := range block.Insts {
					callee := eligible[call.Sym]
					if call.Op != OpCall || callee == nil || caller == callee || call.CallAttr.IsHost || call.CallAttr.IsComptime || len(call.Args) != len(callee.Contract.Params) {
						continue
					}
					pendingDependency := false
					for _, cb := range callee.Blocks {
						for _, ci := range cb.Insts {
							if (ci.Op == OpCall || ci.Op == OpCallCond) && eligible[ci.Sym] != nil {
								pendingDependency = true
							}
						}
					}
					if pendingDependency {
						continue
					}
					if len(stats) > 0 && stats[0] != nil && stats[0].InlinedCallers != nil {
						stats[0].InlinedCallers[caller.Name] = true
					}
					if len(callee.Blocks) == 1 {
						if _, ok := callee.Entry().Term.(*TermRet); ok {
							body := inlineOneCall(caller, callee, call)
							next := append([]*Inst{}, block.Insts[:index]...)
							next = append(next, body...)
							next = append(next, block.Insts[index+1:]...)
							block.Insts = next
						} else {
							inlineCFGCall(caller, block, index, callee, call)
						}
					} else {
						inlineCFGCall(caller, block, index, callee, call)
					}
					localInlineCSE(m, caller)
					if exported[callee.Name] && len(stats) > 0 && stats[0] != nil {
						stats[0].ExportCopies++
						if speedShape[callee.Name] {
							if stats[0].SpeedCopies == nil {
								stats[0].SpeedCopies = map[string]string{}
							}
							stats[0].SpeedCopies[callee.Name] = caller.Name
						}
					}
					delete(eligible, callee.Name)
					progress = true
					changed = true
					break
				}
				if progress {
					break
				}
			}
			if progress {
				break
			}
		}
		if !progress {
			break
		}
	}
	return changed
}

func inlineCFGCall(caller *Func, site *Block, index int, callee *Func, call *Inst) {
	prefix := fmt.Sprintf("inl_%d_", caller.AllocReg())
	regMap := map[Reg]Reg{}
	for i, p := range callee.Contract.Params {
		regMap[p.Reg] = call.Args[i]
	}
	allocate := func(r Reg) {
		if r != NoReg {
			if _, ok := regMap[r]; !ok {
				regMap[r] = caller.AllocReg()
			}
		}
	}
	labels := map[string]string{}
	for _, b := range callee.Blocks {
		labels[b.Label] = prefix + b.Label
		for _, p := range b.Params {
			allocate(p.Dst)
		}
		for _, x := range b.Insts {
			allocate(x.Dst)
			for _, r := range x.ExtraRets {
				allocate(r)
			}
		}
	}
	mr := func(r Reg) Reg { return remapReg(r, regMap) }
	regs := func(rs []Reg) []Reg {
		if rs == nil {
			return nil
		}
		out := make([]Reg, len(rs))
		for i, r := range rs {
			out[i] = mr(r)
		}
		return out
	}
	cont := &Block{Label: prefix + "continue", Insts: append([]*Inst(nil), site.Insts[index+1:]...), Term: site.Term}
	destinations := append([]Reg{call.Dst}, call.ExtraRets...)
	for i, r := range destinations {
		if r != NoReg {
			cont.Params = append(cont.Params, BlockParam{Dst: r, Ty: retTy(callee, i), Class: retCls(callee, i)})
		}
	}
	returns := func(vals []Reg) []Reg {
		var out []Reg
		for i, r := range destinations {
			if r != NoReg && i < len(vals) {
				out = append(out, mr(vals[i]))
			}
		}
		return out
	}
	var clones []*Block
	for _, old := range callee.Blocks {
		b := &Block{Label: labels[old.Label]}
		for _, p := range old.Params {
			p.Dst = mr(p.Dst)
			b.Params = append(b.Params, p)
		}
		for _, x := range old.Insts {
			n := remapInst(x, regMap)
			n.ExtraRets = regs(x.ExtraRets)
			b.Insts = append(b.Insts, n)
		}
		switch t := old.Term.(type) {
		case *TermRet:
			b.Term = &TermJmp{Target: cont.Label, Args: returns(t.Vals)}
		case *TermJmp:
			b.Term = &TermJmp{Target: labels[t.Target], Args: regs(t.Args)}
		case *TermBrIf:
			b.Term = &TermBrIf{Cond: mr(t.Cond), Then: labels[t.Then], ThenArgs: regs(t.ThenArgs), Else: labels[t.Else], ElseArgs: regs(t.ElseArgs)}
		case *TermBrIf2:
			b.Term = &TermBrIf2{Lhs: mr(t.Lhs), Rhs: mr(t.Rhs), Eq: labels[t.Eq], EqArgs: regs(t.EqArgs), Lt: labels[t.Lt], LtArgs: regs(t.LtArgs), Gt: labels[t.Gt], GtArgs: regs(t.GtArgs)}
		case *TermDJNZ:
			b.Term = &TermDJNZ{Counter: mr(t.Counter), Body: labels[t.Body], BodyArgs: regs(t.BodyArgs), Exit: labels[t.Exit], ExitArgs: regs(t.ExitArgs)}
		case *TermCondRet:
			ret := &Block{Label: prefix + old.Label + "_return", Term: &TermJmp{Target: cont.Label, Args: returns(t.Vals)}}
			clones = append(clones, ret)
			b.Term = &TermBrIf{Cond: mr(t.Cond), Then: labels[t.Then], ThenArgs: regs(t.ThenArgs), Else: ret.Label}
		case *TermUnreachable:
			b.Term = &TermUnreachable{}
		}
		if origin, ok := callee.LoopOrigins[old.Label]; ok {
			if caller.LoopOrigins == nil {
				caller.LoopOrigins = map[string]LoopOrigin{}
			}
			origin.SiteID = prefix + origin.SiteID
			caller.LoopOrigins[b.Label] = origin
		}
		clones = append(clones, b)
	}
	site.Insts = site.Insts[:index]
	site.Term = &TermJmp{Target: labels[callee.Entry().Label]}
	caller.Blocks = append(caller.Blocks, clones...)
	caller.Blocks = append(caller.Blocks, cont)
}
