package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestDivModByteResultPreservesPairNeighborNative(t *testing.T) {
	for _, result := range []string{"B", "C"} {
		neighbor := "B"
		if result == "B" {
			neighbor = "C"
		}
		inst := &Inst{Op: OpMod, Dst: 3, Src: [2]Reg{1, 2}, Ty: TyU8}
		g := fix2CG(inst, map[Reg]string{1: "E", 2: "D", 3: result, 4: neighbor})
		g.curBlock.Term = &TermRet{Vals: []Reg{3, 4}}
		g.genDivMod(inst)
		asm := "ORG 0x8000\narith:\n" + g.sb.String() + "    RET\n"
		image, err := z80asm.NewAssembler().AssembleString(asm)
		if err != nil {
			t.Fatalf("assemble: %v\n%s", err, asm)
		}
		if len(image.Errors) > 0 {
			t.Fatal(err, image.Errors)
		}
		cpu := emulator.NewRemogattoZ80()
		cpu.LoadMemory(0x8000, image.Binary)
		for a := 0; a < 256; a++ {
			for divisor := 1; divisor < 256; divisor++ {
				live := byte(a*53 + divisor*17)
				bc := uint16(live)<<8 | uint16(live)
				cpu.LoadMemory(0xeffe, []byte{0, 0xf1})
				cpu.SetRegisters(emulator.Registers{A: 0x77, BC: bc, DE: uint16(divisor)<<8 | uint16(a), HL: 0x1234, PC: 0x8000, SP: 0xeffe})
				for step := 0; step < 256 && cpu.GetRegisters().PC != 0xf100; step++ {
					cpu.Step()
				}
				regs := cpu.GetRegisters()
				got := byte(regs.BC)
				kept := byte(regs.BC >> 8)
				if result == "B" {
					got, kept = kept, got
				}
				if got != byte(a%divisor) || kept != live || regs.PC != 0xf100 || regs.SP != 0xf000 {
					t.Fatalf("%s a%d divisor%d result%d kept%d want%d regs%+v\n%s", result, a, divisor, got, kept, live, regs, asm)
				}
			}
		}
	}
}
