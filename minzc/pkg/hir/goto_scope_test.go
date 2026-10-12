package hir_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"strings"
	"testing"
)

func TestGotoScopeVMZ80(t *testing.T) {
	v := func(n string) hir.Expr { return hir.Var(n, mir2.TyU16) }
	set := func(n string, val int64) hir.Stmt { return &hir.AssignStmt{Target: v(n), Val: hir.U16(val)} }
	inc := func(n string) hir.Stmt {
		return &hir.AssignStmt{Target: v(n), Val: &hir.BinExpr{Op: "+", L: v(n), R: hir.U16(1), Ty: mir2.TyU16}}
	}
	cases := []struct {
		name string
		body *hir.Block
		want int64
	}{
		{"backward_outer_shadow", hir.Blk(hir.Decl("i", mir2.TyU16, hir.U16(0)), &hir.LabelStmt{Name: "again"}, hir.Blk(inc("i"), hir.Decl("x", mir2.TyU16, hir.U16(99)), &hir.IfStmt{Cond: &hir.BinExpr{Op: "<", L: v("i"), R: hir.U16(2), Ty: mir2.TyBool}, Then: hir.Blk(&hir.GotoStmt{Label: "again"})}), &hir.ReturnStmt{Val: v("x")}), 7},
		{"forward_outer_mutation", hir.Blk(&hir.IfStmt{Cond: &hir.BoolLitExpr{Val: true}, Then: hir.Blk(set("x", 9), hir.Blk(hir.Decl("x", mir2.TyU16, hir.U16(99)), &hir.GotoStmt{Label: "done"}))}, set("x", 88), &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: v("x")}), 9},
		{"forward_skip", hir.Blk(&hir.GotoStmt{Label: "done"}, set("x", 99), &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: v("x")}), 7},
		{"into_empty_block", hir.Blk(&hir.GotoStmt{Label: "done"}, hir.Blk(&hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: v("x")})), 7},
	}
	cases = append(cases,
		struct {
			name string
			body *hir.Block
			want int64
		}{"skip_unrelated_block", hir.Blk(&hir.GotoStmt{Label: "done"}, hir.Blk(set("x", 99)), hir.Blk(set("x", 88), &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: v("x")})), 7},
		struct {
			name string
			body *hir.Block
			want int64
		}{"into_false_if", hir.Blk(&hir.GotoStmt{Label: "done"}, &hir.IfStmt{Cond: &hir.BoolLitExpr{Val: false}, Then: hir.Blk(set("x", 99), &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: v("x")})}), 7},
	)
	cases = append(cases,
		struct {
			name string
			body *hir.Block
			want int64
		}{"transitive_labels", hir.Blk(set("x", 9), &hir.GotoStmt{Label: "a"}, set("x", 88), &hir.LabelStmt{Name: "a"}, &hir.GotoStmt{Label: "b"}, &hir.LabelStmt{Name: "b"}, &hir.ReturnStmt{Val: v("x")}), 9},
		struct {
			name string
			body *hir.Block
			want int64
		}{"unused_skipped_local", hir.Blk(&hir.GotoStmt{Label: "done"}, hir.Decl("unused", mir2.TyU16, hir.U16(99)), &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: v("x")}), 7},
		struct {
			name string
			body *hir.Block
			want int64
		}{"range_outward", hir.Blk(&hir.ForRangeStmt{Var: "j", Start: hir.U16(0), End: hir.U16(3), Body: hir.Blk(set("x", 9), hir.Blk(hir.Decl("x", mir2.TyU16, hir.U16(99)), &hir.GotoStmt{Label: "done"}))}, &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: v("x")}), 9},
	)
	cases = append(cases,
		struct {
			name string
			body *hir.Block
			want int64
		}{"outer_local_shadow", hir.Blk(hir.Decl("local", mir2.TyU16, hir.U16(7)), hir.Decl("i", mir2.TyU16, hir.U16(0)), &hir.LabelStmt{Name: "again"}, hir.Blk(inc("i"), hir.Decl("local", mir2.TyU16, hir.U16(99)), &hir.IfStmt{Cond: &hir.BinExpr{Op: "<", L: v("i"), R: hir.U16(2), Ty: mir2.TyBool}, Then: hir.Blk(&hir.GotoStmt{Label: "again"})}), &hir.ReturnStmt{Val: v("local")}), 7},
		struct {
			name string
			body *hir.Block
			want int64
		}{"skipped_then_assigned", hir.Blk(&hir.GotoStmt{Label: "done"}, hir.Decl("unused", mir2.TyU16, hir.U16(99)), &hir.LabelStmt{Name: "done"}, set("unused", 9), &hir.ReturnStmt{Val: v("unused")}), 9},
	)
	cases = append(cases, struct {
		name string
		body *hir.Block
		want int64
	}{"into_false_loop_return", hir.Blk(&hir.GotoStmt{Label: "done"}, &hir.WhileStmt{Cond: &hir.BoolLitExpr{Val: false}, Body: hir.Blk(set("x", 99), &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: v("x")})}), 7})
	cases = append(cases, struct {
		name string
		body *hir.Block
		want int64
	}{"into_loop_fallthrough", hir.Blk(&hir.GotoStmt{Label: "done"}, &hir.WhileStmt{Cond: &hir.BinExpr{Op: "<", L: v("x"), R: hir.U16(10), Ty: mir2.TyBool}, Body: hir.Blk(set("x", 99), &hir.LabelStmt{Name: "done"}, inc("x"))}, &hir.ReturnStmt{Val: v("x")}), 10})
	cases = append(cases, struct {
		name string
		body *hir.Block
		want int64
	}{"transitive_killed", hir.Blk(&hir.GotoStmt{Label: "a"}, hir.Decl("local", mir2.TyU16, hir.U16(99)), &hir.LabelStmt{Name: "a"}, set("local", 9), &hir.GotoStmt{Label: "b"}, &hir.LabelStmt{Name: "b"}, &hir.ReturnStmt{Val: v("local")}), 9})
	cases = append(cases, struct {
		name string
		body *hir.Block
		want int64
	}{"into_switch_fallthrough", hir.Blk(&hir.GotoStmt{Label: "done"}, &hir.SwitchStmt{Val: v("x"), Cases: []*hir.SwitchCase{{Val: 1, Body: hir.Blk(set("x", 99), &hir.LabelStmt{Name: "done"}, set("x", 9))}}}, &hir.ReturnStmt{Val: v("x")}), 9})
	cases = append(cases, struct {
		name string
		body *hir.Block
		want int64
	}{"into_range_return", hir.Blk(&hir.GotoStmt{Label: "done"}, &hir.ForRangeStmt{Var: "i", Start: hir.U16(0), End: hir.U16(3), Body: hir.Blk(set("x", 99), &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: v("x")})}), 7})
	cases = append(cases, struct {
		name string
		body *hir.Block
		want int64
	}{"into_each_return", hir.Blk(&hir.GotoStmt{Label: "done"}, &hir.ForEachStmt{Var: "i", Ptr: &hir.CastExpr{X: hir.U16(4096), Ty: mir2.TyPtr}, Start: hir.U16(0), Len: hir.U16(3), ElemTy: mir2.TyU8, Body: hir.Blk(set("x", 99), &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: v("x")})}), 7})
	for _, iterator := range []bool{false, true} {
		body := hir.Blk(&hir.LabelStmt{Name: "entry"}, set("x", 9), &hir.GotoStmt{Label: "done"})
		var loop hir.Stmt = &hir.ForRangeStmt{Var: "i", Start: hir.U16(0), End: hir.U16(3), Body: body}
		if iterator {
			loop = &hir.ForEachStmt{Var: "i", Ptr: &hir.CastExpr{X: hir.U16(4096), Ty: mir2.TyPtr}, Start: hir.U16(0), Len: hir.U16(3), ElemTy: mir2.TyU8, Body: body}
		}
		cases = append(cases, struct {
			name string
			body *hir.Block
			want int64
		}{fmt.Sprintf("into_loop_outward_%v", iterator), hir.Blk(&hir.GotoStmt{Label: "entry"}, loop, set("x", 99), &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: v("x")}), 9})
	}
	for _, flag := range []bool{false, true} {
		want := int64(11)
		if flag {
			want = 9
		}
		cases = append(cases, struct {
			name string
			body *hir.Block
			want int64
		}{fmt.Sprintf("into_if_two_labels_%v", flag), hir.Blk(&hir.IfStmt{Cond: &hir.BoolLitExpr{Val: flag}, Then: hir.Blk(&hir.GotoStmt{Label: "a"}), Else: hir.Blk(&hir.GotoStmt{Label: "b"})}, &hir.IfStmt{Cond: &hir.BoolLitExpr{Val: false}, Then: hir.Blk(set("x", 99), &hir.LabelStmt{Name: "a"}, set("x", 9)), Else: hir.Blk(set("x", 88), &hir.LabelStmt{Name: "b"}, set("x", 11))}, &hir.ReturnStmt{Val: v("x")}), want})
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			factory := func() *hir.Module {
				return &hir.Module{SourceUnitsComplete: true, SourceUnits: map[string]string{"goto.nanz": "0123456789012345678901234567890123456789012345678901234567890123"}, Funcs: []*hir.Func{{Name: "goto_case", RetTy: mir2.TyU16, Params: []hir.Param{{Name: "x", Ty: mir2.TyU16}}, Body: c.body}}}
			}
			m := factory()
			m.SourceLocations = map[hir.Stmt]hir.SourceLocation{}
			for _, stmt := range c.body.Body {
				switch stmt.(type) {
				case *hir.ForRangeStmt, *hir.WhileStmt, *hir.ForEachStmt:
					m.SourceLocations[stmt] = hir.SourceLocation{Unit: "goto.nanz", SourceSHA256: "0123456789012345678901234567890123456789012345678901234567890123", Line: 1, Column: 1}
				}
			}
			if e := hir.Verify(m); e != nil {
				t.Fatal(e)
			}
			literal := int64(7)
			w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 1000000, MaxMemory: 65536, ExactDataAccess: true, Resources: map[string][]byte{}, Globals: map[string][]byte{}, Calls: []hir.ProfileInvocation{{Function: "goto_case", Arguments: []hir.ProfileArgument{{Value: &literal}}}}}
			mm := hir.LowerModule(m)
			got, e := mir2.NewVM(mm).Call("goto_case", []mir2.Value{{I: 7}})
			if e != nil || len(got) != 1 || got[0].I != c.want {
				t.Fatalf("VMgot%v want%d err%v", got, c.want, e)
			}
			raw, e := hir.RunLoopProfile(m, w)
			if e != nil {
				t.Fatal(e)
			}
			if raw.Results[0][0].I != c.want {
				t.Fatal("raw host disagreement")
			}
			steps, e := pipeline.CompileHIRSteps(m, pipeline.Options{AssertMode: "none", UseHIRGrace: false})
			if e != nil {
				t.Fatal(e)
			}
			run, e := pipeline.RunZ80ProfileWorkload(steps, m, w, *raw)
			if e != nil {
				t.Fatal(e)
			}
			if run.Invocations[0].Results[0].I != c.want {
				t.Fatal("target mismatch")
			}
		})
	}
}

func TestGotoSkippedUsedBindingRejected(t *testing.T) {
	f := &hir.Func{Name: "bad", RetTy: mir2.TyU16, Body: hir.Blk(&hir.GotoStmt{Label: "done"}, hir.Decl("x", mir2.TyU16, hir.U16(99)), &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: hir.Var("x", mir2.TyU16)})}
	if e := hir.Verify(&hir.Module{Funcs: []*hir.Func{f}}); e == nil {
		t.Fatal("jump manufactured skipped local")
	}
}

func TestGotoLocalMutationAcrossScope(t *testing.T) {
	// Explicit local binding at the target must survive a different local shadow.
	x := hir.Var("local", mir2.TyU16)
	f := &hir.Func{Name: "local", RetTy: mir2.TyU16, Body: hir.Blk(hir.Decl("local", mir2.TyU16, hir.U16(7)), hir.Blk(&hir.AssignStmt{Target: x, Val: hir.U16(9)}, hir.Decl("local", mir2.TyU16, hir.U16(99)), &hir.GotoStmt{Label: "done"}), &hir.LabelStmt{Name: "done"}, &hir.ReturnStmt{Val: x})}
	m := &hir.Module{Funcs: []*hir.Func{f}}
	if e := hir.Verify(m); e != nil {
		t.Fatal(e)
	}
	got, e := mir2.NewVM(hir.LowerModule(m)).Call("local", nil)
	if e != nil || got[0].I != 9 {
		t.Fatalf("got%v want9 err%v", got, e)
	}
}

func TestGotoSkippedInductionRejected(t *testing.T) {
	for _, end := range []hir.Stmt{&hir.BreakStmt{}, &hir.ContinueStmt{}, &hir.AssignStmt{Target: hir.Var("x", mir2.TyU16), Val: hir.U16(9)}} {
		m := &hir.Module{Funcs: []*hir.Func{{Name: "induction", RetTy: mir2.TyU16, Params: []hir.Param{{Name: "x", Ty: mir2.TyU16}}, Body: hir.Blk(&hir.GotoStmt{Label: "done"}, &hir.ForRangeStmt{Var: "i", Start: hir.U16(0), End: hir.U16(3), Body: hir.Blk(&hir.LabelStmt{Name: "done"}, end)}, &hir.ReturnStmt{Val: hir.Var("x", mir2.TyU16)})}}}
		if err := hir.Verify(m); err == nil || !strings.Contains(err.Error(), "uninitialized induction") {
			t.Fatalf("expected induction error, got %v", err)
		}
	}
}

func TestGotoSkippedIteratorRejected(t *testing.T) {
	for _, end := range []hir.Stmt{&hir.BreakStmt{}, &hir.ContinueStmt{}, &hir.AssignStmt{Target: hir.Var("x", mir2.TyU16), Val: hir.U16(9)}} {
		m := &hir.Module{Funcs: []*hir.Func{{Name: "iterator", RetTy: mir2.TyU16, Params: []hir.Param{{Name: "x", Ty: mir2.TyU16}}, Body: hir.Blk(&hir.GotoStmt{Label: "done"}, &hir.ForEachStmt{Var: "i", Ptr: &hir.CastExpr{X: hir.U16(4096), Ty: mir2.TyPtr}, Start: hir.U16(0), Len: hir.U16(3), ElemTy: mir2.TyU8, Body: hir.Blk(&hir.LabelStmt{Name: "done"}, end)}, &hir.ReturnStmt{Val: hir.Var("x", mir2.TyU16)})}}}
		if err := hir.Verify(m); err == nil || !strings.Contains(err.Error(), "uninitialized induction") {
			t.Fatalf("expected induction error, got %v", err)
		}
	}
}
