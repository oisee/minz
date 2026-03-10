package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// buildAbsDiffForCondRet builds abs_diff after BranchEquiv has already fired:
// the CmpEq guard is gone; only the CmpLt branch remains.
//
//	block @diff:
//	    lt = CmpLt(a, b)
//	    BrIf(lt, @b_minus_a(), @a_minus_b())
//
//	block @b_minus_a:  r1 = sub(b, a);  ret r1
//	block @a_minus_b:  r2 = sub(a, b);  ret r2
//
// @a_minus_b is the else-branch: trivial (1 pure inst + ret, no params).
// CondRetSink should hoist sub(a,b) into @diff and replace BrIf with TermCondRet.
func buildAbsDiffForCondRet(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("abs_diff_cr")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("diff")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)
	lt := b.Cmp(mir2.CmpLt, a, bv, mir2.ClassFlag, false)
	b.BrIf(lt, "b_minus_a", nil, "a_minus_b", nil)

	b.SwitchToNewBlock("b_minus_a")
	r1 := b.Sub(bv, a, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r1)

	b.SwitchToNewBlock("a_minus_b")
	r2 := b.Sub(a, bv, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r2)

	return f
}

// TestCondRetSink_AbsDiff verifies that CondRetSink converts the else-branch
// return into a TermCondRet and that the optimised function is still correct.
func TestCondRetSink_AbsDiff(t *testing.T) {
	m := &mir2.Module{Name: "condret"}
	f := buildAbsDiffForCondRet(m)

	// Before: 1 BrIf, 0 TermCondRet.
	if n := countBrIf(f); n != 1 {
		t.Fatalf("want 1 BrIf before CondRetSink, got %d", n)
	}
	if n := countTermCondRet(f); n != 0 {
		t.Fatalf("want 0 TermCondRet before CondRetSink, got %d", n)
	}

	changed := mir2.CondRetSink(f)
	if !changed {
		t.Fatal("CondRetSink reported no change")
	}
	mir2.EliminateDeadBlocks(f)

	// After: 0 BrIf, 1 TermCondRet.
	if n := countBrIf(f); n != 0 {
		t.Fatalf("want 0 BrIf after CondRetSink, got %d", n)
	}
	if n := countTermCondRet(f); n != 1 {
		t.Fatalf("want 1 TermCondRet after CondRetSink, got %d", n)
	}

	// Correctness: still computes |a - b|.
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

// TestCondRetSink_NotApplicable verifies that CondRetSink does NOT fire when
// the else-branch carries block arguments (non-trivial join point).
func TestCondRetSink_NotApplicable(t *testing.T) {
	m := &mir2.Module{Name: "condret_no"}
	f := m.AddFunc("fn")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	bv := b.Param("b", mir2.TyU8, mir2.ClassCounter)
	lt := b.Cmp(mir2.CmpLt, a, bv, mir2.ClassFlag, false)

	// else-branch carries an argument → CondRetSink must not fire
	// (len(ElseArgs) != 0 check).
	b.BrIf(lt, "then_blk", nil, "els_blk", []mir2.Reg{a})

	b.SwitchToNewBlock("then_blk")
	b.Ret(a)

	// @els_blk has a block param added via BlockParam to accept the arg.
	elsBlk := b.SwitchToNewBlock("els_blk")
	r := b.BlockParam(elsBlk, mir2.TyU8, mir2.ClassAcc)
	b.Ret(r)

	changed := mir2.CondRetSink(f)
	if changed {
		t.Error("CondRetSink incorrectly fired on a non-trivial else-branch")
	}
}

// TestCondRetSink_Idempotent verifies that a second CondRetSink pass is a no-op.
func TestCondRetSink_Idempotent(t *testing.T) {
	m := &mir2.Module{Name: "condret_idem"}
	f := buildAbsDiffForCondRet(m)

	first := mir2.CondRetSink(f)
	mir2.EliminateDeadBlocks(f)
	second := mir2.CondRetSink(f)

	if !first {
		t.Fatal("first pass: expected change")
	}
	if second {
		t.Fatal("second pass: expected no change (already optimized)")
	}
}

// TestCondRetSink_Z80Assembly verifies that CondRetSink + codegen produces
// a RET CC instruction in the Z80 output for abs_diff.
func TestCondRetSink_Z80Assembly(t *testing.T) {
	m := &mir2.Module{Name: "condret_z80"}
	f := buildAbsDiffForCondRet(m)

	// Apply optimisations.
	mir2.CondRetSink(f)
	mir2.EliminateDeadBlocks(f)

	// Verify + allocate.
	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}
	ct := mir2.Z80CostTable{}
	lr := mir2.ComputeLiveness(f)
	ar := mir2.PBQPAllocate(f, lr, ct)
	combined := &mir2.AllocResult{Locs: make(map[mir2.Reg]mir2.PhysLoc)}
	for r, loc := range ar.Locs {
		combined.Locs[r] = loc
	}

	asm := mir2.Z80Codegen(m, combined)
	t.Logf("Z80 output:\n%s", asm)

	// Must contain a conditional RET (RET C or RET NC or similar).
	if !strings.Contains(asm, "RET C") && !strings.Contains(asm, "RET NC") &&
		!strings.Contains(asm, "RET Z") && !strings.Contains(asm, "RET NZ") {
		t.Errorf("expected conditional RET in output, got:\n%s", asm)
	}
}

func countTermCondRet(f *mir2.Func) int {
	n := 0
	for _, blk := range f.Blocks {
		if _, ok := blk.Term.(*mir2.TermCondRet); ok {
			n++
		}
	}
	return n
}
