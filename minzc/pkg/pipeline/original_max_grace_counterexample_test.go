package pipeline

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"testing"
)

func TestOriginalMaxGraceCounterexample(t *testing.T) {
	b, e := os.ReadFile("../../../reports/showcase-src/2026-03-10/ex6_foreach.nanz")
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(b), "ex6_foreach.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	s, e := CompileHIRSteps(parse(), Options{ContractOpt: true, UseGrace: true, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	for vi, m := range []*mir2.Module{hir.LowerModule(parse()), s.MIR2Module} {
		v := mir2.NewVM(m)
		v.EnsureHeap(65536)
		v.WriteHeapSlice(0x4000, []byte{0, 73, 146, 219, 36})
		out, e := v.Call("max_chain", []mir2.Value{{I: 0x4000}, {I: 5}})
		t.Logf("VM%d out%v err%v", vi, out, e)
		if e != nil || len(out) != 1 || out[0].I != 219 {
			t.Errorf("VM%d want219", vi)
		}
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	z := emulator.NewRemogattoZ80()
	z.LoadMemory(0x8000, a.Binary)
	z.LoadMemory(0x4000, []byte{0, 73, 146, 219, 36})
	addr := a.Symbols["max_chain"]
	z.LoadMemory(0x7000, []byte{0xcd, byte(addr), byte(addr >> 8), 0x76})
	z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, HL: 0x4000, A: 5})
	for n := 0; n < 10000 && z.GetPC() != 0x7003; n++ {
		z.Step()
	}
	r := z.GetRegisters()
	t.Logf("native%+v", r)
	if r.A != 219 || r.PC != 0x7003 || r.SP != 0xf000 {
		t.Errorf("native want219/PC7003/SPf000")
	}
}
