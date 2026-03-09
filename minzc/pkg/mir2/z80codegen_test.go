package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

func TestZ80CodegenFibonacci(t *testing.T) {
	m := &mir2.Module{Name: "fib"}
	buildFib(m)

	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}

	var asm string
	for _, f := range m.Funcs {
		lr := mir2.ComputeLiveness(f)
		ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
		asm = mir2.Z80Codegen(m, ar)
	}

	t.Log("\n" + asm)

	// Structural checks.
	checks := []string{
		"fibonacci:",       // function entry label
		".fibonacci_base:", // base block label
		"CP ",              // comparison
		"JRS ",             // branch (JRS = JR-if-Short, auto-promotes to JP if needed)
		"ADD HL",           // 16-bit add (a+b in loop_body)
		"RET",              // return
	}
	for _, needle := range checks {
		if !strings.Contains(asm, needle) {
			t.Errorf("assembly missing %q", needle)
		}
	}
}

func TestZ80CodegenSMCCounter(t *testing.T) {
	m := &mir2.Module{Name: "smc"}
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

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)

	t.Log("\n" + asm)

	if !strings.Contains(asm, "patch_slot") {
		t.Error("SMC counter assembly should reference patch_slot")
	}
	if !strings.Contains(asm, "RET") {
		t.Error("SMC counter assembly must have RET")
	}
}

func TestZ80CodegenExt(t *testing.T) {
	// fun @zext(%r1: u8 [acc]) -> u16 [pointer]
	//   %r2 = ext %r1 : u8 → u16 [pointer]
	//   ret %r2
	// Expect: LD L, A / LD H, 0
	m := &mir2.Module{Name: "zext"}
	f := m.AddFunc("zext")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPointer}}
	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	r1 := bld.Param("n", mir2.TyU8, mir2.ClassAcc)
	r2 := bld.Ext(r1, mir2.TyU8, mir2.TyU16, mir2.ClassPointer)
	bld.Ret(r2)

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.Z80CostTable{})
	asm := mir2.Z80Codegen(m, ar)

	t.Log("\n" + asm)

	// %r1 → A, %r2 → HL: should emit LD L, A / LD H, 0
	if !strings.Contains(asm, "LD L, A") {
		t.Errorf("zero-extend u8→u16: expected 'LD L, A'; got:\n%s", asm)
	}
	if !strings.Contains(asm, "LD H, 0") {
		t.Errorf("zero-extend u8→u16: expected 'LD H, 0'; got:\n%s", asm)
	}
}
