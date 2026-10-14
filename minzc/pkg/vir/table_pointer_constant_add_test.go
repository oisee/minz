package vir

import (
	"bytes"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestCheckedPointerConstantAddNative(t *testing.T) {
	for _, constantTy := range []mir2.Ty{mir2.TyU16, mir2.TyPtr} {
		f, a := byteLoadProblemFixture(t)
		base := f.Contract.Params[0]
		f.Contract.Params = []mir2.Param{base}
		f.Blocks[0].Params = nil
		original := f.Blocks[0].Insts[0]
		constant := f.AllocReg()
		f.Blocks[0].Insts = append([]*mir2.Inst{{Op: mir2.OpConst, Dst: constant, Ty: constantTy, Imm: 1}}, f.Blocks[0].Insts...)
		original.Op = mir2.OpAdd
		original.Src[1] = constant
		p, e := PrepareLocalTableProblem(f, a)
		if e != nil {
			t.Fatal(e)
		}
		assignment := map[int]int{}
		for _, op := range p.Ops {
			if op.Dst > 0 {
				loc := "A"
				if op.Width == 16 {
					loc = "HL"
				}
				assignment[op.Dst] = Z80.LocByName(loc)
			}
		}
		for v, loc := range p.Options.ParamLocs {
			assignment[v] = loc
		}
		var table RegAllocTable
		table.Init()
		if e = table.AddChecked(p.Ops, assignment, Z80, p.Options); e != nil {
			t.Fatal(e)
		}
		asm, e := p.Emit(&table)
		if e != nil {
			t.Fatal(e)
		}
		bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		z := emulator.NewRemogattoZ80()
		memory := z.Memory()
		for i := range memory {
			memory[i] = byte(i*37 + i/256*11)
		}
		if e = z.LoadMemory(0x8000, bin.Binary); e != nil {
			t.Fatal(e)
		}
		memory[0xeff0], memory[0xeff1] = 0, 0x70
		before := append([]byte(nil), memory...)
		for base := 0; base < 65536; base++ {
			z.SetRegisters(emulator.Registers{PC: uint16(bin.Symbols["read_byte"]), SP: 0xeff0, HL: uint16(base), DE: 0x1234, BC: 0x5678, IX: 0x1357, IY: 0x2468, F: byte(base)})
			for steps := 0; steps < 64 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); steps++ {
				z.Step()
			}
			r := z.GetRegisters()
			if r.PC != 0x7000 || r.SP != 0xeff2 || r.A != memory[(base+1)&65535] || r.HL != uint16(base+1) || r.DE != 0x1234 || r.BC != 0x5678 || r.IX != 0x1357 || r.IY != 0x2468 {
				t.Fatalf("%s base%04x registers %+v", constantTy, base, r)
			}
		}
		if !bytes.Equal(before, memory) {
			t.Fatal("read-only constant add wrote RAM")
		}
	}
}
