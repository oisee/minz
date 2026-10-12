package mir2

import (
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// An argument used ONLY by a conditional call must survive scratch staging of
// its predicate. BC is the value; rhs byte A promotes through BC in genCmp16.
func TestConditionalCallArgumentAcrossWordCompare(t *testing.T) {
	for _, invert := range []bool{false, true} {
		m := &Module{Globals: []Global{{Name: "observed", Ty: TyU16}, {Name: "calls", Ty: TyU8}}}
		f := m.AddFunc("entry")
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		x := b.Param("x", TyU16, ClassGeneral)
		zero := b.Const(0, TyU8, ClassAcc)
		c := b.Cmp(CmpNe, x, zero, ClassFlag, false)
		cc := CmpNe
		if invert {
			cc = CmpEq
		}
		b.Cur.Insts = append(b.Cur.Insts, &Inst{Op: OpCallCond, Src: [2]Reg{c}, Args: []Reg{x}, Sym: "observer", Ty: TyVoid, Cond: cc})
		b.Ret()
		h := m.AddFunc("observer")
		hb := NewBuilder(h)
		hb.SwitchToNewBlock("entry")
		v := hb.Param("v", TyU16, ClassPointer)
		p := hb.AddrOf("observed", ClassPointer)
		hb.Store(p, v, TyU16)
		q := hb.AddrOf("calls", ClassPointer)
		one := hb.Const(1, TyU8, ClassAcc)
		hb.Store(q, one, TyU8)
		hb.Ret()
		ar := &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocReg, Name: "BC"}, zero: {Kind: LocReg, Name: "A"}, c: {Kind: LocFlag, Name: "F"}, v: {Kind: LocReg, Name: "HL"}, p: {Kind: LocReg, Name: "DE"}, q: {Kind: LocReg, Name: "HL"}, one: {Kind: LocReg, Name: "A"}}}
		asm, e := Z80Codegen(m, ar, Z80CodegenOptions{})
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		for _, input := range []uint16{0, 1, 2, 255, 256, 0x7fff, 0x8000, 0xffff} {
			wantTaken := input != 0
			if invert {
				wantTaken = !wantTaken
			}
			want := uint16(0)
			if wantTaken {
				want = input
			}
			vm := NewVM(m)
			if _, e = vm.Call("entry", []Value{{I: int64(input)}}); e != nil {
				t.Fatal(e)
			}
			d := vm.ReadHeap(vm.GlobalAddr("observed"), 2)
			got := uint16(d[0]) | uint16(d[1])<<8
			if got != want || vm.ReadHeap(vm.GlobalAddr("calls"), 1)[0] != boolByte(wantTaken) {
				t.Fatal("VM conditional effects", input, invert, got, want)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.LoadMemory(0x8000, a.Binary)
			target := a.Symbols["entry"]
			z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
			z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, BC: input, DE: 0x2345, HL: 0x3456, IX: 0x4567, IY: 0x5678})
			before := append([]byte(nil), z.Memory()...)
			minSP := uint16(0xf000)
			done := false
			for i := 0; i < 1000; i++ {
				if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
					done = true
					break
				}
				z.Step()
				if z.GetSP() < minSP {
					minSP = z.GetSP()
				}
			}
			addr := a.Symbols["observed"]
			got = uint16(z.Memory()[addr]) | uint16(z.Memory()[addr+1])<<8
			if !done || got != want || z.Memory()[a.Symbols["calls"]] != boolByte(wantTaken) {
				t.Fatalf("native conditional args input%04x invert%v returned%v observed%04x want%04x\n%s", input, invert, done, got, want, asm)
			}
			for p, v := range before {
				if p >= int(minSP) && p < 0xf000 || p == addr || p == addr+1 || p == a.Symbols["calls"] {
					continue
				}
				if z.Memory()[p] != v {
					t.Fatal("RAM", p)
				}
			}
		}
	}
}
func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}
