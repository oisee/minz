package mir2

import (
	"strings"
	"testing"
)

func namedSMCTestModule() *Module {
	m := &Module{Name: "smc"}
	for _, name := range []string{"read", "dead"} {
		f := m.AddFunc(name)
		f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		slot := b.PatchSlotNamed(name+"$dst", 3, TyU16, ClassPointer)
		b.Ret(slot)
		s := m.AddFunc(name + "_set_dst")
		s.SMCSetterFor = name + "$dst"
		s.Attrs.FixedIndirectABI = true
		b = NewBuilder(s)
		b.SwitchToNewBlock("entry")
		value := b.Param("value", TyU16, ClassPointer)
		b.PatchNamed(s.SMCSetterFor, value, TyU16)
		b.Ret()
	}
	return m
}

func TestNamedSMCPersistentVMAndPruning(t *testing.T) {
	for _, root := range []string{"read", "read_set_dst"} {
		m := namedSMCTestModule()
		if e := Verify(m); e != nil {
			t.Fatal(e)
		}
		PruneUnreachableFuncs(m, map[string]bool{root: true})
		if len(m.Funcs) != 2 || m.FuncByName("read") == nil || m.FuncByName("read_set_dst") == nil {
			t.Fatalf("wrong SMC closure: %v", m.Funcs)
		}
		if e := ValidateZ80Symbols(m); e != nil {
			t.Fatal(e)
		}
		vm := NewVM(m)
		for _, value := range []int64{0x12345, 37, 65535, 0} {
			if _, e := vm.Call("read_set_dst", []Value{{I: value}}); e != nil {
				t.Fatal(e)
			}
			for repeat := 0; repeat < 3; repeat++ {
				got, e := vm.Call("read", nil)
				if e != nil || len(got) != 1 || got[0].I != value&65535 {
					t.Fatalf("named state %v %v want%d", got, e, value&65535)
				}
			}
		}
	}
}

func TestNamedSMCRejectsForgedSetter(t *testing.T) {
	for _, mutate := range []func(*Func){
		func(f *Func) { f.Attrs.FixedIndirectABI = false },
		func(f *Func) { f.Contract.Params[0].Class = ClassGeneral },
		func(f *Func) { f.SMCSetterFor = "missing$dst" },
		func(f *Func) { f.Blocks[0].Insts[0].Sym = "dead$dst" },
		func(f *Func) { f.Blocks[0].Insts[0].Src[0] = f.Contract.Params[0].Reg },
	} {
		m := namedSMCTestModule()
		mutate(m.FuncByName("read_set_dst"))
		if e := ValidateZ80Symbols(m); e == nil || !strings.Contains(e.Error(), "SMC") {
			t.Fatalf("forged setter admitted: %v", e)
		}
	}
}

func TestNamedSMCCanonicalSymbols(t *testing.T) {
	m := namedSMCTestModule()
	f := m.FuncByName("read")
	f.Name = "mod.read"
	f.Blocks[0].Insts[0].Sym = "mod.read$dst"
	s := m.FuncByName("read_set_dst")
	s.Name = "mod.read_set_dst"
	s.SMCSetterFor = "mod.read$dst"
	s.Blocks[0].Insts[0].Sym = s.SMCSetterFor
	if label := SMCImmediateLabel(s.SMCSetterFor); label != "mod_read$dst$imm" {
		t.Fatal(label)
	}
	if e := ValidateZ80Symbols(m); e != nil {
		t.Fatal(e)
	}
	m.AddFunc("mod_read_set_dst")
	if e := ValidateZ80Symbols(m); e == nil || !strings.Contains(e.Error(), "ambiguous") {
		t.Fatalf("canonical collision admitted: %v", e)
	}
}
