package mir2

import (
	"fmt"
	"reflect"
)

// verifyDirectCallShape checks physical MIR slots for declared ordinary calls.
// Source coercions are already allowed by HIR and VM argument binding truncates
// integers; this deliberately checks compatible representations, not equality.
func verifyDirectCallShape(m *Module, f *Func) error {
	info := collectRegInfo(f)
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Op != OpCall && i.Op != OpCallCond {
				continue
			}
			callee := m.FuncByName(i.Sym)
			if callee == nil || isIntrinsic(i.Sym) {
				continue
			}
			if len(i.Args) != len(callee.Contract.Params) {
				return fmt.Errorf("direct call @%s in @%s: %d args, want %d", i.Sym, f.Name, len(i.Args), len(callee.Contract.Params))
			}
			for j, r := range i.Args {
				src, ok := info[r]
				want := callee.Contract.Params[j].Ty
				if r == NoReg || !ok || src.Ty == nil || src.Ty == TyVoid || want == nil || want == TyVoid || !callRepresentationCompatible(want, src.Ty) {
					return fmt.Errorf("direct call @%s argument %d: incompatible type %v for %v", i.Sym, j, src.Ty, want)
				}
			}
			if i.FallibleCY {
				continue
			} // independently checked CY outcome schema
			rets := callee.Contract.Returns
			if i.Dst != NoReg {
				if len(rets) == 0 || i.Ty == nil || i.Ty == TyVoid || !callRepresentationCompatible(i.Ty, rets[0].Ty) {
					return fmt.Errorf("direct call @%s: incompatible consumed return", i.Sym)
				}
			}
			for j, r := range i.ExtraRets {
				if r == NoReg {
					continue
				}
				if j+1 >= len(rets) {
					return fmt.Errorf("direct call @%s: extra return slot %d missing", i.Sym, j+1)
				}
				if j < len(i.ExtraRetTys) && !callRepresentationCompatible(i.ExtraRetTys[j], rets[j+1].Ty) {
					return fmt.Errorf("direct call @%s: incompatible extra return slot %d", i.Sym, j+1)
				}
			}
		}
	}
	return nil
}

func callRepresentationCompatible(dst, src Ty) bool {
	if dst == nil || src == nil || reflect.ValueOf(dst).Kind() == reflect.Ptr && reflect.ValueOf(dst).IsNil() || reflect.ValueOf(src).Kind() == reflect.Ptr && reflect.ValueOf(src).IsNil() {
		return false
	}
	dst, src = BaseOf(dst), BaseOf(src)
	if dst == src {
		return true
	}
	if (IsInt(dst) || IsPointer(dst)) && (IsInt(src) || IsPointer(src)) {
		return true
	}
	if IsPointer(dst) {
		switch src.(type) {
		case *StructTy, *ArrayTy:
			return true
		}
	}
	if _, ok := dst.(*StructTy); ok && IsPointer(src) {
		return true
	}
	return dst.String() == src.String()
}
