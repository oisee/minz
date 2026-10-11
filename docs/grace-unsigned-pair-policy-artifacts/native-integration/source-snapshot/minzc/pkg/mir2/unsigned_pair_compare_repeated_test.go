package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestUnsignedPairRepeatedAfterArithmeticNative(t *testing.T) {
	m := &Module{}
	f := m.AddFunc("repeated")
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", TyU16, ClassPair)
	y := b.Param("y", TyU16, ClassPair)
	first := b.Cmp(CmpUlt, x, y, ClassFlag, false)
	b.BrIf(first, "update", nil, "finish", nil)
	b.SwitchToNewBlock("update")
	one := b.Const(1, TyU16, ClassPair)
	next := b.Add(x, one, TyU16, ClassPair)
	second := b.Cmp(CmpUge, next, y, ClassFlag, false)
	b.BrIf(second, "yes", nil, "no", nil)
	b.SwitchToNewBlock("yes")
	b.Ret(y)
	b.SwitchToNewBlock("no")
	b.Ret(next)
	b.SwitchToNewBlock("finish")
	b.Ret(x)
	ar := &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocReg, Name: "HL"}, y: {Kind: LocReg, Name: "DE"}, first: {Kind: LocFlag, Name: "F"}, one: {Kind: LocReg, Name: "BC"}, next: {Kind: LocReg, Name: "HL"}, second: {Kind: LocFlag, Name: "F"}}}
	if e := Verify(m); e != nil {
		t.Fatal(e)
	}
	asm, e := Z80Codegen(m, ar, Z80CodegenOptions{UnsignedPairComparePolicy: UnsignedPairComparePreserveForSpeed})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Count(asm, "    JR NZ, .ucmp_pair_done_") != 2 {
		t.Fatal("two eligible compares not selected", asm)
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
	if e != nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	for _, v := range [][2]uint16{{0, 0}, {0, 1}, {0, 2}, {255, 256}, {255, 257}, {32767, 32768}, {65534, 65535}, {65535, 0}} {
		z := emulator.NewRemogattoZ80()
		z.Reset()
		z.LoadMemory(0x8000, a.Binary)
		z.SetMemory(0xf000, 0)
		z.SetMemory(0xf001, 0x70)
		z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["repeated"]), SP: 0xf000, HL: v[0], DE: v[1], F: 255})
		for step := 0; z.GetRegisters().PC != 0x7000; step++ {
			if step > 70 {
				t.Fatal("repeated did not return")
			}
			z.Step()
		}
		want := v[0]
		if v[0] < v[1] {
			want = v[0] + 1
			if want >= v[1] {
				want = v[1]
			}
		}
		if z.GetRegisters().HL != want {
			t.Fatalf("values%v got%d want%d\n%s", v, z.GetRegisters().HL, want, asm)
		}
	}
}
