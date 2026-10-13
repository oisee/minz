package mir2

// MergeConstantReturnTails shares pure constant-return blocks. It does not move
// flag writers, compute return values before a branch, or cross effects. Callers
// opt into the possible branch-layout tradeoff when optimizing for code size.
func MergeConstantReturnTails(f *Func) bool {
	if f == nil || len(f.Contract.Returns) != 1 || f.Contract.Returns[0].Class == ClassFlag {
		return false
	}
	type key struct {
		ty    Ty
		value int64
		class RegClass
		hard  bool
	}
	canonical := map[key]string{}
	redirect := map[string]string{}
	for i, b := range f.Blocks {
		if i == 0 || b == nil || len(b.Params) != 0 || len(b.Insts) != 1 {
			continue
		}
		inst := b.Insts[0]
		ret, ok := b.Term.(*TermRet)
		if inst == nil || !ok || len(ret.Vals) != 1 || ret.Vals[0] != inst.Dst || inst.Op != OpConst || inst.Dst <= 0 || inst.Ty != f.Contract.Returns[0].Ty || inst.Src != [2]Reg{} || inst.Sym != "" {
			continue
		}
		if inst.Ty != TyU8 && inst.Ty != TyI8 && inst.Ty != TyU16 && inst.Ty != TyI16 {
			continue
		}
		valid := true
		// No exported definitions or implicit DJNZ parameter may be removed.
		for _, other := range f.Blocks {
			if other == b {
				continue
			}
			for _, op := range other.Insts {
				if op != nil {
					for _, r := range op.Src {
						if r == inst.Dst {
							valid = false
						}
					}
				}
			}
			if other.Term == nil {
				valid = false
				continue
			}
			for _, r := range other.Term.termUses() {
				if r == inst.Dst {
					valid = false
				}
			}
			other.Term.ForEachEdge(func(label string, args []Reg, offset int) {
				if label == b.Label && (len(args) != 0 || offset != 0) {
					valid = false
				}
			})
		}
		if !valid {
			continue
		}
		k := key{inst.Ty, inst.Imm, inst.Cls, inst.ClsHard}
		if label, ok := canonical[k]; ok {
			redirect[b.Label] = label
		} else {
			canonical[k] = b.Label
		}
	}
	if len(redirect) == 0 {
		return false
	}
	remap := func(label string) string {
		if target, ok := redirect[label]; ok {
			return target
		}
		return label
	}
	for _, b := range f.Blocks {
		switch t := b.Term.(type) {
		case *TermJmp:
			t.Target = remap(t.Target)
		case *TermBrIf:
			t.Then = remap(t.Then)
			t.Else = remap(t.Else)
		case *TermBrIf2:
			t.Eq = remap(t.Eq)
			t.Lt = remap(t.Lt)
			t.Gt = remap(t.Gt)
		case *TermCondRet:
			t.Then = remap(t.Then)
		case *TermDJNZ:
			t.Exit = remap(t.Exit) // body is excluded by implicit parameter check
		}
	}
	kept := f.Blocks[:0]
	for _, b := range f.Blocks {
		if _, drop := redirect[b.Label]; !drop {
			kept = append(kept, b)
		}
	}
	f.Blocks = kept
	return true
}
