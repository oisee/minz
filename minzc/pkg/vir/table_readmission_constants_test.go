package vir

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestReadmissionDifferentConstantsNative(t *testing.T) {
	var saved RegAllocTable
	saved.Init()
	for i, constant := range []int64{5, 1, 17, 255} {
		m := &mir2.Module{}
		f := m.AddFunc(fmt.Sprintf("plus_%d", constant))
		for n := 0; n < i*13; n++ {
			f.AllocReg()
		}
		x, c, result := f.AllocReg(), f.AllocReg(), f.AllocReg()
		f.Contract.Params = []mir2.Param{{Name: "x", Reg: x, Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		b := f.NewBlock("entry")
		b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: c, Imm: constant, Ty: mir2.TyU8})
		b.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: result, Src: [2]mir2.Reg{x, c}, Ty: mir2.TyU8})
		b.Seal(&mir2.TermRet{Vals: []mir2.Reg{result}})
		p, err := PrepareLocalTableProblem(f, &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: "A"}}})
		if err != nil {
			t.Fatal(err)
		}
		var current RegAllocTable
		current.Init()
		if i == 0 {
			assignment := map[int]int{}
			for _, op := range p.Ops {
				if op.Dst > 0 {
					assignment[op.Dst] = Z80.LocByName("A")
				}
			}
			for v, loc := range p.Options.ParamLocs {
				assignment[v] = loc
			}
			if err := saved.AddChecked(p.Ops, assignment, Z80, p.Options); err != nil {
				t.Fatal(err)
			}
		} else if _, _, hit := saved.LookupChecked(p.Ops, Z80, p.Options); hit {
			t.Fatal("old exact certificate reused")
		}
		n, err := saved.ReadmitCheckedCandidates(p, &current, 1)
		if err != nil || n != 1 {
			t.Fatal(constant, n, err)
		}
		asm, err := p.Emit(&current)
		if err != nil {
			t.Fatal(err)
		}
		bin, err := z80asm.NewAssembler().AssembleString(asm)
		if err != nil || len(bin.Errors) > 0 {
			t.Fatal(err, bin.Errors)
		}
		for input := 0; input < 256; input++ {
			z := emulator.NewRemogattoZ80()
			if err := z.LoadMemory(0x8000, bin.Binary); err != nil {
				t.Fatal(err)
			}
			if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
				t.Fatal(err)
			}
			z.SetRegisters(emulator.Registers{A: byte(input), PC: 0x8000, SP: 0xeffe})
			for steps := 0; steps < 16 && z.GetRegisters().PC != 0xf100; steps++ {
				z.Step()
			}
			r := z.GetRegisters()
			if r.A != byte(int64(input)+constant) || r.PC != 0xf100 || r.SP != 0xf000 {
				t.Fatal(constant, input, r)
			}
		}
		t.Logf("constant=%d binary=%x all256inputs PASS", constant, bin.Binary)
	}
}
