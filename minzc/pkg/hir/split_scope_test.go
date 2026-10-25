package hir

import (
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
	"testing"
)

func TestSplitBranchLocalsStayLocal(t *testing.T) {
	f := &Func{Name: "scope", RetTy: mir2.TyU8, Params: []Param{{Name: "x", Ty: mir2.TyU8}}, Body: Blk(
		&ExprStmt{Expr: Var("x", mir2.TyU8)},
		&IfStmt{Cond: &BoolLitExpr{Val: false}, Then: Blk(&VarDeclStmt{Name: "s", Ty: mir2.TyU16, Init: U16(9)})},
		&IfStmt{Cond: &BoolLitExpr{Val: true}, Then: Blk(&VarDeclStmt{Name: "s", Ty: mir2.TyU8, Init: Var("x", mir2.TyU8)}, &ReturnStmt{Val: Var("s", mir2.TyU8)})},
		&ReturnStmt{Val: U8(0)},
	)}
	m := &Module{Funcs: []*Func{f}}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	candidates := FindSplitPoints(f, []int{9, 9, 9, 9})
	if len(candidates) == 0 || !reflect.DeepEqual(candidates[0].inputs, []string{"x"}) {
		t.Fatalf("branch bindings leaked into interface: %+v", candidates)
	}
	loop := &WhileStmt{Cond: &BoolLitExpr{Val: false}, Body: Blk()}
	f.Body.Body[2].(*IfStmt).Then.Body = append([]Stmt{loop}, f.Body.Body[2].(*IfStmt).Then.Body...)
	origin := SourceLocation{Unit: "scope.nanz", SourceSHA256: "0123456789012345678901234567890123456789012345678901234567890123", Line: 1, Column: 1}
	m.SourceLocations = map[Stmt]SourceLocation{loop: origin}
	m.LoopLineage = map[Stmt]LoopLineage{loop: {SiteID: "test", Role: "original"}}
	sub := ApplySplit(m, f, candidates[0])
	if sub == nil {
		t.Fatal("legal scoped split refused")
	}
	m.Funcs = append(m.Funcs, sub)
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	if m.SourceLocations[loop] != origin || m.LoopLineage[loop].SiteID != "test" {
		t.Fatal("moved loop lost origin/lineage")
	}
	for _, value := range []int64{0, 1, 255} {
		got, err := mir2.NewVM(LowerModule(m)).Call("scope", []mir2.Value{{I: value}})
		if err != nil || len(got) != 1 || got[0].I != value {
			t.Fatalf("scope%d: %v %v", value, got, err)
		}
	}
}

func TestSplitInvalidCandidateRollsBack(t *testing.T) {
	f := &Func{Name: "rollback", RetTy: mir2.TyU8, Params: []Param{{Name: "x", Ty: mir2.TyU8}}, Body: Blk(&ExprStmt{Expr: U8(0)}, &ExprStmt{Expr: U8(1)}, &ReturnStmt{Val: Var("x", mir2.TyU8)})}
	m := &Module{Funcs: []*Func{f}}
	old := append([]Stmt(nil), f.Body.Body...)
	if sub := ApplySplit(m, f, splitCandidate{splitAt: 1}); sub != nil {
		t.Fatal("invalid free-variable interface published")
	}
	if !reflect.DeepEqual(old, f.Body.Body) || len(m.Funcs) != 1 {
		t.Fatal("rejected split mutated module")
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
}

func TestSplitCallableAliasInterfaceRefused(t *testing.T) {
	callee := &Func{Name: "callee", RetTy: mir2.TyU8, Params: []Param{{Name: "v", Ty: mir2.TyU8}}, Body: Blk(&ReturnStmt{Val: Var("v", mir2.TyU8)})}
	f := &Func{Name: "aliascaller", RetTy: mir2.TyU8, Body: Blk(&VarDeclStmt{Name: "alias", Ty: mir2.TyU8, Init: Var("callee", mir2.TyU8)}, &ExprStmt{Expr: U8(0)}, &ReturnStmt{Val: &CallExpr{Fn: "alias", Args: []Expr{U8(7)}, Ty: mir2.TyU8}}, &ReturnStmt{Val: U8(0)})}
	m := &Module{Funcs: []*Func{f, callee}}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	candidates := FindSplitPoints(f, []int{9, 9, 9, 9})
	if len(candidates) == 0 || !reflect.DeepEqual(candidates[0].inputs, []string{"alias"}) {
		t.Fatalf("alias dependency missing: %+v", candidates)
	}
	if sub := ApplySplit(m, f, candidates[0]); sub != nil {
		t.Fatal("callable alias scalar forwarding accepted")
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
}

func TestSplitStandaloneBlockBindingsDoNotEscape(t *testing.T) {
	refs := map[string]bool{}
	bound := map[string]bool{}
	splitFreeRefs(Blk(&VarDeclStmt{Name: "s", Ty: mir2.TyU8, Init: U8(3)}, &ExprStmt{Expr: Var("s", mir2.TyU8)}), bound, refs)
	splitFreeRefs(&ReturnStmt{Val: Var("s", mir2.TyU8)}, bound, refs)
	if !refs["s"] || bound["s"] {
		t.Fatalf("nested block binding escaped: bound%v refs%v", bound, refs)
	}
}

func TestSplitBlockShadowOuterInterface(t *testing.T) {
	f := &Func{Name: "shadow", RetTy: mir2.TyU8, Params: []Param{{Name: "s", Ty: mir2.TyU8}}, Body: Blk(&ExprStmt{Expr: U8(0)}, &ExprStmt{Expr: U8(1)}, Blk(&VarDeclStmt{Name: "s", Ty: mir2.TyU16, Init: U16(999)}), &ReturnStmt{Val: Var("s", mir2.TyU8)})}
	m := &Module{Funcs: []*Func{f}}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	candidates := FindSplitPoints(f, []int{9, 9, 9, 9})
	if len(candidates) == 0 || !reflect.DeepEqual(candidates[0].inputs, []string{"s"}) {
		t.Fatalf("outer shadow dependency lost: %+v", candidates)
	}
	sub := ApplySplit(m, f, candidates[0])
	if sub == nil {
		t.Fatal("valid block shadow interface refused")
	}
	m.Funcs = append(m.Funcs, sub)
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	// LowerModule standalone-Block shadowing is a separate known issue.
	// This regression checks the exact lexical interface, not that backend bug.

}

func TestSplitMissingCallbackCaptureRollsBack(t *testing.T) {
	closure := &Func{Name: "closure", RetTy: mir2.TyU8, Captures: []Param{{Name: "x", Ty: mir2.TyU8}}, Body: Blk(&ReturnStmt{Val: Var("x", mir2.TyU8)})}
	f := &Func{Name: "capturecaller", RetTy: mir2.TyU8, Body: Blk(Decl("x", mir2.TyU8, U8(3)), &ExprStmt{Expr: U8(0)}, &ExprStmt{Expr: Var("closure", mir2.TyU8)}, &ReturnStmt{Val: U8(0)})}
	m := &Module{Funcs: []*Func{f, closure}}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
	old := append([]Stmt(nil), f.Body.Body...)
	if sub := ApplySplit(m, f, splitCandidate{splitAt: 1}); sub != nil {
		t.Fatal("extraction omitted named callable capture")
	}
	if !reflect.DeepEqual(old, f.Body.Body) {
		t.Fatal("invalid capture extraction not rolled back")
	}
	if err := Verify(m); err != nil {
		t.Fatal(err)
	}
}

func TestSplitAddressedStorageInterface(t *testing.T) {
	for _, arrayLen := range []int{0, 3} {
		f := &Func{Name: "address", RetTy: mir2.TyU8, Body: Blk(
			&VarDeclStmt{Name: "cell", Ty: mir2.TyU8, ArrayLen: arrayLen, Init: U8(7)},
			&ExprStmt{Expr: U8(0)},
			&ExprStmt{Expr: &AddrOfExpr{Sym: "cell"}},
			&ReturnStmt{Val: U8(0)},
		)}
		refs := map[string]bool{}
		collectExprRefs(&AddrOfExpr{Sym: "cell"}, refs)
		if !refs["cell"] {
			t.Fatal("address dependency omitted")
		}
		for _, c := range FindSplitPoints(f, []int{9, 9, 9, 9}) {
			if c.splitAt == 1 {
				t.Fatalf("arrayLen%d: addressable storage forwarded as scalar: %+v", arrayLen, c)
			}
		}
	}
}
