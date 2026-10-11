package mir2

import (
	"fmt"
	"reflect"
)

// ReuseNormalGlobalDiamondLoads is an isolated default-off experiment, never
// installed in a pipeline. Normal global read counts and VM steps may change.
func ReuseNormalGlobalDiamondLoads(m *Module) (int, error) {
	if err := normalDiamondShape(m); err != nil {
		return 0, err
	}
	if err := Verify(m); err != nil {
		return 0, err
	}
	globals := map[string]bool{}
	for _, g := range m.Globals {
		if g.Ty == TyU16 && g.At == nil && g.Storage == StorageNormal {
			globals[g.Name] = true
		}
	}
	type edit struct {
		target *Inst
		old    Inst
		source Reg
	}
	plans := []edit{}
	for _, f := range m.Funcs {
		if f.Attrs.IsExtern || len(f.Blocks) == 0 || !normalDiamondAcyclic(f) {
			continue
		}
		entry := f.Entry()
		br, ok := entry.Term.(*TermBrIf)
		if !ok {
			continue
		}
		defs := map[Reg]*Inst{}
		for _, b := range f.Blocks {
			for _, i := range b.Insts {
				if i.Dst != NoReg {
					defs[i.Dst] = i
				}
			}
		}
		symbol := func(i *Inst) string {
			if i.Op != OpLoad || i.Ty != TyU16 || i.Dst == NoReg || i.Src[1] != NoReg {
				return ""
			}
			clean := Inst{Op: OpLoad, Dst: i.Dst, Src: i.Src, Ty: i.Ty, Cls: i.Cls, ClsHard: i.ClsHard, SrcLine: i.SrcLine, SrcFile: i.SrcFile}
			if !reflect.DeepEqual(*i, clean) {
				return ""
			}
			a := defs[i.Src[0]]
			if a == nil || a.Op != OpAddrOf || a.Ty != TyPtr || !globals[a.Sym] {
				return ""
			}
			clean = Inst{Op: OpAddrOf, Dst: a.Dst, Sym: a.Sym, Ty: a.Ty, Cls: a.Cls, ClsHard: a.ClsHard, SrcLine: a.SrcLine, SrcFile: a.SrcFile}
			if !reflect.DeepEqual(*a, clean) {
				return ""
			}
			return a.Sym
		}
		pure := func(i *Inst) bool {
			if i.Op == OpLoad {
				return symbol(i) != ""
			}
			if i.Asm != nil || len(i.Args) != 0 || len(i.ExtraRets) != 0 || len(i.ExtraRetClasses) != 0 || len(i.ExtraRetTys) != 0 || i.CallAttr != (CallAttrs{}) {
				return false
			}
			switch i.Op {
			case OpConst, OpMove, OpAdd, OpSub, OpMul, OpAnd, OpOr, OpXor, OpShl, OpShr, OpSar, OpNeg, OpNot, OpExt, OpSext, OpTrunc, OpCmp:
				return true
			case OpAddrOf:
				return globals[i.Sym] && i.Ty == TyPtr
			}
			return false
		}
		seen := map[string]bool{}
		for _, label := range []string{br.Then, br.Else} {
			if seen[label] {
				continue
			}
			seen[label] = true
			succ := f.BlockByLabel(label)
			if succ == nil || succ == entry || len(succ.Params) != 0 {
				continue
			}
			unique := true
			for _, b := range f.Blocks {
				for _, n := range b.Successors() {
					if n == label && b != entry {
						unique = false
					}
				}
			}
			if !unique {
				continue
			}
			for si, target := range succ.Insts {
				key := symbol(target)
				if key == "" {
					continue
				}
				safe := true
				for _, i := range succ.Insts[:si] {
					if !pure(i) {
						safe = false
						break
					}
				}
				if !safe {
					continue
				}
				source := NoReg
				for pi, load := range entry.Insts {
					if symbol(load) != key {
						continue
					}
					safe = true
					for _, i := range entry.Insts[pi+1:] {
						if !pure(i) {
							safe = false
							break
						}
					}
					if safe {
						source = load.Dst
					}
				}
				if source != NoReg {
					plans = append(plans, edit{target, *target, source})
				}
			}
		}
	}
	for _, p := range plans {
		*p.target = Inst{Op: OpMove, Dst: p.old.Dst, Src: [2]Reg{p.source}, Ty: p.old.Ty, Cls: p.old.Cls, ClsHard: p.old.ClsHard, SrcLine: p.old.SrcLine, SrcFile: p.old.SrcFile}
	}
	if err := Verify(m); err != nil {
		for _, p := range plans {
			*p.target = p.old
		}
		return 0, fmt.Errorf("normal global diamond: post verification: %w", err)
	}
	return len(plans), nil
}
func normalDiamondShape(m *Module) error {
	if m == nil {
		return fmt.Errorf("normal global diamond: nil module")
	}
	fn := map[string]bool{}
	blocks := map[*Block]bool{}
	insts := map[*Inst]bool{}
	globals := map[string]bool{}
	for _, g := range m.Globals {
		if g.Ty == nil || globals[g.Name] {
			return fmt.Errorf("normal global diamond: malformed global")
		}
		globals[g.Name] = true
	}
	for _, f := range m.Funcs {
		if f == nil || fn[f.Name] {
			return fmt.Errorf("normal global diamond: malformed function")
		}
		fn[f.Name] = true
		for _, b := range f.Blocks {
			if b == nil || blocks[b] || b.Term == nil || reflect.ValueOf(b.Term).IsNil() {
				return fmt.Errorf("normal global diamond: malformed block")
			}
			blocks[b] = true
			for _, i := range b.Insts {
				if i == nil || insts[i] {
					return fmt.Errorf("normal global diamond: shared/nil instruction")
				}
				insts[i] = true
			}
		}
	}
	return nil
}
func normalDiamondAcyclic(f *Func) bool {
	state := map[string]int{}
	var visit func(*Block) bool
	visit = func(b *Block) bool {
		if state[b.Label] == 1 {
			return false
		}
		if state[b.Label] == 2 {
			return true
		}
		state[b.Label] = 1
		for _, n := range b.Successors() {
			next := f.BlockByLabel(n)
			if next == nil || !visit(next) {
				return false
			}
		}
		state[b.Label] = 2
		return true
	}
	for _, b := range f.Blocks {
		if !visit(b) {
			return false
		}
	}
	return true
}
