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

// MaterializeAddressMoves rewrites the only copy of a single-definition,
// single-use address constant into the constant itself, when that copy is
// used only as a direct call's argument, and returns how many it rewrote.
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
	uses := countRegUses(f)
	callArg := map[Reg]int{}
	for _, b := range f.Blocks {
		for _, x := range b.Insts {
			if x.Op == OpCall {
				for _, r := range x.Args {
					callArg[r]++
				}
			}
		}
	}
	changed := 0
	for _, b := range f.Blocks {
		for _, x := range b.Insts {
			if x.Op != OpMove || x.Dst == NoReg || x.ClsHard {
				continue
			}
			// Only the copy lowering makes for a call argument: a copy that
			// feeds a block edge is a phi input whose home the coalescer
			// pairs with the parameter (fusion_map_store, Nanz: the loop
			// pointers swapped pairs, +2 bytes).
			if uses[x.Dst] != 1 || callArg[x.Dst] != 1 {
				continue
			}
			d := defs[x.Src[0]]
			// The address's only use must be this move: with several readers
			// each copy would become its own LD rr,sym where one register
			// value served them all (k03_user_count_take_skip_zip, +3 bytes).
			if d == nil || d.Op != OpAddrOf || d.Sym == "" || d.Volatile || count[x.Src[0]] != 1 || uses[x.Src[0]] != 1 {
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

// ForwardCallArgumentMoves removes the copy lowering places between a widened
// byte and the call that takes it: `s = ext b; r = move s` (adjacent) with r
// used only as a direct call's argument and s used only by the move becomes
// `r = ext b`,
// keeping the move's register class (the parameter's preference).  The value
// is then computed in the register the call wants (fat12: the widened global
// byte `ext` was built in HL for its [pointer] class and copied to DE).
func ForwardCallArgumentMoves(f *Func) int {
	if f == nil {
		return 0
	}
	defs := map[Reg]int{}
	uses := map[Reg]int{}
	callArg := map[Reg]int{}
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			defs[p.Dst] += 2
		}
		for _, x := range b.Insts {
			if x.Dst != NoReg {
				defs[x.Dst]++
			}
			for _, r := range x.ExtraRets {
				defs[r] += 2
			}
			for _, r := range x.Uses() {
				uses[r]++
			}
			if x.Op == OpCall {
				for _, r := range x.Args {
					callArg[r]++
				}
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
	changed := 0
	for _, b := range f.Blocks {
		local := map[Reg]*Inst{}
		var keep []*Inst
		for _, x := range b.Insts {
			if x.Op == OpMove && x.Dst != NoReg && !x.ClsHard && defs[x.Dst] == 1 && uses[x.Dst] == 1 && callArg[x.Dst] == 1 {
				s := x.Src[0]
				d := local[s]
				// d must be the instruction right before the move.  Then the
				// merged vreg lives exactly as long as the move's result did;
				// with anything in between (a call, above all) the move is what
				// let the value sit in one home across it and reach the
				// argument register only at the end, and merging the two ties
				// both to one home (tetris_playable: an `and` result kept in A
				// across a call instead of IXH).
				//
				// d must also be a zero extension of a byte to a word, which
				// the Z80 builds in any pair (LD lo,b / LD hi,0), so giving it
				// the move's class costs nothing at the definition.  The
				// class is an input of the contract optimiser: retargeting an
				// ALU byte result (computed in A) to a parameter's class moved
				// unrelated ABI choices module-wide (tetris_playable: four
				// stage_at parameters went from IXH to A, +300 bytes).
				adjacent := len(keep) > 0 && keep[len(keep)-1] == d
				if d != nil && adjacent && d.Op == OpExt && d.SrcTy != nil && d.SrcTy.Width() == 8 && d.Ty != nil && d.Ty.Width() == 16 &&
					defs[s] == 1 && uses[s] == 1 && !d.ClsHard && d.Ty == x.Ty && len(d.ExtraRets) == 0 {
					d.Dst = x.Dst
					d.Cls = x.Cls
					changed++
					continue // drop the move
				}
			}
			if x.Dst != NoReg {
				local[x.Dst] = x
			}
			keep = append(keep, x)
		}
		b.Insts = keep
	}
	return changed
}
