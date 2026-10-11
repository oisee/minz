package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

// ApplyHashmapRD16Prototype is an exact-source manual experiment. Multiplication
// by two modulo 65536 yields an even base, so base+1 never overflows. The two
// physical addresses can therefore share their initial addition, including wrap.
// Both reads remain byte reads in low-then-high order; no wide load is introduced.
func ApplyHashmapRD16Prototype(m *Module) error {
	if err := Verify(m); err != nil {
		return err
	}
	f := m.FuncByName("rd16")
	arr := Var("arr", mir2.TyPtr)
	base := Var("base", mir2.TyU16)
	idx := func(offset Expr) Expr { return &IndexExpr{Base: arr, Idx: offset, ElemTy: mir2.TyU8} }
	want := Func{Name: "rd16", Params: []Param{{Name: "arr", Ty: mir2.TyPtr}, {Name: "idx", Ty: mir2.TyU16}}, RetTy: mir2.TyU16, Body: Blk(Decl("base", mir2.TyU16, &BinExpr{Op: "*", L: Var("idx", mir2.TyU16), R: U8(2), Ty: mir2.TyU16}), &ReturnStmt{Val: Add(Cast(idx(base), mir2.TyU16), &BinExpr{Op: "*", L: Cast(idx(Add(base, U8(1), mir2.TyU16)), mir2.TyU16), R: U16(256), Ty: mir2.TyU16}, mir2.TyU16)})}
	if f == nil || !reflect.DeepEqual(*f, want) {
		return fmt.Errorf("rd16 prototype: source or ABI is not exact")
	}
	reserved := forwardCopyReserved(m)
	for _, name := range []string{"arr", "idx", "base"} {
		if reserved[name] {
			return fmt.Errorf("rd16 prototype: binding collision")
		}
	}
	name := freshGracePointerName(f, reserved)
	p := Var(name, mir2.TyPtr)
	old := f.Body
	f.Body = Blk(old.Body[0], Decl(name, mir2.TyPtr, Add(arr, base, mir2.TyPtr)), &ReturnStmt{Val: Add(Cast(&LoadExpr{Ptr: p, Ty: mir2.TyU8}, mir2.TyU16), &BinExpr{Op: "*", L: Cast(&LoadExpr{Ptr: Add(p, U16(1), mir2.TyPtr), Ty: mir2.TyU8}, mir2.TyU16), R: U16(256), Ty: mir2.TyU16}, mir2.TyU16)})
	if err := Verify(m); err != nil {
		f.Body = old
		return err
	}
	return nil
}
