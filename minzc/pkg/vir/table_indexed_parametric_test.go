package vir

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"io"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestIndexedParametricSourceAndRegisterIdentity(t *testing.T) {
	asset := os.Getenv("INDEXED_ENRT4")
	if asset == "" {
		t.Skip("set INDEXED_ENRT4")
	}
	table := &RegAllocTable{}
	table.Init()
	keys := map[string]bool{}
	var originalShape *EnrichedShape
	originalRecord := -1
	for i, imm := range []int64{3, 17, 255} {
		m := &mir2.Module{}
		f := m.AddFunc([]string{"source_base", "renamed", "renumbered"}[i])
		if i == 2 {
			for j := 0; j < 37; j++ {
				f.AllocReg()
			}
		}
		x, c, r := f.AllocReg(), f.AllocReg(), f.AllocReg()
		f.Contract.Params = []mir2.Param{{Name: "x", Reg: x, Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		b := f.NewBlock("entry")
		b.Append(&mir2.Inst{Op: mir2.OpConst, Dst: c, Imm: imm, Ty: mir2.TyU8})
		b.Append(&mir2.Inst{Op: mir2.OpAdd, Dst: r, Src: [2]mir2.Reg{x, c}, Ty: mir2.TyU8})
		b.Seal(&mir2.TermRet{Vals: []mir2.Reg{r}})
		alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: "A"}}}
		p, e := PrepareLocalTableProblem(f, alloc)
		if e != nil {
			t.Fatal(e)
		}
		key, e := CheckedTableKey(p.Ops, Z80, p.Options)
		if e != nil || keys[key] {
			t.Fatal("name/immediate/register identity failed binding", e)
		}
		keys[key] = true
		if _, _, hit := table.LookupChecked(p.Ops, Z80, p.Options); hit {
			t.Fatal("old exact certificate unexpectedly reused")
		}
		ev, e := ImportIndexedENRT(asset, p, table)
		if e != nil || ev.Import == nil || ev.Import.Admitted != 1 {
			t.Fatal(e, ev)
		}
		if originalShape == nil {
			originalShape = ev.Shape
			originalRecord = ev.RecordIndex
		} else if !reflect.DeepEqual(originalShape, ev.Shape) || originalRecord != ev.RecordIndex {
			t.Fatal("parametric historical proposal not reused")
		}
		asm, e := p.Emit(table)
		if e != nil {
			t.Fatal(e)
		}
		bin, e := z80asm.NewAssembler().AssembleString(asm)
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		for n := 0; n < 256; n++ {
			z := emulator.NewRemogattoZ80()
			z.LoadMemory(0x8000, bin.Binary)
			z.LoadMemory(0xeffe, []byte{0, 0xf1})
			z.SetRegisters(emulator.Registers{A: byte(n), PC: 0x8000, SP: 0xeffe})
			for steps := 0; steps < 16 && z.GetRegisters().PC != 0xf100; steps++ {
				z.Step()
			}
			reg := z.GetRegisters()
			if reg.A != byte(int64(n)+imm) || reg.SP != 0xf000 || reg.PC != 0xf100 {
				t.Fatal(i, n, reg)
			}
		}
	}
}

func TestIndexedReadBudgetDistinctFromTruncation(t *testing.T) {
	for _, err := range []error{io.EOF, io.ErrUnexpectedEOF} {
		if got := indexedReadError(err, 0); got == err || !strings.Contains(got.Error(), "read budget") {
			t.Fatal(got)
		}
		if got := indexedReadError(err, 17); got != err {
			t.Fatal("physical truncation obscured", got)
		}
	}
	if indexedReadError(nil, 0) != nil {
		t.Fatal("exact budget success rejected")
	}
}
