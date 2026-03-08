package mir2_test

import (
	"slices"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// regSet converts a RegSet to a sorted []int for easy comparison in tests.
func regs(s *mir2.RegSet) []int {
	out := []int{}
	for _, r := range s.Slice() {
		out = append(out, int(r))
	}
	slices.Sort(out)
	return out
}

func TestRegSetBasic(t *testing.T) {
	s := mir2.NewRegSetForTest()

	s.Add(1)
	s.Add(5)
	s.Add(64) // crosses word boundary
	s.Add(65)

	for _, r := range []mir2.Reg{1, 5, 64, 65} {
		if !s.Has(r) {
			t.Errorf("Has(%d) = false, want true", r)
		}
	}
	for _, r := range []mir2.Reg{2, 3, 63, 66} {
		if s.Has(r) {
			t.Errorf("Has(%d) = true, want false", r)
		}
	}

	s.Remove(5)
	if s.Has(5) {
		t.Error("Has(5) after Remove = true, want false")
	}
	if s.Len() != 3 {
		t.Errorf("Len() = %d, want 3", s.Len())
	}
}

func TestRegSetUnion(t *testing.T) {
	a := mir2.NewRegSetForTest()
	a.Add(1)
	a.Add(3)

	b := mir2.NewRegSetForTest()
	b.Add(3)
	b.Add(5)

	a.UnionWith(b)
	got := regs(a)
	want := []int{1, 3, 5}
	if !slices.Equal(got, want) {
		t.Errorf("Union = %v, want %v", got, want)
	}
}

func TestRegSetSubtract(t *testing.T) {
	a := mir2.NewRegSetForTest()
	a.Add(1)
	a.Add(2)
	a.Add(3)

	b := mir2.NewRegSetForTest()
	b.Add(2)

	a.SubtractWith(b)
	got := regs(a)
	want := []int{1, 3}
	if !slices.Equal(got, want) {
		t.Errorf("Subtract = %v, want %v", got, want)
	}
}

func TestLivenessSimple(t *testing.T) {
	// fun @id(%r1: u8) -> u8
	//   block @entry:
	//     ret %r1
	//
	// %r1 is a function param → it's in def[entry] (defined at block entry).
	// LiveIn[entry]  = {}   (param is defined here, not upward-exposed)
	// LiveOut[entry] = {}   (ret has no successors)
	m := &mir2.Module{Name: "id"}
	f := m.AddFunc("id")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	bld.Param("n", mir2.TyU8, mir2.ClassAcc)
	bld.Ret(f.Contract.Params[0].Reg)

	lr := mir2.ComputeLiveness(f)

	if lr.LiveIn[0].Len() != 0 {
		t.Errorf("LiveIn[entry] = %v, want empty (param is defined at entry, not upward-exposed)",
			regs(lr.LiveIn[0]))
	}
	if lr.LiveOut[0].Len() != 0 {
		t.Errorf("LiveOut[entry] = %v, want empty (ret has no successors)", regs(lr.LiveOut[0]))
	}
}

func TestLivenessLinear(t *testing.T) {
	// fun @double(%r1: u8) -> u8
	//   block @entry:
	//     %r2 = add %r1, %r1 : u8
	//     ret %r2
	//
	// %r1 is a param → in def[entry]; %r2 is defined in entry.
	// LiveIn[entry]  = {}   (all defs are local)
	// LiveOut[entry] = {}   (ret, no successors)
	m := &mir2.Module{Name: "double"}
	f := m.AddFunc("double")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	x := bld.Param("x", mir2.TyU8, mir2.ClassAcc)
	sum := bld.Add(x, x, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(sum)

	lr := mir2.ComputeLiveness(f)

	if lr.LiveIn[0].Len() != 0 {
		t.Errorf("LiveIn[entry] = %v, want empty", regs(lr.LiveIn[0]))
	}
	if lr.LiveOut[0].Has(sum) {
		t.Errorf("LiveOut[entry] should NOT contain %% r%d (defined and consumed in entry)", sum)
	}
}

func TestLivenessFibonacci(t *testing.T) {
	// Build fibonacci and check key liveness invariants.
	//
	// MIR2 blocks (from vm_test.go buildFib):
	//   entry        → base | loop_init
	//   base         → (ret)
	//   loop_init    → loop_head
	//   loop_head    → loop_body | exit
	//   loop_body    → loop_head
	//   exit         → (ret)
	//
	// %r1 = param n  (used in entry for cmp, in loop_head for cmp)
	// Key invariants:
	//  - %r1 must be live in @loop_head (used for the cmp there)
	//  - block params of loop_head (%r8,%r9,%r10) must be in LiveIn[loop_head]
	//    only if they're defined by the block params (they ARE defined there — not live-in)
	//  - %r1 must be live-out from @entry (it's needed in @loop_head which is reachable)

	m := &mir2.Module{Name: "fib_live"}
	f := buildFib(m)

	lr := mir2.ComputeLiveness(f)

	// Find block indices by label.
	blockIdx := map[string]int{}
	for i, b := range f.Blocks {
		blockIdx[b.Label] = i
	}

	// %r1 is the function param (n).  It should be live-in to loop_head
	// because loop_head uses it in a cmp.
	n := f.Contract.Params[0].Reg // %r1

	headIdx := blockIdx["loop_head"]
	if !lr.LiveIn[headIdx].Has(n) {
		t.Errorf("LiveIn[loop_head] should contain param n (%% r%d); got %v",
			n, regs(lr.LiveIn[headIdx]))
	}

	// loop_head has block params %r8, %r9, %r10.
	// They're DEFINED at loop_head's entry → should NOT be in LiveIn[loop_head].
	headBlock := f.Blocks[headIdx]
	for _, p := range headBlock.Params {
		if lr.LiveIn[headIdx].Has(p.Dst) {
			t.Errorf("LiveIn[loop_head] should NOT contain block param %% r%d (it's defined there)",
				p.Dst)
		}
	}

	// LiveOut[exit] should be empty (exit just returns).
	exitIdx := blockIdx["exit"]
	if lr.LiveOut[exitIdx].Len() != 0 {
		t.Errorf("LiveOut[exit] should be empty; got %v", regs(lr.LiveOut[exitIdx]))
	}

	// entry's live-out must include %r1 (since loop_head uses it and is reachable
	// through loop_init from entry).
	entryIdx := blockIdx["entry"]
	if !lr.LiveOut[entryIdx].Has(n) {
		t.Errorf("LiveOut[entry] should contain param n (%% r%d); got %v",
			n, regs(lr.LiveOut[entryIdx]))
	}
}

func TestLivenessBlockArgs(t *testing.T) {
	// In block-arg SSA, data flows across edges via block parameters:
	//
	//   fun @example(%r1: u8) -> u8
	//     block @entry:
	//       jmp @middle()
	//     block @middle:
	//       // %r1 is used here directly (function param, not block param)
	//       %r3 = add %r1, %r1 : u8
	//       ret %r3
	//
	// %r1 is a function param → def[entry] = {%r1}, def[middle] = {}
	// middle uses %r1 directly (cross-block param reference):
	//   use[middle] = {%r1}   (used in add, not defined in middle)
	// LiveIn[middle]  = {%r1}
	// LiveOut[entry]  = LiveIn[middle] = {%r1}
	// LiveIn[entry]   = use[entry] ∪ (LiveOut[entry] \ def[entry])
	//                 = {} ∪ ({%r1} \ {%r1}) = {}

	m := &mir2.Module{Name: "cross"}
	f := m.AddFunc("example")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	bld := mir2.NewBuilder(f)

	bld.SwitchToNewBlock("entry")
	r1 := bld.Param("x", mir2.TyU8, mir2.ClassAcc)
	bld.Jmp("middle")

	bld.SwitchToNewBlock("middle")
	r3 := bld.Add(r1, r1, mir2.TyU8, mir2.ClassAcc) // uses %r1 directly — cross-block
	bld.Ret(r3)

	lr := mir2.ComputeLiveness(f)

	entryIn := lr.LiveIn[0]   // entry
	entryOut := lr.LiveOut[0] // entry
	middleIn := lr.LiveIn[1]  // middle

	// %r1 used in middle but defined in entry → must be live-in to middle.
	if !middleIn.Has(r1) {
		t.Errorf("LiveIn[middle] should contain param %% r%d (used across blocks); got %v",
			r1, regs(middleIn))
	}
	// LiveOut[entry] must propagate live-in of its successor.
	if !entryOut.Has(r1) {
		t.Errorf("LiveOut[entry] should contain %% r%d (needed by successor middle); got %v",
			r1, regs(entryOut))
	}
	// LiveIn[entry] is empty: %r1 is defined there (param), so it's subtracted out.
	if entryIn.Len() != 0 {
		t.Errorf("LiveIn[entry] = %v, want empty (param is defined at entry)", regs(entryIn))
	}
}
