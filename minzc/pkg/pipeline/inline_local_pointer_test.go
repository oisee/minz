package pipeline

import (
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestInlineLocalPointerCPMNative(t *testing.T) {
	const source = `struct Pair { unsigned char x, y; };
static unsigned char value(struct Pair *p) { return p->x * p->y + p->x; }
unsigned char result;
void main(void) { struct Pair p; p.x=10; p.y=5; result=value(&p); }`
	h, err := c89.CompileWithOpts(source, "inline-local.c", c89.CompileOpts{NoLibc: true})
	if err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(h)
	s, err := CompileHIRSteps(h, Options{ContractOpt: true, Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if s.MIR2Module.FuncByName("value") != nil {
		t.Fatalf("private pointer helper was not inlined:\n%s", s.MIR2Module.Dump())
	}
	fields := 0
	for _, b := range s.MIR2Module.FuncByName("main").Blocks {
		for _, i := range b.Insts {
			if i.Op == mir2.OpField {
				fields++
				if i.ExtraRets != nil {
					t.Fatal("inlining added return metadata to local field", i.ExtraRets)
				}
			}
		}
	}
	if fields == 0 {
		t.Fatal("expected an inlined local field address")
	}
	for _, m := range []*mir2.Module{raw, s.MIR2Module} {
		vm := mir2.NewVM(m)
		if _, err := vm.Call("main", nil); err != nil {
			t.Fatal(err)
		}
		if got := vm.ReadHeap(vm.GlobalAddr("result"), 1)[0]; got != 60 {
			t.Fatal("VM result", got)
		}
	}
	image, err := z80asm.NewAssembler().AssembleString(s.Assembly)
	if err != nil || len(image.Errors) != 0 {
		t.Fatal(err, image.Errors)
	}
	address, ok := image.Symbols["result"]
	if !ok {
		t.Fatal("result symbol missing")
	}
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	if err := z.LoadMemory(0x100, image.Binary); err != nil {
		t.Fatal(err)
	}
	z.Memory()[0xeffe], z.Memory()[0xefff] = 0, 0
	z.SetRegisters(emulator.Registers{PC: 0x100, SP: 0xeffe})
	for steps := 0; steps < 1000; steps++ {
		if z.GetPC() == 0 && z.GetSP() == 0xf000 {
			if got := z.Memory()[address]; got != 60 {
				t.Fatal("Z80 result", got)
			}
			return
		}
		z.Step()
	}
	t.Fatal("CP/M entry did not return", z.GetRegisters())
}
