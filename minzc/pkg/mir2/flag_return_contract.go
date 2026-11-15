package mir2

import (
	"fmt"
	"regexp"
	"sort"
)

// FlagReturnReport records the Z80 return contract chosen for each function
// returning one bool: Chosen maps a function to the condition that means
// true in its flags; Refused maps a function that keeps the byte return in A
// to the reason.
type FlagReturnReport struct {
	Chosen  map[string]CmpCond
	Refused map[string]string
}

// ApplyFlagReturnContracts returns a function's bool in CPU flags instead of
// A when every reference to it is a direct call and every use of every call
// result is a branch condition, possibly through cmp.ne/cmp.eq against 0, or
// the return value of another function returning in flags. It is a Z80 code
// generation contract: MIR semantics stay bool 0/1.
//
// The condition is CmpEq, CmpLt or CmpGe when every return site returns a
// comparison with exactly those flags (directly or through a call returning
// them), and CmpNe otherwise. The callee's contract and the Cls of every call
// to it change together, then FoldFlagReturnTests removes the zero tests.
//
// A non-nil pin fixes the set of functions and their conditions, chosen once
// for a module whose variants are compiled separately: a pinned function that
// is not eligible in m is an error, any other function is refused.
func ApplyFlagReturnContracts(m *Module, pin map[string]CmpCond) (FlagReturnReport, error) {
	rep := FlagReturnReport{Chosen: map[string]CmpCond{}, Refused: map[string]string{}}
	byName := make(map[string]*Func, len(m.Funcs))
	for _, f := range m.Funcs {
		byName[f.Name] = f
	}
	cand := map[string]bool{}
	for _, f := range m.Funcs {
		if len(f.Contract.Returns) != 1 || f.Contract.Returns[0].Ty != TyBool || f.Contract.Returns[0].Class == ClassFlag {
			continue
		}
		if reason := flagReturnAttrRefusal(f); reason != "" {
			rep.Refused[f.Name] = reason
			continue
		}
		cand[f.Name] = true
	}
	refuse := func(name, reason string) {
		if cand[name] {
			delete(cand, name)
			rep.Refused[name] = reason
		}
	}

	// Every reference other than the callee of a direct call fixes the ABI.
	for name := range addrTakenFuncs(m) {
		refuse(name, "address taken")
	}
	for _, g := range m.Globals {
		for _, rel := range g.Relocs {
			refuse(rel.Symbol, "address in a global")
		}
		for _, sym := range g.VtableSyms {
			refuse(sym, "address in a global")
		}
	}
	var templates []string
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				if inst.Asm != nil && inst.Asm.Template != "" {
					templates = append(templates, inst.Asm.Template)
				}
				if inst.Sym == "" || !cand[inst.Sym] {
					continue
				}
				if inst.Op != OpCall || inst.ByteLoop != nil || inst.FallibleCY || len(inst.ExtraRets) != 0 || len(inst.StructReturnBuffers) != 0 {
					refuse(inst.Sym, fmt.Sprintf("referenced by %s", inst.Op))
				}
			}
		}
	}
	if len(templates) != 0 {
		for _, name := range sortedNames(cand) {
			word := regexp.MustCompile(`(^|[^A-Za-z0-9_.$])` + regexp.QuoteMeta(sanitizeIdent(name)) + `([^A-Za-z0-9_]|$)`)
			for _, t := range templates {
				if word.MatchString(t) {
					refuse(name, "named in inline asm")
					break
				}
			}
		}
	}
	if pin != nil {
		for _, name := range sortedNames(cand) {
			if _, ok := pin[name]; !ok {
				refuse(name, "not chosen for this module")
			}
		}
	}

	// Every use of a call result must be a branch condition, directly or
	// through a zero test, or the return value of a candidate. Dropping a
	// candidate can drop the callees whose results it returns: iterate.
	for changed := true; changed; {
		changed = false
		for _, f := range m.Funcs {
			for _, name := range flagReturnUseRefusals(f, cand) {
				if cand[name] {
					refuse(name, "result used as a value")
					changed = true
				}
			}
		}
	}

	// Choose each condition from the return sites: a meet over the
	// natural conditions of the returned comparisons and the conditions of
	// returned candidate calls, starting from "any" and falling to CmpNe.
	const anyCond = CmpCond(255)
	cond := map[string]CmpCond{}
	for name := range cand {
		cond[name] = anyCond
	}
	meet := func(a, b CmpCond) CmpCond {
		switch {
		case a == anyCond:
			return b
		case b == anyCond || a == b:
			return a
		}
		return CmpNe
	}
	for changed := true; changed; {
		changed = false
		for _, name := range sortedNames(cand) {
			f := byName[name]
			c := anyCond
			for _, val := range flagReturnSites(f) {
				c = meet(c, flagReturnSiteCond(f, val, byName, cond, cand))
			}
			if c != cond[name] {
				cond[name] = c
				changed = true
			}
		}
	}
	for name := range cand {
		c := cond[name]
		if c == anyCond {
			c = CmpNe
		}
		if pin != nil {
			c = pin[name]
		}
		rep.Chosen[name] = c
	}
	for name := range pin {
		if _, ok := rep.Chosen[name]; !ok && byName[name] != nil {
			return rep, fmt.Errorf("flag-return contract of %s differs from the module's: %s", name, rep.Refused[name])
		}
	}

	// The contract and every call's result class change together.
	for name, c := range rep.Chosen {
		byName[name].Contract.Returns[0] = Return{Ty: TyBool, Class: ClassFlag, FlagCond: c}
	}
	for _, f := range m.Funcs {
		for _, b := range f.Blocks {
			for _, inst := range b.Insts {
				if inst.Op == OpCall && inst.Dst != NoReg {
					if _, ok := rep.Chosen[inst.Sym]; ok {
						inst.Cls = ClassFlag
					}
				}
			}
		}
	}
	FoldFlagReturnTests(m)
	return rep, nil
}

// flagReturnAttrRefusal names a property of f that fixes its return ABI.
func flagReturnAttrRefusal(f *Func) string {
	switch {
	case f.Attrs.IsExtern || len(f.Blocks) == 0 || f.Attrs.ExternAddr != 0:
		return "extern"
	case f.Name == "main":
		return "entry point"
	case f.Attrs.ReferencedByName:
		return "referenced by name"
	case f.Attrs.FixedIndirectABI:
		return "indirect ABI"
	case f.Attrs.FallibleCY:
		return "CY error ABI"
	case funcHasAsm(f):
		return "inline asm"
	case f.SMCSetterFor != "" || len(f.Contract.StructReturnBuffers) != 0:
		return "fixed ABI"
	case lutEligible(f):
		return "lookup table"
	}
	return ""
}

// flagReturnSites returns the value of every return site of f.
func flagReturnSites(f *Func) []Reg {
	var vals []Reg
	for _, b := range f.Blocks {
		switch t := b.Term.(type) {
		case *TermRet:
			vals = append(vals, t.Vals...)
		case *TermCondRet:
			vals = append(vals, t.Vals...)
		}
	}
	return vals
}

// flagReturnSiteCond is the condition a return site naturally leaves in
// the flags: an unsigned or equality comparison's own condition, a
// candidate call's chosen condition, or CmpNe for any other value.
func flagReturnSiteCond(f *Func, val Reg, byName map[string]*Func, cond map[string]CmpCond, cand map[string]bool) CmpCond {
	var def *Inst
	for _, b := range f.Blocks {
		for _, p := range b.Params {
			if p.Dst == val {
				return CmpNe
			}
		}
		for _, inst := range b.Insts {
			if inst.Dst == val {
				if def != nil {
					return CmpNe
				}
				def = inst
			}
		}
	}
	if def == nil {
		return CmpNe
	}
	switch def.Op {
	case OpCmp:
		switch def.Cond {
		case CmpEq:
			return CmpEq
		case CmpUlt:
			return CmpLt
		case CmpUge:
			return CmpGe
		}
	case OpCall:
		if cand[def.Sym] {
			return cond[def.Sym]
		}
		if callee := byName[def.Sym]; callee != nil && flagReturn(callee) {
			return callee.Contract.Returns[0].FlagCond
		}
	}
	return CmpNe
}

// flagReturnUseRefusals returns the candidates called in f whose result has
// a use other than a branch condition, a zero test feeding only branch
// conditions, or the return value of f when f is a candidate.
func flagReturnUseRefusals(f *Func, cand map[string]bool) []string {
	callee := map[Reg]string{}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op == OpCall && inst.Dst != NoReg && cand[inst.Sym] {
				callee[inst.Dst] = inst.Sym
			}
		}
	}
	if len(callee) == 0 {
		return nil
	}
	// branchOnly reports whether every use of r is a br_if or cond_ret
	// condition and not also a block argument or return value.
	uses := map[Reg]int{}
	condUses := map[Reg]int{}
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			for _, r := range inst.Uses() {
				uses[r]++
			}
		}
		for _, r := range b.Term.termUses() {
			uses[r]++
		}
		switch t := b.Term.(type) {
		case *TermBrIf:
			condUses[t.Cond]++
		case *TermCondRet:
			condUses[t.Cond]++
		}
	}
	branchOnly := func(r Reg) bool { return uses[r] == condUses[r] }
	allowed := map[Reg]int{} // uses of a call result accounted for
	for _, b := range f.Blocks {
		for _, inst := range b.Insts {
			if inst.Op != OpCmp || inst.Cond != CmpEq && inst.Cond != CmpNe {
				continue
			}
			for i, r := range inst.Src {
				if _, ok := callee[r]; !ok {
					continue
				}
				if v, ok := flagReturnConst(f, inst.Src[1-i]); ok && v == 0 && branchOnly(inst.Dst) {
					allowed[r]++
				}
			}
		}
		switch t := b.Term.(type) {
		case *TermBrIf:
			if _, ok := callee[t.Cond]; ok {
				allowed[t.Cond]++
			}
		case *TermCondRet:
			if _, ok := callee[t.Cond]; ok {
				allowed[t.Cond]++
			}
		}
		if t, ok := b.Term.(*TermRet); ok && cand[f.Name] && len(t.Vals) == 1 {
			if _, ok := callee[t.Vals[0]]; ok {
				allowed[t.Vals[0]]++
			}
		}
	}
	var out []string
	for r, name := range callee {
		if uses[r] != allowed[r] {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
}

func sortedNames(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for name := range set {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}
