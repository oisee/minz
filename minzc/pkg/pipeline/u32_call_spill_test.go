package pipeline

import (
	"encoding/binary"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestU32MemoryAcrossCallProductionVMZ80(t *testing.T) {
	const source = `global input: u32
 global result: u32 = 0
 global tick: u32 = 0
 fun mutate() -> u32 { tick = tick + 65539
 return tick }
 fun run_u32() { input = 4294967294
 let a: u32 = input
 let b: u32 = mutate()
 result = a + b }
 `
	h, err := nanz.Parse(source, "u32_call_spill.nanz")
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h)
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(s.MIR2Module)
	vm.EnsureHeap(65536)
	if _, err := vm.Call("run_u32", nil); err != nil {
		t.Fatal(err)
	}
	want := uint32(65537) // (2^32-2)+65539 modulo 2^32; Book v7 §§3.2,3.4.
	offset := 0
	for _, g := range s.MIR2Module.Globals {
		if g.Name == "result" {
			break
		}
		offset += mir2.ByteWidth(g.Ty)
	}
	if got := binary.LittleEndian.Uint32(vm.ReadHeap(int64(offset), 4)); got != want {
		t.Fatalf("VM=%d want=%d globals=%v\n%s", got, want, s.MIR2Module.Globals, s.MIR2Module.Dump())
	}
	res, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, res.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, res.Binary); err != nil {
		t.Fatal(err)
	}
	mem := z.Memory()
	mem[0xfff0] = 0
	mem[0xfff1] = 0x70
	z.SetRegisters(emulator.Registers{PC: uint16(res.Symbols["run_u32"]), SP: 0xfff0})
	for i := 0; i < 100000; i++ {
		if z.GetPC() == 0x7000 && z.GetSP() == 0xfff2 {
			break
		}
		z.Step()
	}
	if z.GetPC() != 0x7000 || z.GetSP() != 0xfff2 {
		t.Fatalf("return PC=%04x SP=%04x\n%s", z.GetPC(), z.GetSP(), s.Assembly)
	}
	address := res.Symbols["result"]
	if got := binary.LittleEndian.Uint32(mem[address : address+4]); got != want {
		t.Fatalf("Z80=%d want=%d\n%s", got, want, s.Assembly)
	}
}
