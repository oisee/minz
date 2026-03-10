package mir2_test

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// buildAbsDiffWithEqCheck builds the abs_diff function as produced by the Nanz
// frontend: a CmpEq early-exit followed by CmpLt for the sign check.
//
//	fun abs_diff(a: u8, b: u8) → u8 {
//	    if a == b { return 0 }       // ← this branch is redundant
//	    if a < b  { return b - a }
//	    return a - b
//	}
//
// The CmpEq branch is redundant because when a==b, both (b-a) and (a-b) equal 0.
func buildAbsDiffWithEqCheck(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("abs_diff_eq")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	// entry: if a == b → zero_block, else → diff_block
	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)
	eq := b.Cmp(mir2.CmpEq, a, bv, mir2.ClassFlag, false)
	b.BrIf(eq, "zero_block", nil, "diff_block", nil)

	// zero_block: return 0  (when a == b)
	b.SwitchToNewBlock("zero_block")
	zero := b.Const(0, mir2.TyU8, mir2.ClassAcc)
	b.Ret(zero)

	// diff_block: if a < b → b_minus_a, else → a_minus_b
	b.SwitchToNewBlock("diff_block")
	lt := b.Cmp(mir2.CmpLt, a, bv, mir2.ClassFlag, false)
	b.BrIf(lt, "b_minus_a", nil, "a_minus_b", nil)

	// b_minus_a: return b - a
	b.SwitchToNewBlock("b_minus_a")
	r1 := b.Sub(bv, a, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r1)

	// a_minus_b: return a - b
	b.SwitchToNewBlock("a_minus_b")
	r2 := b.Sub(a, bv, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r2)

	return f
}

// TestBranchEquiv_AbsDiff verifies that BranchEquiv eliminates the redundant
// CmpEq branch in abs_diff, and that the optimized function still computes the
// correct absolute difference.
func TestBranchEquiv_AbsDiff(t *testing.T) {
	m := &mir2.Module{Name: "branchequiv"}
	f := buildAbsDiffWithEqCheck(m)

	// Count TermBrIf terminators before.
	before := countBrIf(f)
	if before != 2 {
		t.Fatalf("want 2 TermBrIf before optimization, got %d", before)
	}

	changed := mir2.BranchEquiv(m, f)
	if !changed {
		t.Fatal("BranchEquiv reported no change; expected CmpEq branch to be eliminated")
	}

	after := countBrIf(f)
	if after != 1 {
		t.Fatalf("want 1 TermBrIf after optimization, got %d", after)
	}

	// Verify correctness: optimized function must still compute |a-b|.
	vm := mir2.NewVM(m)
	cases := [][3]int64{
		{0, 0, 0},
		{5, 5, 0},
		{10, 3, 7},
		{3, 10, 7},
		{0, 255, 255},
		{255, 0, 255},
		{100, 200, 100},
		{200, 100, 100},
	}
	for _, tc := range cases {
		got, err := vm.CallFunc(f, []mir2.Value{{I: tc[0]}, {I: tc[1]}})
		if err != nil {
			t.Errorf("abs_diff(%d,%d): VM error: %v", tc[0], tc[1], err)
			continue
		}
		if got[0].I != tc[2] {
			t.Errorf("abs_diff(%d,%d) = %d, want %d", tc[0], tc[1], got[0].I, tc[2])
		} else {
			t.Logf("abs_diff(%d,%d) = %d ✓", tc[0], tc[1], got[0].I)
		}
	}
}

// TestBranchEquiv_NotRedundant verifies that BranchEquiv does NOT eliminate a
// branch that is genuinely needed.
//
//	fun sign(a: u8) → u8 {
//	    if a == 0 { return 99 }   // ← distinct result, NOT redundant
//	    return 1
//	}
func TestBranchEquiv_NotRedundant(t *testing.T) {
	m := &mir2.Module{Name: "branchequiv_neg"}
	f := m.AddFunc("sign")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)
	eq := b.Cmp(mir2.CmpEq, a, bv, mir2.ClassFlag, false)
	b.BrIf(eq, "eq_block", nil, "ne_block", nil)

	// eq_block: return 99 (sentinel — different from ne_block's result when a==b>0)
	b.SwitchToNewBlock("eq_block")
	r99 := b.Const(99, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r99)

	// ne_block: return a + b (when a != b, but when a==b this differs from 99)
	b.SwitchToNewBlock("ne_block")
	sum := b.Add(a, bv, mir2.TyU8, mir2.ClassAcc)
	b.Ret(sum)

	before := countBrIf(f)
	changed := mir2.BranchEquiv(m, f)
	after := countBrIf(f)

	if changed || after != before {
		t.Errorf("BranchEquiv incorrectly eliminated a non-redundant branch")
	} else {
		t.Log("correctly preserved non-redundant branch ✓")
	}
}

// TestBranchEquiv_Idempotent verifies that running BranchEquiv twice produces
// the same result as running it once.
func TestBranchEquiv_Idempotent(t *testing.T) {
	m := &mir2.Module{Name: "branchequiv_idem"}
	f := buildAbsDiffWithEqCheck(m)

	first := mir2.BranchEquiv(m, f)
	second := mir2.BranchEquiv(m, f)

	if !first {
		t.Fatal("first pass: expected change")
	}
	if second {
		t.Fatal("second pass: expected no change (already optimized)")
	}
}

// countBrIf returns the number of *TermBrIf terminators in f.
func countBrIf(f *mir2.Func) int {
	n := 0
	for _, blk := range f.Blocks {
		if _, ok := blk.Term.(*mir2.TermBrIf); ok {
			n++
		}
	}
	return n
}
