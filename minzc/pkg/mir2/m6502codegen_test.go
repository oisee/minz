package mir2_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// TestM6502CodegenAdd tests fun add(a: u8, b: u8) -> u8 { return a + b }
func TestM6502CodegenAdd(t *testing.T) {
	m := &mir2.Module{Name: "add_test"}
	f := m.AddFunc("add")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	b := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	sum := bld.Add(a, b, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(sum)

	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.M6502CostTable{})

	asm := mir2.M6502Codegen(m, ar)
	t.Log("\n" + asm)

	checks := []string{"add:", "CLC", "ADC", "RTS"}
	for _, needle := range checks {
		if !strings.Contains(asm, needle) {
			t.Errorf("assembly missing %q", needle)
		}
	}
}

// TestM6502CodegenConst tests loading constant 42.
func TestM6502CodegenConst(t *testing.T) {
	m := &mir2.Module{Name: "const_test"}
	f := m.AddFunc("answer")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	c := bld.Const(42, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(c)

	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.M6502CostTable{})

	asm := mir2.M6502Codegen(m, ar)
	t.Log("\n" + asm)

	if !strings.Contains(asm, "LDA #$2A") {
		t.Error("expected LDA #$2A (42 decimal)")
	}
	if !strings.Contains(asm, "RTS") {
		t.Error("expected RTS")
	}
}

// TestM6502CodegenSub tests subtraction.
func TestM6502CodegenSub(t *testing.T) {
	m := &mir2.Module{Name: "sub_test"}
	f := m.AddFunc("sub")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}

	bld := mir2.NewBuilder(f)
	bld.SwitchToNewBlock("entry")
	a := bld.Param("a", mir2.TyU8, mir2.ClassAcc)
	b := bld.Param("b", mir2.TyU8, mir2.ClassGeneral)
	diff := bld.Sub(a, b, mir2.TyU8, mir2.ClassAcc)
	bld.Ret(diff)

	if err := mir2.Verify(m); err != nil {
		t.Fatalf("verify: %v", err)
	}

	lr := mir2.ComputeLiveness(f)
	ar := mir2.Allocate(f, lr, mir2.M6502CostTable{})

	asm := mir2.M6502Codegen(m, ar)
	t.Log("\n" + asm)

	checks := []string{"SEC", "SBC", "RTS"}
	for _, needle := range checks {
		if !strings.Contains(asm, needle) {
			t.Errorf("assembly missing %q", needle)
		}
	}
}
