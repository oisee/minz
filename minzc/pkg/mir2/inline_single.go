package mir2

import "fmt"

// InlineSingleCallSites removes the size limit for private single-use bodies.
// Exported straight-line, parameterless leaf helpers may be copied, but their
// callable definitions are retained by the ordinary export/pruning roots.
func InlineSingleCallSites(m *Module, exported map[string]bool) bool {
	counts := map[string]int{}
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
	for _, f := range m.Funcs {
		if counts[f.Name] != 1 || address[f.Name] || f.Attrs.FixedIndirectABI || f.Attrs.IsExtern || f.Attrs.IsRecursive || f.Attrs.FallibleCY || len(f.Contract.StructReturnBuffers) > 0 || len(f.Blocks) == 0 || len(f.Entry().Params) > 0 || recursive(f.Name) {
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
			if len(f.Blocks) != 1 || len(f.Contract.Params) != 0 {
				safe = false
			}
			for _, b := range f.Blocks {
				for _, x := range b.Insts {
					if x.Op == OpCall || x.Op == OpCallCond || x.Op == OpCallIndirect || x.Op == OpStore || x.Volatile {
						safe = false
					}
				}
			}
		}
		if safe {
			eligible[f.Name] = f
		}
	}
	changed := false
	// Re-scan after each splice: a private single-use call chain can be expanded
	// without relying on declaration order. Recursion was excluded above.
	for {
		progress := false
		for _, caller := range m.Funcs {
			for _, block := range caller.Blocks {
				for index, call := range block.Insts {
					callee := eligible[call.Sym]
					if call.Op != OpCall || callee == nil || caller == callee || call.CallAttr.IsHost || len(call.Args) != len(callee.Contract.Params) {
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
