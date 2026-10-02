package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"testing"
)

func TestP8SpilledByteExtension(t *testing.T) {
	for _, signed := range []bool{false, true} {
		t.Run(fmt.Sprint(signed), func(t *testing.T) {
			m := &mir2.Module{Name: "p8"}
			f := m.AddFunc("arith")
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			a := b.Param("a", mir2.TyU8, mir2.ClassGeneral)
			var out mir2.Reg
			if signed {
				out = b.Sext(a, mir2.TyU8, mir2.TyU16, mir2.ClassPair)
			} else {
				out = b.Ext(a, mir2.TyU8, mir2.TyU16, mir2.ClassPair)
			}
			b.Ret(out)
			ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocReg, Name: "C"}, out: {Kind: mir2.LocMem}}}
			asm := "ORG 0x8000\nLD SP,0xFF00\nLD C,254\nCALL arith\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, ar))
			want := uint16(254)
			if signed {
				want = 65534
			}
			if got := fix1Execute(t, asm).HL; got != want {
				t.Fatalf("got %x want %x\n%s", got, want, asm)
			}
		})
	}
}

func TestP8ByteALUWithAccumulatorRHS(t *testing.T) {
	for _, op := range []mir2.Op{mir2.OpAdd, mir2.OpSub} {
		t.Run(op.String(), func(t *testing.T) {
			m := &mir2.Module{Name: "p8"}
			f := m.AddFunc("arith")
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			a := b.Param("a", mir2.TyU8, mir2.ClassGeneral)
			c := b.Param("b", mir2.TyU8, mir2.ClassAcc)
			out := b.BinOp(op, a, c, mir2.TyU8, mir2.ClassAcc)
			b.Ret(out)
			ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocMem}, c: {Kind: mir2.LocReg, Name: "A"}, out: {Kind: mir2.LocReg, Name: "A"}}}
			asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD A,29\nLD (_spill_arith_r%d),A\nLD A,7\nCALL arith\nDI\nHALT\n", a) + mustZ80Asm(mir2.Z80Codegen(m, ar))
			want := uint8(36)
			if op == mir2.OpSub {
				want = 22
			}
			if got := fix1Execute(t, asm).A; got != want {
				t.Fatalf("got %d want %d", got, want)
			}
		})
	}
}

func TestP8SubtractIndexByte(t *testing.T) {
	m := &mir2.Module{Name: "p8"}
	f := m.AddFunc("arith")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU16, mir2.ClassPair)
	c := b.Param("b", mir2.TyU8, mir2.ClassGeneral)
	out := b.Sub(a, c, mir2.TyU16, mir2.ClassPair)
	b.Ret(out)
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocReg, Name: "HL"}, c: {Kind: mir2.LocIXY8, Name: "IYL"}, out: {Kind: mir2.LocReg, Name: "HL"}}}
	asm := "ORG 0x8000\nLD SP,0xFF00\nLD HL,1000\nLD IY,7\nCALL arith\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, ar))
	if got := fix1Execute(t, asm).HL; got != 993 {
		t.Fatalf("got %d want 993", got)
	}
}

func TestP8SpilledConstantPreservesAccumulator(t *testing.T) {
	m := &mir2.Module{Name: "p8"}
	f := m.AddFunc("arith")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	c := b.Const(7, mir2.TyU8, mir2.ClassGeneral)
	out := b.Add(a, c, mir2.TyU8, mir2.ClassAcc)
	b.Ret(out)
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocReg, Name: "A"}, c: {Kind: mir2.LocMem}, out: {Kind: mir2.LocReg, Name: "A"}}}
	want, err := mir2.NewVM(m).Call("arith", []mir2.Value{{I: 29}})
	if err != nil {
		t.Fatal(err)
	}
	asm := "ORG 0x8000\nLD SP,0xFF00\nLD A,29\nCALL arith\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, ar))
	if got := fix1Execute(t, asm).A; got != uint8(want[0].I) {
		t.Fatalf("got %d want %d\n%s", got, want[0].I, asm)
	}
}

func TestP8WidenIndexByteToSpill(t *testing.T) {
	m := &mir2.Module{Name: "p8"}
	f := m.AddFunc("arith")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU8, mir2.ClassGeneral)
	out := b.Move(a, mir2.TyU16, mir2.ClassPair)
	b.Ret(out)
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocIXY8, Name: "IYL"}, out: {Kind: mir2.LocMem}}}
	asm := "ORG 0x8000\nLD SP,0xFF00\nLD IY,7\nCALL arith\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, ar))
	if got := fix1Execute(t, asm).HL; got != 7 {
		t.Fatalf("got %d want 7", got)
	}
}

func TestP8WideSpill(t *testing.T) {
	for _, op := range []mir2.Op{mir2.OpAdd, mir2.OpSub, mir2.OpSar} {
		t.Run(op.String(), func(t *testing.T) {
			m := &mir2.Module{Name: "p8"}
			f := m.AddFunc("arith")
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyI32, Class: mir2.ClassDWord}}
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			a := b.Const(0x12345678, mir2.TyI32, mir2.ClassDWord)
			c := b.Const(3, mir2.TyI32, mir2.ClassDWord)
			out := b.BinOp(op, a, c, mir2.TyI32, mir2.ClassDWord)
			b.Ret(out)
			ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocMem}, c: {Kind: mir2.LocMem}, out: {Kind: mir2.LocMem}}}
			want, err := mir2.NewVM(m).Call("arith", nil)
			if err != nil {
				t.Fatal(err)
			}
			asm := "ORG 0x8000\nLD SP,0xFF00\nCALL arith\nPUSH HL\nEXX\nPUSH HL\nEXX\nPOP BC\nPOP HL\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, ar))
			regs := fix1Execute(t, asm)
			got := uint32(regs.HL) | uint32(regs.BC)<<16
			if got != uint32(want[0].I) {
				t.Fatalf("got %x want %x\n%s", got, want[0].I, asm)
			}
		})
	}
}

func TestP8ConditionalIntrinsicAndSanitizedCall(t *testing.T) {
	for _, sym := range []string{"@mir.io.print.nl", "callee$with.dots"} {
		t.Run(sym, func(t *testing.T) {
			m := &mir2.Module{Name: "p8"}
			f := m.AddFunc("arith")
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			f.Blocks[0].Insts = append(f.Blocks[0].Insts, &mir2.Inst{Op: mir2.OpCallCond, Sym: sym, Cond: mir2.CmpEq, Ty: mir2.TyVoid})
			b.Ret()
			if sym != "@mir.io.print.nl" {
				callee := m.AddFunc(sym)
				c := mir2.NewBuilder(callee)
				c.SwitchToNewBlock("entry")
				c.Ret()
			}
			asm := "ORG 0x8000\nLD SP,0xFF00\nXOR A\nCALL arith\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}))
			fix1Execute(t, asm)
		})
	}
}

func TestP8WordSpillShift(t *testing.T) {
	for _, op := range []mir2.Op{mir2.OpShl, mir2.OpShr, mir2.OpSar} {
		t.Run(op.String(), func(t *testing.T) {
			m := &mir2.Module{Name: "p8"}
			f := m.AddFunc("arith")
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyI16, Class: mir2.ClassPair}}
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			a := b.Param("a", mir2.TyI16, mir2.ClassPair)
			c := b.Const(3, mir2.TyU8, mir2.ClassGeneral)
			out := b.BinOp(op, a, c, mir2.TyI16, mir2.ClassPair)
			b.Ret(out)
			ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocReg, Name: "DE"}, c: {Kind: mir2.LocReg, Name: "B"}, out: {Kind: mir2.LocMem}}}
			want, err := mir2.NewVM(m).Call("arith", []mir2.Value{{I: 0x9234}})
			if err != nil {
				t.Fatal(err)
			}
			asm := "ORG 0x8000\nLD SP,0xFF00\nLD DE,0x9234\nCALL arith\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, ar))
			if got := fix1Execute(t, asm).HL; got != uint16(want[0].I) {
				t.Fatalf("got %x want %x", got, want[0].I)
			}
		})
	}
}

func TestP8WideSpillInPlace(t *testing.T) {
	m := &mir2.Module{Name: "p8"}
	f := m.AddFunc("arith")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU32, Class: mir2.ClassDWord}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyU32, mir2.ClassDWord)
	c := b.Const(3, mir2.TyU32, mir2.ClassDWord)
	b.Add(a, c, mir2.TyU32, mir2.ClassDWord)
	f.Blocks[0].Insts[1].Dst = a
	b.Ret(a)
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocMem}, c: {Kind: mir2.LocDWord, Name: "DE"}}}
	want, err := mir2.NewVM(m).Call("arith", []mir2.Value{{I: 0x12345678}})
	if err != nil {
		t.Fatal(err)
	}
	asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD HL,0x5678\nLD (_spill_arith_r%d),HL\nLD HL,0x1234\nLD (_spill_arith_r%d+2),HL\nCALL arith\nPUSH HL\nEXX\nPUSH HL\nEXX\nPOP BC\nPOP HL\nDI\nHALT\n", a, a) + mustZ80Asm(mir2.Z80Codegen(m, ar))
	regs := fix1Execute(t, asm)
	got := uint32(regs.HL) | uint32(regs.BC)<<16
	if got != uint32(want[0].I) {
		t.Fatalf("got %x want %x", got, want[0].I)
	}
}
