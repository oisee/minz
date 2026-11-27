package mir2

// Global addresses passed as call arguments.
//
// `p = addr_of @g` used only as an argument of a direct call is a link-time
// constant: the call can load it straight into the parameter's register
// (`LD rr, g`) while it sets up the other arguments.  Giving it a home of its
// own instead makes it occupy a pair from its definition to the call, and the
// argument copy then shuffles it there:
//
//	LD HL,values / LD C,L / LD B,H / … / LD H,B / LD L,C / CALL f   (main)
//	… / LD HL,values / CALL f                                        (here)
//
// Two pieces:
//
//   - MaterializeAddressMoves (MIR, before allocation) turns a copy of an
//     address constant, `r = move s` with `s = addr_of @g`, into
//     `r = addr_of @g`; lowering inserts such moves for call arguments.
//   - callArgumentAddressConstants (allocation and codegen) names the addr_of
//     vregs whose single use is an argument of a direct call whose parameter
//     home is a register pair or index register.  computeDeadConsts reports
//     them dead, constant rematerialisation gives them no home, and
//     emitCallArgsForInst loads the symbol into the parameter register as an
//     immediate after the register-to-register argument copies.

// MaterializeAddressMoves rewrites copies of single-definition address
// constants into the constant itself and returns how many it rewrote.
func MaterializeAddressMoves(f *Func) int {
	if f == nil {
		return 0
	}
	defs := map[Reg]*Inst{}
	count := map[Reg]int{}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			count[p.Dst] += 2
		}
		for _, x := range b.Insts {
			if x.Dst != NoReg {
				defs[x.Dst] = x
				count[x.Dst]++
			}
			for _, r := range x.ExtraRets {
				count[r] += 2
			}
		}
	}
	for _, p := range f.Contract.Params {
		count[p.Reg] += 2
	}
	changed := 0
	for _, b := range f.Blocks {
		for _, x := range b.Insts {
			if x.Op != OpMove || x.Dst == NoReg || x.ClsHard {
				continue
			}
			d := defs[x.Src[0]]
			if d == nil || d.Op != OpAddrOf || d.Sym == "" || d.Volatile || count[x.Src[0]] != 1 {
				continue
			}
			x.Op = OpAddrOf
			x.Sym = d.Sym
			x.Src = [2]Reg{}
			changed++
		}
	}
	if changed != 0 {
		DeadStoreElim(f)
	}
	return changed
}

// callArgumentAddressConstants maps each addr_of vreg of f that a direct call
// takes as its only use to its symbol, when the parameter's home under ar is
// HL, DE, BC, IX or IY (registers `LD rr, nn` can load) or the callee's
// memory slot (stored through HL, which is saved around it).  The callee may
// not be allocated yet when the caller is; its contract's canonical home is
// used then, and codegen re-checks against the final allocation.
func callArgumentAddressConstants(f *Func, ar *AllocResult) map[Reg]string {
	if f == nil || f.mod == nil || ar == nil {
		return nil
	}
	defs := map[Reg]int{}
	addr := map[Reg]*Inst{}
	uses := map[Reg]int{}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			defs[p.Dst] += 2
		}
		for _, x := range b.Insts {
			if x.Dst != NoReg {
				defs[x.Dst]++
				if x.Op == OpAddrOf && x.Sym != "" && !x.Volatile {
					addr[x.Dst] = x
				}
			}
			for _, r := range x.ExtraRets {
				defs[r] += 2
			}
			for _, r := range x.Uses() {
				uses[r]++
			}
		}
		if b.Term != nil {
			for _, r := range b.Term.termUses() {
				uses[r]++
			}
		}
	}
	for _, p := range f.Contract.Params {
		defs[p.Reg] += 2
	}
	if len(addr) == 0 {
		return nil
	}
	var out map[Reg]string
	for _, b := range f.Blocks {
		seen := map[Reg]bool{}
		for _, x := range b.Insts {
			if x.Op == OpAddrOf && addr[x.Dst] == x {
				seen[x.Dst] = true
			}
			if x.Op != OpCall || x.ByteLoop != nil || x.Sym == "" || x.Sym[0] == '@' {
				continue
			}
			var callee *Func
			for _, g := range f.mod.Funcs {
				if sanitizeIdent(g.Name) == sanitizeIdent(x.Sym) {
					if callee != nil {
						callee = nil
						break
					}
					callee = g
				}
			}
			if callee == nil || callee.Attrs.IsExtern || callee.Attrs.FixedIndirectABI {
				continue
			}
			for i, arg := range x.Args {
				if !seen[arg] || defs[arg] != 1 || uses[arg] != 1 || i >= len(callee.Contract.Params) {
					continue
				}
				param := callee.Contract.Params[i]
				if param.Ty == nil || param.Ty.Width() != 16 {
					continue
				}
				home := ""
				if loc, ok := ar.Locs[param.Reg]; ok && loc.Name != "" {
					switch loc.Kind {
					case LocReg, LocIXY:
						home = physName(loc)
					case LocMem:
						home = "mem" // the callee's named spill slot
					default:
						continue
					}
				} else {
					home = canonicalReturnLoc(param.Class, param.Ty)
				}
				switch home {
				case "HL", "DE", "BC", "IX", "IY", "mem":
				default:
					continue
				}
				if out == nil {
					out = map[Reg]string{}
				}
				out[arg] = addr[arg].Sym
			}
		}
	}
	return out
}
