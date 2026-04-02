package mir2

// PruneUnreachableFuncs removes functions that are not reachable from the
// explicit root set after whole-module rewrites such as inlining.
//
// In addition to the caller-provided roots, any function whose address is
// taken via OpAddrOf is treated conservatively as reachable.
func PruneUnreachableFuncs(m *Module, roots map[string]bool) bool {
	if len(m.Funcs) == 0 {
		return false
	}

	reachable := make(map[string]bool)
	work := make([]string, 0, len(roots))

	enqueue := func(name string) {
		if name == "" || reachable[name] || m.FuncByName(name) == nil {
			return
		}
		reachable[name] = true
		work = append(work, name)
	}

	for name := range roots {
		enqueue(name)
	}

	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				if inst.Op == OpAddrOf && inst.Sym != "" && m.FuncByName(inst.Sym) != nil {
					enqueue(inst.Sym)
				}
			}
		}
	}

	for len(work) > 0 {
		name := work[len(work)-1]
		work = work[:len(work)-1]

		f := m.FuncByName(name)
		if f == nil {
			continue
		}
		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				if inst.Op == OpCall && inst.Sym != "" && inst.Sym[0] != '@' {
					enqueue(inst.Sym)
					continue
				}
				if inst.Op == OpAddrOf && inst.Sym != "" && m.FuncByName(inst.Sym) != nil {
					enqueue(inst.Sym)
				}
			}
		}
	}

	kept := make([]*Func, 0, len(m.Funcs))
	changed := false
	for _, f := range m.Funcs {
		if reachable[f.Name] {
			kept = append(kept, f)
			continue
		}
		changed = true
	}
	if changed {
		m.Funcs = kept
	}
	return changed
}
