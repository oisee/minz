package hir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

// ApplyHashmapSlotPrototype is a manual exact-source experiment, not a Grace rule
// or profile API. The masked induction is always0..15, including its wrap.
func ApplyHashmapSlotPrototype(m *Module) error {
	if e := Verify(m); e != nil {
		return e
	}
	reserved := forwardCopyReserved(m)
	// Only the original typed storage/ABIs are in scope. Effects of the calls
	// remain ordered; no purity inference or external dispatch is permitted.
	for _, spec := range []struct {
		name   string
		params []Param
		ret    mir2.Ty
	}{
		{"hash16", []Param{{Name: "key", Ty: mir2.TyU16}}, mir2.TyU16},
		{"rd16", []Param{{Name: "arr", Ty: mir2.TyPtr}, {Name: "idx", Ty: mir2.TyU16}}, mir2.TyU16},
		{"wr16", []Param{{Name: "arr", Ty: mir2.TyPtr}, {Name: "idx", Ty: mir2.TyU16}, {Name: "val", Ty: mir2.TyU16}}, mir2.TyVoid},
	} {
		f := m.FuncByName(spec.name)
		if f == nil || f.IsExtern || f.IsIO || f.ExternAddr != 0 || len(f.ExternClobbers) != 0 || len(f.Captures) != 0 || len(f.RetTys) != 0 || f.RetTy != spec.ret || !reflect.DeepEqual(f.Params, spec.params) {
			return fmt.Errorf("slot prototype: helper ABI")
		}
	}
	wantGlobals := map[string]mir2.Ty{"map_keys": mir2.NewArray(mir2.TyU8, 32), "map_vals": mir2.NewArray(mir2.TyU8, 32), "map_used": mir2.NewArray(mir2.TyU8, 16), "map_count": mir2.TyU16}
	for name, ty := range wantGlobals {
		found := false
		for _, g := range m.Globals {
			if g.Name == name && reflect.DeepEqual(g.Ty, ty) {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("slot prototype: storage shape")
		}
	}

	slot := Var("slot", mir2.TyU16)
	probes := Var("probes", mir2.TyU8)
	key := Var("key", mir2.TyU16)
	val := Var("val", mir2.TyU16)
	used := func() Expr {
		return &IndexExpr{Base: Var("map_used", mir2.NewArray(mir2.TyU8, 16)), Idx: slot, ElemTy: mir2.TyU8}
	}
	rd := func(name string) Expr { return Call("rd16", mir2.TyU16, Var(name, mir2.NewArray(mir2.TyU8, 32)), slot) }
	wr := func(name string, value Expr) Stmt {
		return &ExprStmt{Expr: Call("wr16", mir2.TyVoid, Var(name, mir2.NewArray(mir2.TyU8, 32)), slot, value)}
	}
	init := Decl("slot", mir2.TyU16, &BinExpr{Op: "&", L: Call("hash16", mir2.TyU16, key), R: U8(15), Ty: mir2.TyU16})
	step := Assign(slot, &BinExpr{Op: "&", L: Add(slot, U8(1), mir2.TyU16), R: U8(15), Ty: mir2.TyU16})
	probeStep := Assign(probes, Add(probes, U8(1), mir2.TyU8))
	put := Blk(init, Decl("probes", mir2.TyU8, U8(0)), While(Lt(probes, U8(16)), Blk(If(Eq(used(), U8(0)), Blk(Assign(used(), U8(1)), wr("map_keys", key), wr("map_vals", val), Assign(Var("map_count", mir2.TyU16), Add(Var("map_count", mir2.TyU16), U8(1), mir2.TyU16)), &ReturnStmt{Val: U8(1)}), nil), If(Eq(rd("map_keys"), key), Blk(wr("map_vals", val), &ReturnStmt{Val: U8(1)}), nil), step, probeStep)), &ReturnStmt{Val: U8(0)})
	get := Blk(init, Decl("probes", mir2.TyU8, U8(0)), While(Lt(probes, U8(16)), Blk(If(Eq(used(), U8(0)), Blk(&ReturnStmt{Val: U16(65535)}), nil), If(Eq(rd("map_keys"), key), Blk(&ReturnStmt{Val: rd("map_vals")}), nil), step, probeStep)), &ReturnStmt{Val: U16(65535)})
	for _, name := range []string{"map_put", "map_get"} {
		f := m.FuncByName(name)
		if f == nil {
			return fmt.Errorf("slot prototype: missing kernel")
		}
		expected := get
		params := []Param{{Name: "key", Ty: mir2.TyU16}}
		ret := mir2.TyU16
		if name == "map_put" {
			expected = put
			params = append(params, Param{Name: "val", Ty: mir2.TyU16})
			ret = mir2.TyU8
		}
		want := Func{Name: name, Params: params, RetTy: ret, Body: expected}
		if !reflect.DeepEqual(*f, want) {
			return fmt.Errorf("slot prototype: not exact %s\nactual:%s\nexpected:%s", name, (&Module{Funcs: []*Func{f}}).Dump(), (&Module{Funcs: []*Func{&want}}).Dump())
		}
		for _, n := range []string{"slot", "probes", "key", "val"} {
			if reserved[n] {
				return fmt.Errorf("slot prototype: binding collision")
			}
		}
	}
	for _, name := range []string{"map_put", "map_get"} {
		f := m.FuncByName(name)
		fresh := freshGracePointerName(f, reserved)
		narrow := Var(fresh, mir2.TyU8)
		body := hashmapSlotReplace(f.Body, Cast(narrow, mir2.TyU16))
		body.Body[0] = Decl(fresh, mir2.TyU8, Cast(f.Body.Body[0].(*VarDeclStmt).Init, mir2.TyU8))
		loop := body.Body[2].(*WhileStmt)
		loop.Body.Body[2] = Assign(narrow, &BinExpr{Op: "&", L: Add(narrow, U8(1), mir2.TyU8), R: U8(15), Ty: mir2.TyU8})
		f.Body = body
	}
	return Verify(m)
}

func hashmapSlotReplace(b *Block, value Expr) *Block {
	out := Blk()
	for _, stmt := range b.Body {
		copy := rewriteGraceBlock(Blk(stmt), nil, nil).Body[0]
		// replaceGraceExpr checks identity; run one structural slot occurrence at a
		// time until no old reference remains, with cloned nodes for each replacement.
		refs := []Expr{}
		walkGraceBlocks(Blk(copy), "", func(_ string, block *Block) {
			for _, s := range block.Body {
				visitStmtExprs(s, func(e Expr) {
					if graceRef(e, "slot", mir2.TyU16) {
						refs = append(refs, e)
					}
				})
			}
		})
		for _, ref := range refs {
			copy = rewriteGraceBlock(Blk(copy), ref, value).Body[0]
		}
		out.Body = append(out.Body, copy)
	}
	return out
}
