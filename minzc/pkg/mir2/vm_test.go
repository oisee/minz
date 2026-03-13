package mir2_test

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// buildFib builds the fibonacci function in MIR2 using block arguments.
//
// MinZ source:
//
//	fun fibonacci(n: u8) -> u16 {
//	    if n <= 1 { return n as u16 }
//	    var a: u16 = 0
//	    var b: u16 = 1
//	    var i: u8  = 2
//	    while i <= n {
//	        let temp = a + b
//	        a = b
//	        b = temp
//	        i++
//	    }
//	    return b
//	}
//
// MIR2 with block arguments:
//
//	fun @fibonacci(%r1: u8 [acc]) -> u16 [pointer]
//	  block @entry:
//	    %r2 = const 1 : u8
//	    %r3 = cmp.le %r1, %r2 : bool [flag]
//	    br_if %r3, @base(), @loop_init()
//	  block @base:
//	    %r4 = ext %r1 : u8 -> u16
//	    ret %r4
//	  block @loop_init:
//	    %r5 = const 0 : u16   ; a
//	    %r6 = const 1 : u16   ; b
//	    %r7 = const 2 : u8    ; i
//	    jmp @loop_head(%r5, %r6, %r7)
//	  block @loop_head(%r8: u16 [pointer], %r9: u16 [index], %r10: u8 [counter]):
//	    %r11 = cmp.le %r10, %r1 : bool [flag]
//	    br_if %r11, @loop_body(%r8, %r9, %r10), @exit(%r9)
//	  block @loop_body(%r12: u16, %r13: u16, %r14: u8):
//	    %r15 = add %r12, %r13 : u16   ; temp = a + b
//	    %r16 = add %r14, const(1) : u8 ; i++
//	    jmp @loop_head(%r13, %r15, %r16)  ; a=b, b=temp
//	  block @exit(%r17: u16):
//	    ret %r17
func buildFib(m *mir2.Module) *mir2.Func {
	f := m.AddFunc("fibonacci")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)

	// ── entry ──
	b.SwitchToNewBlock("entry")
	n := b.Param("n", mir2.TyU8, mir2.ClassAcc)
	one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	cmp := b.Cmp(mir2.CmpLe, n, one, mir2.ClassFlag, false)
	b.BrIf(cmp, "base", nil, "loop_init", nil)

	// ── base (n <= 1: return n as u16) ──
	b.SwitchToNewBlock("base")
	n16 := b.Ext(n, mir2.TyU8, mir2.TyU16, mir2.ClassAcc)
	b.Ret(n16)

	// ── loop_init ──
	b.SwitchToNewBlock("loop_init")
	a0 := b.Const(0, mir2.TyU16, mir2.ClassPointer)
	b0 := b.Const(1, mir2.TyU16, mir2.ClassIndex)
	i0 := b.Const(2, mir2.TyU8, mir2.ClassCounter)
	b.Jmp("loop_head", a0, b0, i0)

	// ── loop_head (block args: a, b, i) ──
	head := b.SwitchToNewBlock("loop_head")
	aP := b.BlockParam(head, mir2.TyU16, mir2.ClassPointer) // a
	bP := b.BlockParam(head, mir2.TyU16, mir2.ClassIndex)   // b (loop var)
	iP := b.BlockParam(head, mir2.TyU8, mir2.ClassCounter)  // i
	cond := b.Cmp(mir2.CmpLe, iP, n, mir2.ClassFlag, false)
	b.BrIf(cond,
		"loop_body", []mir2.Reg{aP, bP, iP},
		"exit", []mir2.Reg{bP})

	// ── loop_body ──
	body := b.SwitchToNewBlock("loop_body")
	aB := b.BlockParam(body, mir2.TyU16, mir2.ClassPointer)
	bB := b.BlockParam(body, mir2.TyU16, mir2.ClassIndex)
	iB := b.BlockParam(body, mir2.TyU8, mir2.ClassCounter)
	temp := b.Add(aB, bB, mir2.TyU16, mir2.ClassPointer) // temp = a + b
	one8 := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	iNext := b.Add(iB, one8, mir2.TyU8, mir2.ClassCounter)  // i++
	b.Jmp("loop_head", bB, temp, iNext)                      // a=b, b=temp

	// ── exit ──
	exit := b.SwitchToNewBlock("exit")
	result := b.BlockParam(exit, mir2.TyU16, mir2.ClassPointer)
	b.Ret(result)

	return f
}

func TestVMFibonacci(t *testing.T) {
	expected := []struct {
		n   int64
		fib int64
	}{
		{0, 0}, {1, 1}, {2, 1}, {3, 2}, {4, 3},
		{5, 5}, {6, 8}, {7, 13}, {8, 21}, {10, 55},
	}

	m := &mir2.Module{Name: "test"}
	buildFib(m)

	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}

	vm := mir2.NewVM(m)

	for _, tc := range expected {
		got, err := vm.Call("fibonacci", []mir2.Value{{I: tc.n}})
		if err != nil {
			t.Errorf("fib(%d): vm error: %v", tc.n, err)
			continue
		}
		if len(got) != 1 || got[0].I != tc.fib {
			t.Errorf("fib(%d) = %v, want %d", tc.n, got, tc.fib)
		}
	}
}

func TestVMSMCCounter(t *testing.T) {
	// Build an SMC counter: function that returns its call count.
	// patch_slot holds the count across calls.
	//
	//   fun @counter() -> u8
	//     block @entry:
	//       %slot = patch_slot 0 : u8
	//       %val  = load_patched %slot : u8
	//       %one  = const 1 : u8
	//       %next = add %val, %one : u8
	//       patch %slot, %next
	//       ret %val
	//
	// NOTE: patch_slot in the VM allocates a heap slot each call, so
	// this tests single-call SMC semantics. Cross-call persistence
	// requires a module-level global (shown here via global var).

	m := &mir2.Module{Name: "smc_test"}
	f := m.AddFunc("counter")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	slot := bld.PatchSlot(0, mir2.TyU8, mir2.ClassAcc)
	val := bld.LoadPatched(slot, mir2.TyU8, mir2.ClassAcc)
	one := bld.Const(1, mir2.TyU8, mir2.ClassGeneral)
	next := bld.Add(val, one, mir2.TyU8, mir2.ClassAcc)
	bld.Patch(slot, next, mir2.TyU8)
	bld.Ret(val)

	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}

	vm := mir2.NewVM(m)
	// Single call: slot starts at 0, returns 0, patches to 1.
	got, err := vm.Call("counter", nil)
	if err != nil {
		t.Fatalf("counter: %v", err)
	}
	if len(got) != 1 || got[0].I != 0 {
		t.Errorf("counter() = %v, want 0", got)
	}
}

func TestVMVerifyBlockArgArity(t *testing.T) {
	// Build a function with mismatched block arg count — Verify must catch it.
	m := &mir2.Module{Name: "bad"}
	f := m.AddFunc("bad")

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	bld.Jmp("target") // supplies 0 args

	target := bld.SwitchToNewBlock("target")
	bld.BlockParam(target, mir2.TyU8, mir2.ClassGeneral) // expects 1 param
	bld.Ret()

	err := mir2.Verify(m)
	if err == nil {
		t.Fatal("expected verify error for arity mismatch, got nil")
	}
}

func TestVMDump(t *testing.T) {
	m := &mir2.Module{Name: "fib_dump"}
	buildFib(m)
	out := m.Dump()
	if len(out) == 0 {
		t.Fatal("Dump returned empty string")
	}
	// Spot-check key substrings.
	checks := []string{
		"fun @fibonacci",
		"block @entry",
		"block @loop_head(",
		"br_if",
		"patch_slot",
	}
	for _, needle := range checks {
		// patch_slot won't appear in fibonacci; skip it
		if needle == "patch_slot" {
			continue
		}
		found := false
		for i := 0; i < len(out)-len(needle)+1; i++ {
			if out[i:i+len(needle)] == needle {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Dump missing %q", needle)
		}
	}
}

func TestVMSignedComparison(t *testing.T) {
	// max_i8(a, b) → a >= b ? a : b   (signed comparison)
	// With i8: -5 is stored as 251. Signed comparison must recover the sign.
	//
	//   fun @max_i8(a: i8, b: i8) -> i8
	//     block @entry:
	//       %cmp = cmp ge %a, %b : bool   [SrcTy=i8]
	//       br_if %cmp, @ret_a(), @ret_b()
	//     block @ret_a:
	//       ret %a
	//     block @ret_b:
	//       ret %b
	m := &mir2.Module{Name: "signed_cmp"}
	f := m.AddFunc("max_i8")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyI8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)

	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyI8, mir2.ClassAcc)
	bParam := b.Param("b", mir2.TyI8, mir2.ClassGeneral)

	cmp := b.CmpWithSrcTy(mir2.CmpGe, a, bParam, mir2.ClassFlag, false, mir2.TyI8)
	b.BrIf(cmp, "ret_a", nil, "ret_b", nil)

	retA := b.SwitchToNewBlock("ret_a")
	_ = retA
	b.Ret(a)

	retB := b.SwitchToNewBlock("ret_b")
	_ = retB
	b.Ret(bParam)

	vm := mir2.NewVM(m)

	tests := []struct {
		a, b, want int64
	}{
		{5, 3, 5},       // 5 >= 3 → true → return 5
		{3, 5, 5},       // 3 >= 5 → false → return 5
		{251, 5, 5},     // -5 (251) >= 5 → false (signed) → return 5
		{5, 251, 5},     // 5 >= -5 (251) → true (signed) → return 5
		{251, 253, 253}, // -5 >= -3 → false → return -3 (253)
		{253, 251, 253}, // -3 >= -5 → true → return -3 (253)
		{0, 0, 0},       // equal
	}

	for _, tc := range tests {
		got, err := vm.Call("max_i8", []mir2.Value{{I: tc.a}, {I: tc.b}})
		if err != nil {
			t.Errorf("max_i8(%d, %d): %v", tc.a, tc.b, err)
			continue
		}
		if len(got) != 1 || got[0].I != tc.want {
			t.Errorf("max_i8(%d, %d) = %d, want %d", tc.a, tc.b, got[0].I, tc.want)
		}
	}
}
