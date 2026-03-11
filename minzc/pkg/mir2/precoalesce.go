package mir2

// PropagateClassHintsForTest exports propagateClassHints for external test packages.
func PropagateClassHintsForTest(f *Func, info map[Reg]RegInfo) {
	propagateClassHints(f, info)
}

// propagateClassHints propagates register class preferences through phi-web
// edges before PBQP. It runs two sub-passes:
//
//  1. Return-value promotion: registers used directly in TermRet/TermCondRet
//     are upgraded to ClassAcc (they must be in A on Z80).
//
//  2. Backward phi-web propagation: if a block parameter has a more-specific
//     class than an arg feeding it, the arg's class is upgraded.
//     Repeats to fixpoint so chains of length >1 are handled.
//
// This ensures that when an ALU op's result (ClassAcc) is threaded through
// block arguments to a return, PBQP sees ClassAcc on every node in the chain
// and assigns them all to A — avoiding LD C,A / LD A,C round-trips.
func propagateClassHints(f *Func, info map[Reg]RegInfo) {
	// ── Pass 1: mark return values as ClassAcc ────────────────────────────────
	for _, b := range f.Blocks {
		promoteRetVals := func(vals []Reg) {
			for _, v := range vals {
				if v == NoReg {
					continue
				}
				ri, ok := info[v]
				if !ok {
					continue
				}
				// Only upgrade scalar ints; bool/ptr/u16 have their own classes.
				if ri.Ty == TyBool || ri.Ty == TyPtr ||
					ri.Ty == TyU16 || ri.Ty == TyI16 {
					continue
				}
				if canUpgradeClass(ri.Cls, ClassAcc) {
					ri.Cls = ClassAcc
					info[v] = ri
				}
			}
		}
		switch t := b.Term.(type) {
		case *TermRet:
			promoteRetVals(t.Vals)
		case *TermCondRet:
			promoteRetVals(t.Vals)
		}
	}

	// ── Pass 2: backward phi-web propagation ─────────────────────────────────
	blockByLabel := make(map[string]*Block, len(f.Blocks))
	for _, b := range f.Blocks {
		blockByLabel[b.Label] = b
	}

	changed := true
	for changed {
		changed = false
		for _, b := range f.Blocks {
			propagateArgs := func(target string, args []Reg) {
				tb := blockByLabel[target]
				if tb == nil {
					return
				}
				for i, arg := range args {
					if i >= len(tb.Params) || arg == NoReg {
						break
					}
					paramInfo, ok := info[tb.Params[i].Dst]
					if !ok {
						continue
					}
					argInfo, ok := info[arg]
					if !ok {
						continue
					}
					// Upgrade arg's class if the param requires a more-specific class.
					if canUpgradeClass(argInfo.Cls, paramInfo.Cls) {
						argInfo.Cls = paramInfo.Cls
						info[arg] = argInfo
						changed = true
					}
				}
			}

			switch t := b.Term.(type) {
			case *TermJmp:
				propagateArgs(t.Target, t.Args)
			case *TermBrIf:
				propagateArgs(t.Then, t.ThenArgs)
				propagateArgs(t.Else, t.ElseArgs)
			case *TermBrIf2:
				propagateArgs(t.Eq, t.EqArgs)
				propagateArgs(t.Lt, t.LtArgs)
				propagateArgs(t.Gt, t.GtArgs)
			case *TermDJNZ:
				propagateArgs(t.Body, t.BodyArgs)
				propagateArgs(t.Exit, t.ExitArgs)
			case *TermCondRet:
				propagateArgs(t.Then, t.ThenArgs)
			}
		}
	}
}

// canUpgradeClass reports whether a register currently assigned `from` can be
// safely upgraded to `to` without restricting its use.  The only upgrade we
// perform is ClassGeneral → ClassAcc: ClassAcc is a subset of ClassGeneral
// (both map to A as the optimal location), so narrowing to ClassAcc never
// causes a new spill.
func canUpgradeClass(from, to RegClass) bool {
	return from == ClassGeneral && to == ClassAcc
}
