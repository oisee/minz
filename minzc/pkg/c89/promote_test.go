package c89

import (
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

func TestPromote_DivmodStruct(t *testing.T) {
	// Simulate: typedef struct { u8 q; u8 r; } DivResult;
	// DivResult divmod(u8 a, u8 b) { return (DivResult){a/b, a%b}; }
	// void test() { DivResult res = divmod(17,5); u8 q = res.q; u8 r = res.r; }

	divResult := &mir2.StructTy{
		Name: "DivResult",
		Fields: []mir2.StructField{
			{Name: "q", Ty: mir2.TyU8},
			{Name: "r", Ty: mir2.TyU8},
		},
	}

	divmod := &hir.Func{
		Name: "divmod",
		Params: []hir.Param{
			{Name: "a", Ty: mir2.TyU8},
			{Name: "b", Ty: mir2.TyU8},
		},
		RetTy: mir2.TyPtr, // struct return = pointer before promotion
		Body: hir.Blk(
			&hir.ReturnStmt{
				Val: &hir.StructLitExpr{
					St: divResult,
					Fields: []hir.FieldInit{
						{Name: "q", Val: &hir.BinExpr{Op: "/", L: hir.Var("a", mir2.TyU8), R: hir.Var("b", mir2.TyU8), Ty: mir2.TyU8}},
						{Name: "r", Val: &hir.BinExpr{Op: "%", L: hir.Var("a", mir2.TyU8), R: hir.Var("b", mir2.TyU8), Ty: mir2.TyU8}},
					},
				},
			},
		),
	}

	test := &hir.Func{
		Name:  "test",
		RetTy: mir2.TyVoid,
		Body: hir.Blk(
			// let res = divmod(17, 5)
			&hir.VarDeclStmt{
				Name: "res",
				Ty:   mir2.TyPtr,
				Init: hir.Call("divmod", mir2.TyPtr, hir.U8(17), hir.U8(5)),
			},
			// let q = res.q
			&hir.VarDeclStmt{
				Name: "q",
				Ty:   mir2.TyU8,
				Init: &hir.FieldExpr{X: hir.Var("res", mir2.TyPtr), Field: "q", Ty: mir2.TyU8},
			},
			// let r = res.r
			&hir.VarDeclStmt{
				Name: "r",
				Ty:   mir2.TyU8,
				Init: &hir.FieldExpr{X: hir.Var("res", mir2.TyPtr), Field: "r", Ty: mir2.TyU8},
			},
			// return call using field: assert(res.q + res.r)
			&hir.ExprStmt{
				Expr: hir.Call("assert", mir2.TyVoid,
					hir.Add(
						&hir.FieldExpr{X: hir.Var("res", mir2.TyPtr), Field: "q", Ty: mir2.TyU8},
						&hir.FieldExpr{X: hir.Var("res", mir2.TyPtr), Field: "r", Ty: mir2.TyU8},
						mir2.TyU8,
					),
				),
			},
		),
	}

	m := &hir.Module{
		Name:    "test",
		Funcs:   []*hir.Func{divmod, test},
		Structs: []*mir2.StructTy{divResult},
	}

	// Run promotion.
	PromoteStructReturns(m)

	// Check divmod was promoted to tuple return.
	if len(divmod.RetTys) != 2 {
		t.Fatalf("divmod.RetTys = %v, want 2 elements", divmod.RetTys)
	}
	if divmod.RetTys[0] != mir2.TyU8 || divmod.RetTys[1] != mir2.TyU8 {
		t.Errorf("divmod.RetTys = %v, want [u8, u8]", divmod.RetTys)
	}
	t.Logf("divmod promoted to tuple return: %v", divmod.RetTys)

	// Check return was rewritten to multi-return.
	ret, ok := divmod.Body.Body[0].(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("divmod body[0] = %T, want ReturnStmt", divmod.Body.Body[0])
	}
	if len(ret.Vals) != 2 {
		t.Fatalf("ret.Vals = %d, want 2", len(ret.Vals))
	}
	t.Logf("return rewritten to tuple: %d values", len(ret.Vals))

	// Check call site was rewritten to TupleLetStmt.
	tup, ok := test.Body.Body[0].(*hir.TupleLetStmt)
	if !ok {
		t.Fatalf("test body[0] = %T, want TupleLetStmt", test.Body.Body[0])
	}
	if len(tup.Names) != 2 {
		t.Errorf("tuple names = %v, want 2", tup.Names)
	}
	t.Logf("call site → TupleLetStmt: names=%v, tys=%v", tup.Names, tup.Tys)

	// Check field refs were rewritten to var refs.
	decl1, ok := test.Body.Body[1].(*hir.VarDeclStmt)
	if !ok {
		t.Fatalf("test body[1] = %T, want VarDeclStmt", test.Body.Body[1])
	}
	ref1, ok := decl1.Init.(*hir.VarRefExpr)
	if !ok {
		t.Fatalf("q init = %T, want VarRefExpr (was FieldExpr)", decl1.Init)
	}
	t.Logf("res.q → %s", ref1.Name)

	decl2, ok := test.Body.Body[2].(*hir.VarDeclStmt)
	if !ok {
		t.Fatalf("test body[2] = %T, want VarDeclStmt", test.Body.Body[2])
	}
	ref2, ok := decl2.Init.(*hir.VarRefExpr)
	if !ok {
		t.Fatalf("r init = %T, want VarRefExpr (was FieldExpr)", decl2.Init)
	}
	t.Logf("res.r → %s", ref2.Name)

	// Check nested field refs in expression were also rewritten.
	exprSt, ok := test.Body.Body[3].(*hir.ExprStmt)
	if !ok {
		t.Fatalf("test body[3] = %T, want ExprStmt", test.Body.Body[3])
	}
	call, ok := exprSt.Expr.(*hir.CallExpr)
	if !ok {
		t.Fatalf("expr = %T, want CallExpr", exprSt.Expr)
	}
	add, ok := call.Args[0].(*hir.BinExpr)
	if !ok {
		t.Fatalf("assert arg = %T, want BinExpr", call.Args[0])
	}
	lRef, lok := add.L.(*hir.VarRefExpr)
	rRef, rok := add.R.(*hir.VarRefExpr)
	if !lok || !rok {
		t.Fatalf("add operands: L=%T, R=%T — want VarRefExpr", add.L, add.R)
	}
	t.Logf("res.q + res.r → %s + %s", lRef.Name, rRef.Name)
}

func TestPromote_NotEligible_LargeStruct(t *testing.T) {
	// Struct with >4 bytes should not be promoted.
	big := &mir2.StructTy{
		Name: "Big",
		Fields: []mir2.StructField{
			{Name: "a", Ty: mir2.TyU16},
			{Name: "b", Ty: mir2.TyU16},
			{Name: "c", Ty: mir2.TyU16}, // 6 bytes > 4
		},
	}

	fn := &hir.Func{
		Name:  "getBig",
		RetTy: mir2.TyPtr,
		Body: hir.Blk(
			&hir.ReturnStmt{Val: &hir.StructLitExpr{
				St: big,
				Fields: []hir.FieldInit{
					{Name: "a", Val: hir.U16(1)},
					{Name: "b", Val: hir.U16(2)},
					{Name: "c", Val: hir.U16(3)},
				},
			}},
		),
	}

	m := &hir.Module{
		Funcs:   []*hir.Func{fn},
		Structs: []*mir2.StructTy{big},
	}

	PromoteStructReturns(m)

	if len(fn.RetTys) != 0 {
		t.Errorf("large struct should not be promoted, but RetTys = %v", fn.RetTys)
	}
	t.Log("large struct correctly rejected")
}

func TestPromote_SSS_Detection(t *testing.T) {
	tests := []struct {
		name string
		st   *mir2.StructTy
		want bool
	}{
		{"2xu8", &mir2.StructTy{Fields: []mir2.StructField{
			{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU8},
		}}, true},
		{"u8+u16", &mir2.StructTy{Fields: []mir2.StructField{
			{Name: "a", Ty: mir2.TyU8}, {Name: "b", Ty: mir2.TyU16},
		}}, true},
		{"2xu16", &mir2.StructTy{Fields: []mir2.StructField{
			{Name: "a", Ty: mir2.TyU16}, {Name: "b", Ty: mir2.TyU16},
		}}, true},
		{"3xu16=6bytes", &mir2.StructTy{Fields: []mir2.StructField{
			{Name: "a", Ty: mir2.TyU16}, {Name: "b", Ty: mir2.TyU16}, {Name: "c", Ty: mir2.TyU16},
		}}, false},
		{"empty", &mir2.StructTy{}, false},
		{"nil", nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isShallowScalarStruct(tt.st)
			if got != tt.want {
				t.Errorf("isSSS(%s) = %v, want %v", tt.name, got, tt.want)
			}
		})
	}
}
