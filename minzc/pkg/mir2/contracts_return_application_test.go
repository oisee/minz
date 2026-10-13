package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func buildFixedCounterReturn() *Module {
	m := &Module{Name: "fixed_return"}
	f := m.AddFunc("counter_result")
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassCounter}}
	b := f.NewBlock("entry")
	v := f.AllocReg()
	b.Append(&Inst{Op: OpConst, Dst: v, Imm: 37, Ty: TyU8, Cls: ClassAcc})
	b.Seal(&TermRet{Vals: []Reg{v}})
	return m
}

func TestContractChoicesMatchAppliedReturnConvention(t *testing.T) {
	for _, optimizer := range []struct {
		name string
		run  func(*Module, CostTable) ContractSet
	}{{"greedy", OptimizeContracts}, {"pbqp", OptimizeContractsPBQP}} {
		t.Run(optimizer.name, func(t *testing.T) {
			m := buildFixedCounterReturn()
			f := m.FuncByName("counter_result")
			cs := optimizer.run(m, Z80CostTable{})
			chosen := cs[f.Name].ReturnClasses[0]
			ApplyContracts(m, cs)
			if chosen != f.Contract.Returns[0].Class {
				t.Fatalf("selected return %s but applied %s", chosen, f.Contract.Returns[0].Class)
			}
		})
	}
}

func TestContractCandidatesKeepAllReturnSlots(t *testing.T) {
	m := buildFixedCounterReturn()
	f := m.FuncByName("counter_result")
	f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassCounter}, {Ty: TyU16, Class: ClassIndex}, {Ty: TyBool, Class: ClassFlag}}
	param := f.AllocReg()
	f.Contract.Params = []Param{{Name: "x", Reg: param, Ty: TyU8, Class: ClassGeneral}}
	candidates := candidateParamOnlyChoices(f, Z80CostTable{})
	if len(candidates) < 2 {
		t.Fatal("parameter choices no longer vary")
	}
	for _, ch := range candidates {
		for i, ret := range f.Contract.Returns {
			if ch.ReturnClasses[i] != ret.Class {
				t.Fatalf("slot %d varied", i)
			}
		}
	}
}

func TestFixedCounterReturnVMZ80(t *testing.T) {
	m := buildFixedCounterReturn()
	f := m.FuncByName("counter_result")
	cs := OptimizeContracts(m, Z80CostTable{})
	ApplyContracts(m, cs)
	got, err := NewVM(m).Call(f.Name, nil)
	if err != nil || len(got) != 1 || got[0].I != 37 {
		t.Fatalf("VM got %v error%v", got, err)
	}
	ar := Allocate(f, ComputeLiveness(f), Z80CostTable{})
	asm, err := Z80Codegen(m, ar)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(asm, "-> u8 = B") {
		t.Fatalf("fixed B return ABI lost: %s", asm)
	}
	result, err := z80asm.NewAssembler().AssembleString("ORG $8000\nLD SP,$f000\nCALL counter_result\nLD A,B\nDI\nHALT\n" + asm)
	if err != nil || len(result.Errors) != 0 {
		t.Fatalf("assemble %v diagnostics%v", err, result.Errors)
	}
	z := emulator.NewRemogattoZ80()
	z.MaxCycles = 1000
	if err = z.LoadMemory(0x8000, result.Binary); err != nil {
		t.Fatal(err)
	}
	z.SetPC(0x8000)
	if err = z.Run(); err != nil {
		t.Fatal(err)
	}
	regs := z.GetRegisters()
	if regs.A != 37 || regs.SP != 0xf000 {
		t.Fatalf("Z80 A=%d SP=%04x", regs.A, regs.SP)
	}
	t.Logf("selected=%s applied=%s actual B/A=%d/%d SP=%04x BIN=%x T=%d", cs[f.Name].ReturnClasses[0], f.Contract.Returns[0].Class, regs.BC>>8, regs.A, regs.SP, result.Binary, z.GetCycles())
}
