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
	// CFG rewrites may share or overlap edge-argument backing arrays. Detach
	// every owned argument field before mutating IDs; each function's mapping
	// must read its original IDs, including when another function shares a slice.
	clone := func(rs []Reg) []Reg {
		if rs == nil {
			return nil
		}
		return append([]Reg{}, rs...)
	}
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, in := range b.Insts {
				in.Args = clone(in.Args)
				in.ExtraRets = clone(in.ExtraRets)
				if in.Asm != nil {
					a := *in.Asm
					a.Ins = clone(a.Ins)
					a.Outs = clone(a.Outs)
					in.Asm = &a
				}
			}
			switch t := b.Term.(type) {
			case *TermJmp:
				n := *t
				n.Args = clone(t.Args)
				b.Term = &n
			case *TermBrIf:
				n := *t
				n.ThenArgs = clone(t.ThenArgs)
				n.ElseArgs = clone(t.ElseArgs)
				b.Term = &n
			case *TermBrIf2:
				n := *t
				n.EqArgs = clone(t.EqArgs)
				n.LtArgs = clone(t.LtArgs)
				n.GtArgs = clone(t.GtArgs)
				b.Term = &n
			case *TermDJNZ:
				n := *t
				n.BodyArgs = clone(t.BodyArgs)
				n.ExitArgs = clone(t.ExitArgs)
				b.Term = &n
			case *TermCondRet:
				n := *t
				n.Vals = clone(t.Vals)
				n.ThenArgs = clone(t.ThenArgs)
				b.Term = &n
			case *TermRet:
				n := *t
				n.Vals = clone(t.Vals)
				b.Term = &n
			}
		}
	}
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
				if inst.Asm != nil {
					for _, r := range append(append([]Reg{}, inst.Asm.Ins...), inst.Asm.Outs...) {
						if r != NoReg {
							if _, ok := remap[r]; !ok {
								remap[r] = next
								next++
							}
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

		// Allocate metadata-only IDs after executable registers, preserving native allocation.
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				var regs []Reg
				for _, d := range i.SequentialAddressDefs {
					if d != nil {
						regs = append(regs, d.Dst, d.Src[0], d.Src[1])
					}
				}
				for _, o := range i.SequentialByteOrigins {
					if o != nil {
						regs = append(regs, o.Src[0])
					}
				}
				for _, r := range regs {
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
				// Metadata registers need a consistent mapping even when their
				// executable definitions were eliminated after fusion.
				for _, d := range inst.SequentialAddressDefs {
					if d != nil {
						d.Dst = mr(d.Dst)
						d.Src[0] = mr(d.Src[0])
						d.Src[1] = mr(d.Src[1])
					}
				}
				for _, o := range inst.SequentialByteOrigins {
					if o != nil {
						o.Src[0] = mr(o.Src[0])
					}
				}
				mrs(inst.Args)
				mrs(inst.ExtraRets)
				if inst.Asm != nil {
					mrs(inst.Asm.Ins)
					mrs(inst.Asm.Outs)
				}
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
	case *TermErrorRet:
		t.Code = mr(t.Code)
	case *TermRet:
		mrs(t.Vals)
	case *TermUnreachable:
		// nothing
	}
}
