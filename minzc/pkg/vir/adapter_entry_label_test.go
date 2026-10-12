package vir

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestAdapterUsesExactFunctionEntryNative(t *testing.T) {
	f := &mir2.Func{Name: "target", Contract: mir2.Contract{Params: []mir2.Param{{Reg: 1, Ty: mir2.TyU8}}}}
	asm, err := emitAdapterEntry("ORG 0x8000\nother:\nRET\ntarget:\nLD A, B\nRET\n", f, map[int]int{1: Z80.LocByName("A")}, map[int]int{1: Z80.LocByName("B")}, Z80)
	if err != nil {
		t.Fatal(err)
	}
	ar, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(ar.Errors) > 0 {
		t.Fatalf("assembly %v %v", err, ar.Errors)
	}
	z := emulator.NewRemogattoZ80()
	if err := z.LoadMemory(0x8000, ar.Binary); err != nil {
		t.Fatal(err)
	}
	for x := 0; x < 256; x++ {
		if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		// The unrelated RET must remain one byte before the target label.
		z.SetRegisters(emulator.Registers{A: byte(x), BC: 0x9127, F: 0x45, PC: 0x8001, SP: 0xeffe})
		for i := 0; i < 32 && z.GetRegisters().PC != 0xf100; i++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.A != byte(x) || r.BC != uint16(x)<<8|0x27 || r.F != 0x45 || r.PC != 0xf100 || r.SP != 0xf000 {
			t.Fatalf("x=%d got=%+v ASM=%s", x, r, asm)
		}
	}
}

func TestAdapterRejectsMissingOrDuplicateEntry(t *testing.T) {
	f := &mir2.Func{Name: "target", Contract: mir2.Contract{Params: []mir2.Param{{Reg: 1, Ty: mir2.TyU8}}}}
	for _, asm := range []string{"other:\nRET\n", "target:\nRET\ntarget:\nRET\n"} {
		result, err := emitAdapterEntry(asm, f, map[int]int{1: Z80.LocByName("A")}, map[int]int{1: Z80.LocByName("B")}, Z80)
		if err == nil || result != "" {
			t.Fatalf("ambiguous entry accepted: %q %v", result, err)
		}
	}
}
