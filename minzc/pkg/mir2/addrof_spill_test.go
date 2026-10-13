package mir2

import (
	"bytes"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestNativeAddrOfSpillPreservesWordAndHL(t *testing.T) {
	m := &Module{Globals: []Global{{Name: "target", Ty: TyU8, Init: []byte{73}}}}
	f := m.AddFunc("probe")
	f.Contract.Params = []Param{{Reg: 1, Name: "live", Ty: TyPtr, Class: ClassPointer}}
	f.Contract.Returns = []Return{{Ty: TyPtr, Class: ClassPointer}}
	b := f.NewBlock("entry")
	b.Insts = []*Inst{{Op: OpAddrOf, Dst: 2, Ty: TyPtr, Cls: ClassPointer, Sym: "target"}, {Op: OpLoad, Dst: 3, Ty: TyU8, Cls: ClassGeneral, Src: [2]Reg{2, 0}}, {Op: OpStore, Ty: TyU8, Src: [2]Reg{1, 3}}}
	b.Term = &TermRet{Vals: []Reg{1}}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: "HL"}, 2: {Kind: LocMem, Name: "_spill_probe_r2"}, 3: {Kind: LocReg, Name: "C"}}, Spilled: []Reg{2}}
	text, e := Z80Codegen(m, ar)
	if e != nil {
		t.Fatal(e)
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + text)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	z := emulator.NewRemogattoZ80WithScreen()
	z.SetROMEnd(0)
	mem := z.Memory()
	copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
	copy(mem[0x8000:], a.Binary)
	entry := a.Symbols["probe"]
	copy(mem[0x7000:], []byte{0xcd, byte(entry), byte(entry >> 8), 0})
	before := append([]byte(nil), mem...)
	z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, HL: 0xc000, A: 0x71, F: 0x85, BC: 0x1234, DE: 0x5678, IX: 0x1357, IY: 0x2468})
	minSP := uint16(0xf000)
	for n := 0; n < 1000 && z.GetPC() != 0x7003; n++ {
		z.Step()
		if z.GetSP() < minSP {
			minSP = z.GetSP()
		}
	}
	target := uint16(a.Symbols["target"])
	slot := a.Symbols["_spill_probe_r2"]
	want := append([]byte(nil), before...)
	want[0xc000] = 73
	want[slot] = byte(target)
	want[slot+1] = byte(target >> 8)
	copy(want[minSP:0xf000], mem[minSP:0xf000])
	if z.GetPC() != 0x7003 || z.GetSP() != 0xf000 || z.GetRegisters().HL != 0xc000 || !bytes.Equal(want, mem) {
		t.Fatalf("spilled address/live HL gotregs%v slot%02x%02x want%04x output%d", z.GetRegisters(), mem[slot+1], mem[slot], target, mem[0xc000])
	}
}
