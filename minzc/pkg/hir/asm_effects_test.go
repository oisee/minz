package hir

import (
	"testing"

	"github.com/minz/minzc/pkg/mir2"
)

// classStackOnly is the clobber set for a template that moves SP and nothing
// else: the frame slots go, no allocatable register does.
var classStackOnly = func() mir2.ClassSet {
	var s mir2.ClassSet
	s.Add(mir2.ClassStack)
	return s
}()

func TestAsmEffectsImplicitWritesAndAliases(t *testing.T) {
	for _, code := range []string{"XOR B", "NEG", "RRA", "EX AF, AF'"} {
		effects := z80AutoClobbers(code)
		if !effects.Has(mir2.ClassAcc) || !effects.Has(mir2.ClassFlag) {
			t.Fatalf("implicit A/F missed: %s %v", code, effects)
		}
	}
	for _, code := range []string{"INC H", "LD L, 7", "POP HL"} {
		effects := z80AutoClobbers(code)
		if !effects.Has(mir2.ClassPointer) || !effects.Has(mir2.ClassPair) || !effects.Has(mir2.ClassDWord) {
			t.Fatalf("pair alias missed: %s %v", code, effects)
		}
	}
	index := z80AutoClobbers("INC IXH")
	if !index.Has(mir2.ClassIX) || !index.Has(mir2.ClassIXY8) || !index.Has(mir2.ClassGeneral) || !index.Has(mir2.ClassPair) {
		t.Fatal(index)
	}
	exchange := z80AutoClobbers("EXX")
	if !exchange.Has(mir2.ClassShadow) || !exchange.Has(mir2.ClassPointer) || exchange.Has(mir2.ClassFlag) {
		t.Fatal(exchange)
	}
	if x := z80AutoClobbers("EX AF, AF'"); !x.Has(mir2.ClassAccShadow) {
		t.Fatal(x)
	}
}
func TestAsmEffectsKnownNonWritesAndOpaqueFallback(t *testing.T) {
	for _, code := range []string{"LD (HL), A", "DI / EI"} {
		if !z80AutoClobbers(code).Empty() {
			t.Fatalf("non-allocatable write treated as register clobber: %s", code)
		}
	}
	// Moving SP invalidates frame-relative slots without touching any register.
	if sp := z80AutoClobbers("INC SP"); sp != classStackOnly {
		t.Fatalf("INC SP should report only the frame slots: %v", sp)
	}
	djnz := z80AutoClobbers("DJNZ loop")
	if !djnz.Has(mir2.ClassCounter) || djnz.Has(mir2.ClassFlag) || djnz == mir2.AllClasses {
		t.Fatal(djnz)
	}
	for _, code := range []string{"CALL helper", "DB 195 / DW next", "LD DE, HL"} {
		if z80AutoClobbers(code) != mir2.AllClasses {
			t.Fatalf("opaque code accepted: %s", code)
		}
	}
}
func TestAsmEffectsSurviveHIRLowering(t *testing.T) {
	h := &Module{Name: "effects", Funcs: []*Func{{Name: "test", RetTy: mir2.TyVoid, Body: &Block{Body: []Stmt{&AsmStmt{Target: "z80", Code: "HALT / DI / LD SP, 0x5800", ClobberAuto: true}}}}}}
	module := LowerModule(h)
	inst := module.FuncByName("test").Entry().Insts[0]
	if inst.Asm == nil || len(inst.Asm.Effects) != 3 || !inst.Asm.Effects[2].ReplacesSP {
		t.Fatal(inst)
	}
}
