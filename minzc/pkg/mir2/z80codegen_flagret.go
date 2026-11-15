package mir2

// flagReturn reports whether f returns its single value in flags.
func flagReturn(f *Func) bool {
	return len(f.Contract.Returns) == 1 && f.Contract.Returns[0].Class == ClassFlag
}

// flagReturnConst reports the value of r when its only definition is an
// OpConst. A block or function parameter coalesced with a constant has
// other definitions and is not a constant.
func flagReturnConst(f *Func, r Reg) (int64, bool) {
	for _, p := range f.Contract.Params {
		if p.Reg == r {
			return 0, false
		}
	}
	var imm int64
	defs := 0
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			if p.Dst == r {
				return 0, false
			}
		}
		for _, inst := range b.Insts {
			if inst.Dst == r {
				if inst.Op != OpConst {
					return 0, false
				}
				imm = inst.Imm
				defs++
			}
			for _, x := range inst.ExtraRets {
				if x == r {
					return 0, false
				}
			}
		}
	}
	return imm, defs == 1
}

// emitFlagReturnValue leaves val in F for a flag return: the contract's
// condition holds exactly when val is true. It writes A and F only, and
// is the last flag writer before the return's INC SP / RET.
//
//	constant          XOR A / OR 1 (NZ), SCF / OR A (carry)
//	predicate in F    nothing when its condition is the contract's,
//	                  CCF for the opposite carry, else 0/1 into A
//	byte (0/1)        loaded into A
//
// A 0/1 byte in A becomes the contract's condition with OR A (NZ),
// XOR 1 (Z), NEG (C) or CP 1 (NC).
func (g *z80cg) emitFlagReturnValue(f *Func, val Reg) {
	want := cmpCondCode(f.Contract.Returns[0].FlagCond)
	if imm, ok := flagReturnConst(f, val); ok && g.loc(val) != "F" {
		truth := imm != 0
		switch {
		case want == "NZ" && truth, want == "Z" && !truth:
			g.emit("    OR 1")
		case want == "NZ" || want == "Z":
			g.emit("    XOR A")
		case want == "C" && truth, want == "NC" && !truth:
			g.emit("    SCF")
		default:
			g.emit("    OR A")
		}
		g.invalidate("A")
		return
	}
	normalized := false // A = 0/1 and Z is set exactly when A = 0
	if g.loc(val) == "F" {
		cc := g.condCode(f, val)
		switch {
		case cc == want:
			return
		case cc == "C" && want == "NC", cc == "NC" && want == "C":
			g.emit("    CCF")
			return
		case cc == "C":
			g.emit("    SBC A, A")
			g.emit("    NEG")
			normalized = true
		case cc == "NC":
			g.emit("    SBC A, A")
			g.emit("    INC A")
			normalized = true
		case cc == "Z" || cc == "NZ":
			idx := g.trampIdx
			g.trampIdx++
			g.emit("    LD A, 0")
			g.emitf("    JR %s, .flag_ret_%d", invertCC(cc), idx)
			g.emit("    INC A")
			g.emitf(".flag_ret_%d:", idx)
		default:
			g.emitFlagPredicateByte(cc)
		}
	} else {
		g.emitLD8("A", g.loc(val))
	}
	g.invalidate("A")
	switch want {
	case "NZ":
		if !normalized {
			g.emit("    OR A")
		}
	case "Z":
		g.emit("    XOR 1")
	case "C":
		g.emit("    NEG")
	case "NC":
		g.emit("    CP 1")
	}
}
