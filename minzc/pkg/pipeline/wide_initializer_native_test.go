package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// Book v7 §3.1 declares u24/u32; initialization must preserve every byte.
func TestWideGlobalInitializerNativeAndVM(t *testing.T) {
	const source = `global word:u32=4294967294
 global triple:u24=1193046
 global array:[u32;2]=[305419896,4294967295]
 fun touch() { word=word+1 }`
	h, err := nanz.Parse(source, "wide_initializers")
	if err != nil {
		t.Fatal(err)
	}
	s, err := CompileHIRSteps(h)
	if err != nil {
		t.Fatal(err)
	}
	vm := mir2.NewVM(s.MIR2Module)
	if _, err := vm.Call("touch", nil); err != nil {
		t.Fatal(err)
	}
	want := map[string][]byte{"word": {255, 255, 255, 255}, "triple": {0x56, 0x34, 0x12}, "array": {0x78, 0x56, 0x34, 0x12, 255, 255, 255, 255}}
	offset := int64(0)
	for _, g := range s.MIR2Module.Globals {
		if !bytes.Equal(vm.ReadHeap(offset, len(want[g.Name])), want[g.Name]) {
			t.Fatalf("VM global %s differs", g.Name)
		}
		offset += int64(mir2.ByteWidth(g.Ty))
	}
	result, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil || len(result.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, result.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, result.Binary); err != nil {
		t.Fatal(err)
	}
	mem := z.Memory()
	mem[0xfff0] = 0
	mem[0xfff1] = 0x70
	z.SetRegisters(emulator.Registers{PC: uint16(result.Symbols["touch"]), SP: 0xfff0})
	for i := 0; i < 10000 && !(z.GetPC() == 0x7000 && z.GetSP() == 0xfff2); i++ {
		z.Step()
	}
	if z.GetPC() != 0x7000 || z.GetSP() != 0xfff2 {
		t.Fatal("unbalanced native return")
	}
	for name, b := range want {
		a := result.Symbols[name]
		if !bytes.Equal(mem[a:a+len(b)], b) {
			t.Fatalf("native %s got%x want%x", name, mem[a:a+len(b)], b)
		}
	}
}
