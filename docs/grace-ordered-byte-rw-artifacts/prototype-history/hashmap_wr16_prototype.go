package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

// Manual experiment only. Even base makes logical base+1 nonoverflowing;
// physical pointer wrap and sequential partial stores remain unchanged.
func ApplyHashmapWR16Prototype(m *Module) error {
	if e := Verify(m); e != nil {
		return e
	}
	f := m.FuncByName("wr16")
	if f == nil || len(f.Params) != 3 || f.Body == nil || len(f.Body.Body) != 3 {
		return fmt.Errorf("wr16 prototype: shape")
	}
	d, ok := f.Body.Body[0].(*VarDeclStmt)
	if !ok {
		return fmt.Errorf("wr16 prototype: base")
	}
	arrName, idxName, valName, baseName := f.Params[0].Name, f.Params[1].Name, f.Params[2].Name, d.Name
	names := map[string]bool{}
	reserved := forwardCopyReserved(m)
	for _, n := range []string{arrName, idxName, valName, baseName} {
		if n == "" || names[n] || reserved[n] {
			return fmt.Errorf("wr16 prototype: binding")
		}
		names[n] = true
	}
	arr := Var(arrName, mir2.TyPtr)
	base := Var(baseName, mir2.TyU16)
	val := Var(valName, mir2.TyU16)
	ix := func(offset Expr) Expr { return &IndexExpr{Base: arr, Idx: offset, ElemTy: mir2.TyU8} }
	want := Func{Name: f.Name, Params: []Param{{Name: arrName, Ty: mir2.TyPtr}, {Name: idxName, Ty: mir2.TyU16}, {Name: valName, Ty: mir2.TyU16}}, RetTy: mir2.TyVoid, Body: Blk(Decl(baseName, mir2.TyU16, &BinExpr{Op: "*", L: Var(idxName, mir2.TyU16), R: U8(2), Ty: mir2.TyU16}), Assign(ix(base), Cast(val, mir2.TyU8)), Assign(ix(Add(base, U8(1), mir2.TyU16)), Cast(&BinExpr{Op: "/", L: val, R: U16(256), Ty: mir2.TyU16}, mir2.TyU8)))}
	if !reflect.DeepEqual(*f, want) {
		return fmt.Errorf("wr16 prototype: exact source/metadata")
	}
	pointer := freshGracePointerName(f, reserved)
	p := Var(pointer, mir2.TyPtr)
	old := f.Body
	// Each original value is evaluated at its original store, not hoisted. Address
	// arithmetic is pure, so StoreStmt's pointer-first order is observationally safe.
	f.Body = Blk(old.Body[0], Decl(pointer, mir2.TyPtr, Add(arr, base, mir2.TyPtr)), &StoreStmt{Ptr: p, Val: old.Body[1].(*AssignStmt).Val}, &StoreStmt{Ptr: Add(p, U16(1), mir2.TyPtr), Val: old.Body[2].(*AssignStmt).Val})
	if e := Verify(m); e != nil {
		f.Body = old
		return e
	}
	return nil
}
