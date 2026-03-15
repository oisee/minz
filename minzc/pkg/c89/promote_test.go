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

func TestPromote_BraceInit_EndToEnd(t *testing.T) {
	// Compile C with struct brace init and verify StructLitExpr is produced.
	src := `
typedef struct { uint8_t q; uint8_t r; } Pair;

uint8_t test(void) {
    Pair p = { 10, 3 };
    return p.q + p.r;
}
`
	m, err := Compile(src, "test.c")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}

	// Check struct was created.
	if len(m.Structs) == 0 {
		t.Fatal("no structs found")
	}
	t.Logf("structs: %d", len(m.Structs))
	for _, st := range m.Structs {
		t.Logf("  struct %s { %d fields }", st.Name, len(st.Fields))
	}

	// Check that test() was compiled.
	var testFn *hir.Func
	for _, f := range m.Funcs {
		t.Logf("func %s — retTy=%v", f.Name, f.RetTy)
		if f.Name == "test" {
			testFn = f
		}
	}
	if testFn == nil {
		t.Fatal("test function not found")
	}

	// Walk body to find StructLitExpr or VarDeclStmt with struct init.
	foundInit := false
	for _, s := range testFn.Body.Body {
		if decl, ok := s.(*hir.VarDeclStmt); ok {
			t.Logf("VarDecl: %s init=%T", decl.Name, decl.Init)
			if _, isLit := decl.Init.(*hir.StructLitExpr); isLit {
				foundInit = true
				t.Log("  → StructLitExpr found!")
			}
		}
	}
	if !foundInit {
		t.Error("expected StructLitExpr from brace init { 10, 3 }")
	}
}

func TestPromote_OutParam(t *testing.T) {
	// Simulate: void divmod(u8 a, u8 b, DivResult *out) { out->q = a/b; out->r = a%b; }
	// Caller:   DivResult res; divmod(17, 5, &res); use(res.q + res.r);
	// Expected: (u8, u8) divmod(u8 a, u8 b) { return (a/b, a%b); }
	//           let (res_q, res_r) = divmod(17, 5); use(res_q + res_r);

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
			{Name: "out", Ty: mir2.TyPtr},
		},
		RetTy: mir2.TyVoid,
		Body: hir.Blk(
			// out.q = a / b
			&hir.AssignStmt{
				Target: &hir.FieldExpr{X: hir.Var("out", mir2.TyPtr), Field: "q", Ty: mir2.TyU8},
				Val:    &hir.BinExpr{Op: "/", L: hir.Var("a", mir2.TyU8), R: hir.Var("b", mir2.TyU8), Ty: mir2.TyU8},
			},
			// out.r = a % b
			&hir.AssignStmt{
				Target: &hir.FieldExpr{X: hir.Var("out", mir2.TyPtr), Field: "r", Ty: mir2.TyU8},
				Val:    &hir.BinExpr{Op: "%", L: hir.Var("a", mir2.TyU8), R: hir.Var("b", mir2.TyU8), Ty: mir2.TyU8},
			},
		),
	}

	caller := &hir.Func{
		Name:  "caller",
		RetTy: mir2.TyU8,
		Body: hir.Blk(
			// DivResult res; (zero-init)
			&hir.VarDeclStmt{Name: "res", Ty: mir2.TyPtr},
			// divmod(17, 5, &res)
			&hir.ExprStmt{
				Expr: &hir.CallExpr{
					Fn:  "divmod",
					Ty:  mir2.TyVoid,
					Args: []hir.Expr{
						hir.U8(17), hir.U8(5),
						&hir.UnaryExpr{Op: "&", X: hir.Var("res", mir2.TyPtr), Ty: mir2.TyPtr},
					},
				},
			},
			// return res.q + res.r
			&hir.ReturnStmt{
				Val: hir.Add(
					&hir.FieldExpr{X: hir.Var("res", mir2.TyPtr), Field: "q", Ty: mir2.TyU8},
					&hir.FieldExpr{X: hir.Var("res", mir2.TyPtr), Field: "r", Ty: mir2.TyU8},
					mir2.TyU8,
				),
			},
		),
	}

	m := &hir.Module{
		Name:    "test",
		Funcs:   []*hir.Func{divmod, caller},
		Structs: []*mir2.StructTy{divResult},
	}

	PromoteStructReturns(m)

	// Check divmod was promoted to tuple return.
	if len(divmod.RetTys) != 2 {
		t.Fatalf("divmod.RetTys = %v, want 2 elements", divmod.RetTys)
	}
	if divmod.RetTys[0] != mir2.TyU8 || divmod.RetTys[1] != mir2.TyU8 {
		t.Errorf("divmod.RetTys = %v, want [u8, u8]", divmod.RetTys)
	}
	t.Logf("divmod promoted to tuple return: %v", divmod.RetTys)

	// Check out-param was removed.
	if len(divmod.Params) != 2 {
		t.Fatalf("divmod.Params = %d, want 2 (out removed)", len(divmod.Params))
	}
	t.Logf("out-param removed: %d params remain", len(divmod.Params))

	// Check body ends with tuple return.
	lastStmt := divmod.Body.Body[len(divmod.Body.Body)-1]
	ret, ok := lastStmt.(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("last stmt = %T, want ReturnStmt", lastStmt)
	}
	if len(ret.Vals) != 2 {
		t.Fatalf("ret.Vals = %d, want 2", len(ret.Vals))
	}
	t.Logf("tuple return synthesized: %d values", len(ret.Vals))

	// Check caller: VarDeclStmt removed, ExprStmt → TupleLetStmt.
	tup, ok := caller.Body.Body[0].(*hir.TupleLetStmt)
	if !ok {
		t.Fatalf("caller body[0] = %T, want TupleLetStmt", caller.Body.Body[0])
	}
	if len(tup.Names) != 2 {
		t.Errorf("tuple names = %v, want 2", tup.Names)
	}
	t.Logf("call site → TupleLetStmt: names=%v", tup.Names)

	// Check field refs were rewritten.
	retStmt, ok := caller.Body.Body[1].(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("caller body[1] = %T, want ReturnStmt", caller.Body.Body[1])
	}
	add, ok := retStmt.Val.(*hir.BinExpr)
	if !ok {
		t.Fatalf("return val = %T, want BinExpr", retStmt.Val)
	}
	lRef, lok := add.L.(*hir.VarRefExpr)
	rRef, rok := add.R.(*hir.VarRefExpr)
	if !lok || !rok {
		t.Fatalf("add operands: L=%T, R=%T — want VarRefExpr", add.L, add.R)
	}
	t.Logf("res.q + res.r → %s + %s", lRef.Name, rRef.Name)
}

func TestPromote_PtrReturn(t *testing.T) {
	// Simulate: DivResult* make_pair(u8 a, u8 b) { DivResult tmp; tmp.q = a; tmp.r = b; return &tmp; }
	// Expected: (u8, u8) make_pair(u8 a, u8 b) { return (a, b); }

	divResult := &mir2.StructTy{
		Name: "DivResult",
		Fields: []mir2.StructField{
			{Name: "q", Ty: mir2.TyU8},
			{Name: "r", Ty: mir2.TyU8},
		},
	}

	makePair := &hir.Func{
		Name: "make_pair",
		Params: []hir.Param{
			{Name: "a", Ty: mir2.TyU8},
			{Name: "b", Ty: mir2.TyU8},
		},
		RetTy: mir2.TyPtr, // struct pointer return
		Body: hir.Blk(
			// DivResult tmp;
			&hir.VarDeclStmt{Name: "tmp", Ty: mir2.TyPtr},
			// tmp.q = a
			&hir.AssignStmt{
				Target: &hir.FieldExpr{X: hir.Var("tmp", mir2.TyPtr), Field: "q", Ty: mir2.TyU8},
				Val:    hir.Var("a", mir2.TyU8),
			},
			// tmp.r = b
			&hir.AssignStmt{
				Target: &hir.FieldExpr{X: hir.Var("tmp", mir2.TyPtr), Field: "r", Ty: mir2.TyU8},
				Val:    hir.Var("b", mir2.TyU8),
			},
			// return &tmp
			&hir.ReturnStmt{
				Val: &hir.UnaryExpr{Op: "&", X: hir.Var("tmp", mir2.TyPtr), Ty: mir2.TyPtr},
			},
		),
	}

	caller := &hir.Func{
		Name:  "caller",
		RetTy: mir2.TyU8,
		Body: hir.Blk(
			// let res = make_pair(10, 20)
			&hir.VarDeclStmt{
				Name: "res",
				Ty:   mir2.TyPtr,
				Init: hir.Call("make_pair", mir2.TyPtr, hir.U8(10), hir.U8(20)),
			},
			// return res.q + res.r
			&hir.ReturnStmt{
				Val: hir.Add(
					&hir.FieldExpr{X: hir.Var("res", mir2.TyPtr), Field: "q", Ty: mir2.TyU8},
					&hir.FieldExpr{X: hir.Var("res", mir2.TyPtr), Field: "r", Ty: mir2.TyU8},
					mir2.TyU8,
				),
			},
		),
	}

	m := &hir.Module{
		Name:    "test",
		Funcs:   []*hir.Func{makePair, caller},
		Structs: []*mir2.StructTy{divResult},
	}

	PromoteStructReturns(m)

	// Check make_pair was promoted to tuple return.
	if len(makePair.RetTys) != 2 {
		t.Fatalf("make_pair.RetTys = %v, want 2 elements", makePair.RetTys)
	}
	t.Logf("make_pair promoted to tuple return: %v", makePair.RetTys)

	// Check body: field assigns and var decl removed, tuple return added.
	lastStmt := makePair.Body.Body[len(makePair.Body.Body)-1]
	ret, ok := lastStmt.(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("last stmt = %T, want ReturnStmt", lastStmt)
	}
	if len(ret.Vals) != 2 {
		t.Fatalf("ret.Vals = %d, want 2", len(ret.Vals))
	}
	t.Logf("body stmts after promotion: %d (want 1: just the return)", len(makePair.Body.Body))
	if len(makePair.Body.Body) != 1 {
		for i, s := range makePair.Body.Body {
			t.Logf("  [%d] %T", i, s)
		}
	}

	// Check caller: call site rewritten to TupleLetStmt.
	tup, ok := caller.Body.Body[0].(*hir.TupleLetStmt)
	if !ok {
		t.Fatalf("caller body[0] = %T, want TupleLetStmt", caller.Body.Body[0])
	}
	t.Logf("call site → TupleLetStmt: names=%v", tup.Names)

	// Check field refs rewritten.
	retStmt, ok := caller.Body.Body[1].(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("caller body[1] = %T, want ReturnStmt", caller.Body.Body[1])
	}
	add, ok := retStmt.Val.(*hir.BinExpr)
	if !ok {
		t.Fatalf("return val = %T, want BinExpr", retStmt.Val)
	}
	lRef, lok := add.L.(*hir.VarRefExpr)
	rRef, rok := add.R.(*hir.VarRefExpr)
	if !lok || !rok {
		t.Fatalf("add operands: L=%T, R=%T — want VarRefExpr", add.L, add.R)
	}
	t.Logf("res.q + res.r → %s + %s", lRef.Name, rRef.Name)
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
