package mir2

// RenumberRegs walks every function in the module and remaps all virtual
// register numbers to be globally unique, starting from 1.
//
// This is necessary after passes like InlineTrivial that allocate fresh regs
// in existing functions — those regs may collide with later functions' ranges
// that were assigned during LowerModule before inlining.
//
// The combined AllocResult (one flat map[Reg]PhysLoc for all functions) relies
// on every function's regs being unique.
func (m *Module) RenumberRegs() {
	next := Reg(1)
	for _, f := range m.Funcs {
		remap := make(map[Reg]Reg)

		// Collect all regs used in this function.
		for _, p := range f.Contract.Params {
			if p.Reg != NoReg {
				if _, ok := remap[p.Reg]; !ok {
					remap[p.Reg] = next
					next++
				}
			}
		}
		for _, b := range f.Blocks {
			for _, bp := range b.Params {
				if bp.Dst != NoReg {
					if _, ok := remap[bp.Dst]; !ok {
						remap[bp.Dst] = next
						next++
					}
				}
			}
			for _, inst := range b.Insts {
				if inst.Dst != NoReg {
					if _, ok := remap[inst.Dst]; !ok {
						remap[inst.Dst] = next
						next++
					}
				}
				for _, s := range inst.Src {
					if s != NoReg {
						if _, ok := remap[s]; !ok {
							remap[s] = next
							next++
						}
					}
				}
				for _, a := range inst.Args {
					if a != NoReg {
						if _, ok := remap[a]; !ok {
							remap[a] = next
							next++
						}
					}
				}
				for _, er := range inst.ExtraRets {
					if er != NoReg {
						if _, ok := remap[er]; !ok {
							remap[er] = next
							next++
						}
					}
				}
			}
			if b.Term != nil {
				for _, r := range b.Term.termUses() {
					if r != NoReg {
						if _, ok := remap[r]; !ok {
							remap[r] = next
							next++
						}
					}
				}
			}
		}

		if len(remap) == 0 {
			continue
		}

		// Apply remap.
		mr := func(r Reg) Reg {
			if r == NoReg {
				return NoReg
			}
			if nr, ok := remap[r]; ok {
				return nr
			}
			return r
		}
		mrs := func(rs []Reg) {
			for i := range rs {
				rs[i] = mr(rs[i])
			}
		}

		for i := range f.Contract.Params {
			f.Contract.Params[i].Reg = mr(f.Contract.Params[i].Reg)
		}
		for _, b := range f.Blocks {
			for i := range b.Params {
				b.Params[i].Dst = mr(b.Params[i].Dst)
			}
			for _, inst := range b.Insts {
				inst.Dst = mr(inst.Dst)
				for i := range inst.Src {
					inst.Src[i] = mr(inst.Src[i])
				}
				mrs(inst.Args)
				mrs(inst.ExtraRets)
			}
			if b.Term != nil {
				remapTerm(b.Term, mr, mrs)
			}
		}

		f.nextReg = next
	}
	m.nextReg = next
}

func remapTerm(t Term, mr func(Reg) Reg, mrs func([]Reg)) {
	switch t := t.(type) {
	case *TermJmp:
		mrs(t.Args)
	case *TermBrIf:
		t.Cond = mr(t.Cond)
		mrs(t.ThenArgs)
		mrs(t.ElseArgs)
	case *TermBrIf2:
		t.Lhs = mr(t.Lhs)
		t.Rhs = mr(t.Rhs)
		mrs(t.EqArgs)
		mrs(t.LtArgs)
		mrs(t.GtArgs)
	case *TermDJNZ:
		t.Counter = mr(t.Counter)
		mrs(t.BodyArgs)
		mrs(t.ExitArgs)
	case *TermCondRet:
		t.Cond = mr(t.Cond)
		mrs(t.Vals)
		mrs(t.ThenArgs)
	case *TermRet:
		mrs(t.Vals)
	case *TermUnreachable:
		// nothing
	}
}
