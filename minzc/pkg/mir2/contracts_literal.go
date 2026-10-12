package mir2

import "reflect"

// pfccoCallWordLiteral permits placement of a call-only word constant directly
// in the chosen parameter class. This is producer placement, not a claim that a
// runtime register transfer or rematerialization is free.
func pfccoCallWordLiteral(f *Func, call *Inst, ai int) *Inst {
	if f == nil || call == nil || call.Op != OpCall || ai >= len(call.Args) {
		return nil
	}
	r := call.Args[ai]
	if r == NoReg {
		return nil
	}
	for _, p := range f.Contract.Params {
		if p.Reg == r {
			return nil
		}
	}
	var def *Inst
	var db, cb *Block
	defs, uses := 0, 0
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			if p.Dst == r {
				return nil
			}
		}
		for _, i := range b.Insts {
			if i == call {
				cb = b
			}
			if i.Dst == r {
				defs++
				def = i
				db = b
			}
			for _, x := range i.ExtraRets {
				if x == r {
					defs++
				}
			}
			for _, x := range i.Uses() {
				if x == r {
					uses++
				}
			}
		}
		if b.Term != nil {
			for _, x := range b.Term.termUses() {
				if x == r {
					uses++
				}
			}
		}
	}
	if defs != 1 || uses != 1 || db != cb || def == nil || def.Op != OpConst ||
		(def.Ty != TyU16 && def.Ty != TyI16) || !pfccoWordLiteralClass(def.Cls) {
		return nil
	}
	canonical := &Inst{Op: OpConst, Dst: r, Imm: def.Imm, Ty: def.Ty, Cls: def.Cls, SrcFile: def.SrcFile, SrcLine: def.SrcLine}
	if !reflect.DeepEqual(def, canonical) {
		return nil
	}
	if (def.Ty == TyU16 && (def.Imm < 0 || def.Imm > 65535)) || (def.Ty == TyI16 && (def.Imm < -32768 || def.Imm > 32767)) {
		return nil
	}
	seen := false
	for _, i := range cb.Insts {
		if i == call {
			if seen {
				return def
			}
			return nil
		}
		if i == def {
			seen = true
		}
	}
	return nil
}

func pfccoWordLiteralClass(cls RegClass) bool {
	return cls == ClassPointer || cls == ClassIndex || cls == ClassPair
}

// PlacePFCCOCallWordLiterals implements the placement assumed by PFCCO edge
// costs. Callers must invoke it before allocation, with the selected contracts.
// Greedy compilation does not use this transformation.
func PlacePFCCOCallWordLiterals(m *Module, cs ContractSet, ct CostTable) int {
	if _, ok := ct.(Z80CostTable); !ok {
		return 0
	}
	changed := 0
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, call := range b.Insts {
				if call.Op != OpCall {
					continue
				}
				callee := m.FuncByName(call.Sym)
				ch := cs[call.Sym]
				if callee == nil || ch == nil || callee.Attrs.IsExtern || callee.Attrs.FixedIndirectABI || funcHasAsm(callee) {
					continue
				}
				for ai := range call.Args {
					if ai >= len(ch.ParamClasses) || ai >= len(callee.Contract.Params) || !pfccoWordLiteralClass(ch.ParamClasses[ai]) {
						continue
					}
					if d := pfccoCallWordLiteral(f, call, ai); d != nil && d.Ty == callee.Contract.Params[ai].Ty && d.Cls != ch.ParamClasses[ai] {
						d.Cls = ch.ParamClasses[ai]
						changed++
					}
				}
			}
		}
	}
	return changed
}
