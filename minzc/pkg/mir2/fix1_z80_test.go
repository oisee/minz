package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func fix1Execute(t *testing.T, asm string) emulator.Registers {
	t.Helper()
	res, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble %v %v\n%s", err, res.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	z.Reset()
	z.LoadMemory(0x8000, res.Binary)
	z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
	for n := 0; !z.IsHalted(); n++ {
		if n >= 200000 {
			t.Fatal("no HALT")
		}
		z.Step()
	}
	return z.GetRegisters()
}
func TestFix1WordOverlap(t *testing.T) {
	for _, op := range []mir2.Op{mir2.OpAdd, mir2.OpSub, mir2.OpAnd, mir2.OpOr, mir2.OpXor} {
		t.Run(op.String(), func(t *testing.T) {
			for _, lhs := range []string{"BC", "DE", "HL"} {
				for _, rhs := range []string{"BC", "DE", "HL"} {
					for _, dst := range []string{"BC", "DE", "HL"} {
						m := &mir2.Module{Name: "overlap"}
						f := m.AddFunc("arith")
						f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU16, Class: mir2.ClassPair}}
						b := mir2.NewBuilder(f)
						b.SwitchToNewBlock("entry")
						a := b.Param("a", mir2.TyU16, mir2.ClassPair)
						c := b.Param("b", mir2.TyU16, mir2.ClassPair)
						out := b.BinOp(op, a, c, mir2.TyU16, mir2.ClassPair)
						b.Ret(out)
						alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocReg, Name: lhs}, c: {Kind: mir2.LocReg, Name: rhs}, out: {Kind: mir2.LocReg, Name: dst}}}
						values := map[string]uint16{"BC": 0x1234, "DE": 0x4567, "HL": 0x89AB}
						x, y := values[lhs], values[rhs]
						want := x + y
						switch op {
						case mir2.OpSub:
							want = x - y
						case mir2.OpAnd:
							want = x & y
						case mir2.OpOr:
							want = x | y
						case mir2.OpXor:
							want = x ^ y
						}
						asm := "ORG 0x8000\nLD SP,0xFF00\nLD BC,4660\nLD DE,17767\nLD HL,35243\nCALL arith\nDI\nHALT\n" + mir2.Z80Codegen(m, alloc)
						r := fix1Execute(t, asm)
						if r.HL != want {
							t.Fatalf("%s %s,%s -> %s got %x want %x\n%s", op, lhs, rhs, dst, r.HL, want, asm)
						}
					}
				}
			}
		})
	}
}
func TestFix1SignedSpillOperands(t *testing.T) {
	for _, op := range []mir2.Op{mir2.OpSDiv, mir2.OpSMod} {
		t.Run(op.String(), func(t *testing.T) {
			for _, loc := range []mir2.PhysLoc{{Kind: mir2.LocMem}, {Kind: mir2.LocIXY8, Name: "IXL"}, {Kind: mir2.LocReg, Name: "C"}, {Kind: mir2.LocIXY, Name: "IX"}} {
				m := &mir2.Module{Name: "spill"}
				f := m.AddFunc("arith")
				f.Contract.Returns = []mir2.Return{{Ty: mir2.TyI16, Class: mir2.ClassPair}}
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				a := b.Param("a", mir2.TyI16, mir2.ClassPair)
				c := b.Param("b", mir2.TyI16, mir2.ClassPair)
				out := b.BinOp(op, a, c, mir2.TyI16, mir2.ClassPair)
				b.Ret(out)
				alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocMem}, c: loc, out: {Kind: mir2.LocReg, Name: "HL"}}}
				setup := fmt.Sprintf("LD HL,65529\nLD (_spill_arith_r%d),HL\n", a)
				if loc.Kind == mir2.LocMem {
					setup += fmt.Sprintf("LD HL,2\nLD (_spill_arith_r%d),HL\n", c)
				} else if loc.Name == "IXL" {
					setup += "LD IX,2\n"
				} else {
					setup += fmt.Sprintf("LD %s,2\n", loc.Name)
				}
				asm := "ORG 0x8000\nLD SP,0xFF00\n" + setup + "CALL arith\nDI\nHALT\n" + mir2.Z80Codegen(m, alloc)
				got := fix1Execute(t, asm).HL
				want := uint16(65533)
				if op == mir2.OpSMod {
					want = 65535
				}
				if got != want {
					t.Fatalf("%s %+v got %d want %d\n%s", op, loc, got, want, asm)
				}
			}
		})
	}
}

func TestFix1PointerOverlap(t *testing.T) {
	// The lookup-table shape: base in BC, offset in HL. Choosing BC as
	// offset staging would overwrite the base and produce twice the offset.
	m := &mir2.Module{Name: "pointer"}
	f := m.AddFunc("arith")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyPtr, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	base := b.Param("base", mir2.TyPtr, mir2.ClassPointer)
	off := b.Param("off", mir2.TyU16, mir2.ClassPair)
	out := b.PtrAdd(base, off, mir2.ClassPointer)
	b.Ret(out)
	alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{base: {Kind: mir2.LocReg, Name: "BC"}, off: {Kind: mir2.LocReg, Name: "HL"}, out: {Kind: mir2.LocReg, Name: "HL"}}}
	asm := "ORG 0x8000\nLD SP,0xFF00\nLD BC,4660\nLD HL,5\nCALL arith\nDI\nHALT\n" + mir2.Z80Codegen(m, alloc)
	if got := fix1Execute(t, asm).HL; got != 4665 {
		t.Fatalf("base+offset got %d want 4665\n%s", got, asm)
	}
}

func TestFix1NarrowWordCompare(t *testing.T) {
	for _, cond := range []mir2.CmpCond{mir2.CmpUlt, mir2.CmpUgt} {
		m := &mir2.Module{Name: "compare"}
		f := m.AddFunc("arith")
		f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		a := b.Param("byte", mir2.TyU8, mir2.ClassGeneral)
		w := b.Param("word", mir2.TyU16, mir2.ClassPair)
		lhs, rhs := a, w
		if cond == mir2.CmpUgt {
			lhs, rhs = w, a
		}
		c := b.Cmp(cond, lhs, rhs, mir2.ClassFlag, false)
		b.BrIf(c, "yes", nil, "no", nil)
		b.SwitchToNewBlock("yes")
		one := b.Const(1, mir2.TyU8, mir2.ClassAcc)
		b.Ret(one)
		b.SwitchToNewBlock("no")
		zero := b.Const(0, mir2.TyU8, mir2.ClassAcc)
		b.Ret(zero)
		alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{a: {Kind: mir2.LocReg, Name: "D"}, w: {Kind: mir2.LocReg, Name: "HL"}, c: {Kind: mir2.LocReg, Name: "F"}, one: {Kind: mir2.LocReg, Name: "A"}, zero: {Kind: mir2.LocReg, Name: "A"}}}
		for _, word := range []int{254, 255, 256, 300, 65535} {
			asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD D,255\nLD HL,%d\nCALL arith\nDI\nHALT\n", word) + mir2.Z80Codegen(m, alloc)
			want := uint8(0)
			if word > 255 {
				want = 1
			}
			if got := fix1Execute(t, asm).A; got != want {
				t.Errorf("%s word %d got %d want %d\n%s", cond, word, got, want, asm)
			}
		}
	}
}

func TestFix1ByteResultWithLiveA(t *testing.T) {
	m := &mir2.Module{Name: "live_acc"}
	f := m.AddFunc("arith")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
	a := b.Param("a", mir2.TyU8, mir2.ClassAcc)
	two := b.Const(2, mir2.TyU8, mir2.ClassGeneral)
	div := b.Div(x, two, mir2.TyU8, mir2.ClassGeneral)
	out := b.Add(a, div, mir2.TyU8, mir2.ClassAcc)
	b.Ret(out)
	alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: "B"}, a: {Kind: mir2.LocReg, Name: "A"}, two: {Kind: mir2.LocReg, Name: "D"}, div: {Kind: mir2.LocReg, Name: "C"}, out: {Kind: mir2.LocReg, Name: "A"}}}
	asm := "ORG 0x8000\nLD SP,0xFF00\nLD B,10\nLD A,7\nCALL arith\nDI\nHALT\n" + mir2.Z80Codegen(m, alloc)
	if got := fix1Execute(t, asm).A; got != 12 {
		t.Fatalf("10/2+7: got %d want 12\n%s", got, asm)
	}
}

func TestFix1BitPredicateConstantOverlap(t *testing.T) {
	m := &mir2.Module{Name: "bit"}
	f := m.AddFunc("arith")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyBool, Class: mir2.ClassAcc}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	x := b.Param("x", mir2.TyU16, mir2.ClassPair)
	one := b.Const(1, mir2.TyU16, mir2.ClassPair)
	masked := b.And(x, one, mir2.TyU16, mir2.ClassPair)
	zero := b.Const(0, mir2.TyU16, mir2.ClassPair)
	out := b.Cmp(mir2.CmpEq, masked, zero, mir2.ClassFlag, false)
	b.Ret(out)
	alloc := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{x: {Kind: mir2.LocReg, Name: "HL"}, one: {Kind: mir2.LocReg, Name: "BC"}, masked: {Kind: mir2.LocReg, Name: "DE"}, zero: {Kind: mir2.LocReg, Name: "HL"}, out: {Kind: mir2.LocReg, Name: "F"}}}
	for _, v := range []int{0, 1, 2, 7, 255, 256, 257} {
		asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\nLD HL,%d\nCALL arith\nDI\nHALT\n", v) + mir2.Z80Codegen(m, alloc)
		want := uint8(0)
		if v%2 == 0 {
			want = 1
		}
		if got := fix1Execute(t, asm).A; got != want {
			t.Errorf("even(%d): got %d want %d\n%s", v, got, want, asm)
		}
	}
}
