package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// TestPropagateClassHints_ReturnValue verifies that a register used directly
// in TermRet gets upgraded to ClassAcc by propagateClassHints.
func TestPropagateClassHints_ReturnValue(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("f")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	pa := bld.Param("a", mir2.TyU8, mir2.ClassGeneral)
	r := bld.Add(pa, pa, mir2.TyU8, mir2.ClassGeneral)
	bld.Ret(r)

	info := mir2.CollectRegInfoForTest(f)
	mir2.PropagateClassHintsForTest(f, info)

	if info[r].Cls != mir2.ClassAcc {
		t.Errorf("return value: got class %v, want ClassAcc", info[r].Cls)
	}
}

// TestPropagateClassHints_PhiWeb verifies that ClassAcc propagates backward
// through block-arg edges so that all members of the phi-web get ClassAcc.
func TestPropagateClassHints_PhiWeb(t *testing.T) {
	// entry: r_c = Sub(a, b); BrIf(cond, @then1(r_c), @join2(r_c))
	// then1(p1): TermRet(p1)
	// join2(p2): TermRet(p2)
	// After propagation: r_c, p1, p2 should all be ClassAcc.
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("abs_diff")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	pa := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	pb := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)

	r_c := bld.Sub(pa, pb, mir2.TyU8, mir2.ClassGeneral)
	r_cond := bld.Cmp(mir2.CmpGe, pa, pb, mir2.ClassFlag, false)

	then1 := f.NewBlock("then1")
	join2 := f.NewBlock("join2")

	p1 := bld.BlockParam(then1, mir2.TyU8, mir2.ClassGeneral)
	p2 := bld.BlockParam(join2, mir2.TyU8, mir2.ClassGeneral)

	bld.BrIf(r_cond, "then1", []mir2.Reg{r_c}, "join2", []mir2.Reg{r_c})

	bld.SwitchTo(then1)
	bld.Ret(p1)

	bld.SwitchTo(join2)
	bld.Ret(p2)

	info := mir2.CollectRegInfoForTest(f)
	mir2.PropagateClassHintsForTest(f, info)

	for name, r := range map[string]mir2.Reg{"r_c": r_c, "p1": p1, "p2": p2} {
		if info[r].Cls != mir2.ClassAcc {
			t.Errorf("%s: got class %v, want ClassAcc", name, info[r].Cls)
		}
	}
}

// TestIsaAlwaysWritesA verifies that an 8-bit ALU Sub result lands in A after
// allocation (indirectly testing the isaAlwaysWritesA helper via coalescing).
func TestIsaAlwaysWritesA(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("sub8")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	pa := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	pb := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	r := bld.Sub(pa, pb, mir2.TyU8, mir2.ClassGeneral)
	bld.Ret(r)

	ct := mir2.Z80CostTable{}
	lr := mir2.ComputeLiveness(f)
	// PBQPAllocate already runs coalesceAllocResult internally.
	ar := mir2.PBQPAllocate(f, lr, ct)

	if ar.Locs[r].Name != "A" {
		t.Errorf("Sub result allocated to %v, want A", ar.Locs[r])
	}
}

// TestZ80Codegen_SubThroughBlockParamNoSpill verifies that when a Sub result
// flows through a block parameter to a return, the allocator avoids a spurious
// LD C,A (i.e., the sub result lands in A throughout the phi-web).
//
// Structure:
//
//	entry(a, b, cond):
//	    r = sub(a, b)    ; a and b are dead after this point
//	    BrIf(cond, @then(r), @else(r))
//	then(p):  ret p
//	else(p):  ret p
func TestZ80Codegen_SubThroughBlockParamNoSpill(t *testing.T) {
	m := &mir2.Module{Name: "test"}
	f := m.AddFunc("sub_phi")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	pa := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	pb := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	pcond := bld.Param("cond", mir2.TyBool, mir2.ClassFlag)

	// a and b are only used in the Sub; they die here.
	r_c := bld.Sub(pa, pb, mir2.TyU8, mir2.ClassGeneral)

	thenBlk := f.NewBlock("then")
	elseBlk := f.NewBlock("else")

	p1 := bld.BlockParam(thenBlk, mir2.TyU8, mir2.ClassGeneral)
	p2 := bld.BlockParam(elseBlk, mir2.TyU8, mir2.ClassGeneral)

	bld.BrIf(pcond, "then", []mir2.Reg{r_c}, "else", []mir2.Reg{r_c})

	bld.SwitchTo(thenBlk)
	bld.Ret(p1)

	bld.SwitchTo(elseBlk)
	bld.Ret(p2)

	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}

	ct := mir2.Z80CostTable{}
	lr := mir2.ComputeLiveness(f)
	ar := mir2.PBQPAllocate(f, lr, ct)
	asm := mir2.Z80Codegen(m, ar)

	t.Logf("Z80 output:\n%s", asm)

	// Must NOT contain the spurious save/restore pattern.
	if strings.Contains(asm, "LD C, A") {
		t.Errorf("emitted spurious LD C, A:\n%s", asm)
	}
	// The phi-web members should all land in A.
	if ar.Locs[r_c].Name != "A" {
		t.Errorf("r_c allocated to %v, want A", ar.Locs[r_c])
	}
	if ar.Locs[p1].Name != "A" {
		t.Errorf("p1 allocated to %v, want A", ar.Locs[p1])
	}
	if ar.Locs[p2].Name != "A" {
		t.Errorf("p2 allocated to %v, want A", ar.Locs[p2])
	}
}
