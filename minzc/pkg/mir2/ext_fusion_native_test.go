package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// nativeFn assembles one generated function behind a CALL/HALT stub and runs
// it repeatedly from chosen register states on a single emulator.
type nativeFn struct {
	t   *testing.T
	asm string
	z   *emulator.RemogattoZ80
}

func newNativeFn(t *testing.T, m *Module, ar *AllocResult, name string) *nativeFn {
	t.Helper()
	asm, err := Z80Codegen(m, ar)
	if err != nil {
		t.Fatal(err)
	}
	program, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\nCALL " + name + "\nHALT\n" + asm)
	if err != nil || len(program.Errors) > 0 {
		t.Fatalf("assemble %v %v\n%s", err, program.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	copy(z.Memory()[0x8000:], program.Binary)
	return &nativeFn{t: t, asm: asm, z: z}
}

func (n *nativeFn) run(regs emulator.Registers) emulator.Registers {
	n.t.Helper()
	regs.PC, regs.SP = 0x8000, 0xff00
	n.z.SetRegisters(regs)
	n.z.Unhalt()
	for i := 0; i < 2000 && !n.z.IsHalted(); i++ {
		n.z.Step()
	}
	got := n.z.GetRegisters()
	if !n.z.IsHalted() || got.SP != 0xff00 {
		n.t.Fatalf("did not return cleanly: %+v\n%s", got, n.asm)
	}
	return got
}

// setReg writes v into the named register (a pair or a byte half) of regs.
func setReg(regs *emulator.Registers, name string, v uint16) {
	setByte := func(p *uint16, hi bool) {
		if hi {
			*p = *p&0x00ff | v<<8
		} else {
			*p = *p&0xff00 | v&0xff
		}
	}
	switch name {
	case "A":
		regs.A = byte(v)
	case "BC":
		regs.BC = v
	case "DE":
		regs.DE = v
	case "HL":
		regs.HL = v
	case "IX":
		regs.IX = v
	case "IY":
		regs.IY = v
	case "B":
		setByte(&regs.BC, true)
	case "C":
		setByte(&regs.BC, false)
	case "D":
		setByte(&regs.DE, true)
	case "E":
		setByte(&regs.DE, false)
	case "H":
		setByte(&regs.HL, true)
	case "L":
		setByte(&regs.HL, false)
	case "IXH":
		setByte(&regs.IX, true)
	case "IXL":
		setByte(&regs.IX, false)
	case "IYH":
		setByte(&regs.IY, true)
	case "IYL":
		setByte(&regs.IY, false)
	default:
		panic("setReg " + name)
	}
}

// locOf names an allocation for a byte register, a pair, or a spill slot.
func locOf(name string) PhysLoc {
	switch {
	case name == "mem":
		return PhysLoc{Kind: LocMem, Name: "mem", Offset: 0xF040}
	case name == "remat":
		return PhysLoc{Kind: LocRemat, Name: RematLocName}
	case name == "IX" || name == "IY":
		return PhysLoc{Kind: LocIXY, Name: name}
	case isIXYReg(name):
		return PhysLoc{Kind: LocIXY8, Name: name}
	}
	return PhysLoc{Kind: LocReg, Name: name}
}

// sparePair returns a pair overlapping none of used, or a spill slot.
func sparePair(used ...string) string {
	for _, p := range []string{"BC", "DE", "HL", "IY", "IX"} {
		free := true
		for _, u := range used {
			if u == p || u == lowByte(p) || u == highByte(p) {
				free = false
			}
		}
		if free {
			return p
		}
	}
	return "mem"
}

var wordBoundaries = []uint16{0x0000, 0x0001, 0x007F, 0x00FE, 0x00FF, 0x0100, 0x0101, 0x01FF, 0x7FFF, 0x8000, 0xFF00, 0xFFFE, 0xFFFF}

// cmp.<cond>(x, ext n) and cmp.<cond>(ext n, x), consumed by a branch, for x in
// a pair, an index register or a constant and n in A, a primary byte or an
// index half.  With keep (a live accumulator) the fusion must step aside.
func TestFusedExtCompareVMZ80(t *testing.T) {
	for _, xHome := range []string{"HL", "DE", "IX", "const"} {
		for _, nHome := range []string{"A", "C", "H", "IYL"} {
			if xHome == "HL" && nHome == "H" {
				continue
			}
			for _, keep := range []bool{false, true} {
				if keep && nHome == "A" {
					continue
				}
				for _, cond := range []CmpCond{CmpUlt, CmpUge, CmpUgt, CmpUle} {
					for _, extFirst := range []bool{false, true} {
						name := fmt.Sprintf("x=%s/n=%s/keep=%v/%s/extFirst=%v", xHome, nHome, keep, cond, extFirst)
						t.Run(name, func(t *testing.T) {
							consts := []uint16{0}
							if xHome == "const" {
								consts = []uint16{0x0000, 0x0080, 0x00FF, 0x0100}
							}
							for _, kx := range consts {
								testFusedExtCompare(t, xHome, nHome, keep, cond, extFirst, kx)
							}
						})
					}
				}
			}
		}
	}
}

func testFusedExtCompare(t *testing.T, xHome, nHome string, keep bool, cond CmpCond, extFirst bool, kx uint16) {
	m := &Module{}
	f := m.AddFunc("fused_cmp")
	b := NewBuilder(f)
	entry := b.SwitchToNewBlock("entry")
	var x Reg
	if xHome != "const" {
		x = b.Param("x", TyU16, ClassPair)
	}
	n := b.Param("n", TyU8, ClassGeneral)
	var k Reg
	if keep {
		k = b.Param("keep", TyU8, ClassAcc)
	}
	if xHome == "const" {
		x = b.Const(int64(kx), TyU16, ClassPair)
	}
	e := b.Ext(n, TyU8, TyU16, ClassPair)
	var c Reg
	if extFirst {
		c = b.Cmp(cond, e, x, ClassFlag, false)
	} else {
		c = b.Cmp(cond, x, e, ClassFlag, false)
	}
	b.BrIf(c, "yes", nil, "no", nil)
	ret := func(label string, v int64) Reg {
		b.SwitchToNewBlock(label)
		r := b.Const(v, TyU16, ClassPair)
		if keep {
			b.Ret(r, k)
		} else {
			b.Ret(r)
		}
		return r
	}
	yes, no := ret("yes", 1), ret("no", 0)
	_ = entry
	f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
	if keep {
		f.Contract.Returns = append(f.Contract.Returns, Return{Ty: TyU8, Class: ClassAcc})
	}
	xLoc := xHome
	if xHome == "const" {
		xLoc = "DE"
	}
	ar := &AllocResult{Locs: map[Reg]PhysLoc{x: locOf(xLoc), n: locOf(nHome), e: locOf(sparePair(xLoc, nHome)), c: {Kind: LocFlag, Name: "F"}, yes: locOf("HL"), no: locOf("HL")}}
	if keep {
		ar.Locs[k] = locOf("A")
	}
	nf := newNativeFn(t, m, ar, "fused_cmp")
	fused := strings.Contains(nf.asm, "zero-extension fused")
	if fused == keep {
		t.Fatalf("fusion fired=%v with keep=%v:\n%s", fused, keep, nf.asm)
	}
	vm := NewVM(m)
	for nv := 0; nv < 256; nv++ {
		xs := wordBoundaries
		if xHome == "const" {
			xs = []uint16{kx}
		} else {
			xs = append(append([]uint16{}, xs...), uint16(nv-1), uint16(nv), uint16(nv+1))
		}
		for _, xv := range xs {
			args := []Value{}
			if xHome != "const" {
				args = append(args, Value{I: int64(xv)})
			}
			args = append(args, Value{I: int64(nv)})
			if keep {
				args = append(args, Value{I: 0x5A})
			}
			want, err := vm.CallFunc(f, args)
			if err != nil {
				t.Fatal(err)
			}
			regs := emulator.Registers{BC: 0x1111, DE: 0x2222, HL: 0x3333, IX: 0x4444, IY: 0x5555, A: 0x66}
			if xHome != "const" {
				setReg(&regs, xHome, xv)
			}
			setReg(&regs, nHome, uint16(nv))
			if keep {
				regs.A = 0x5A
			}
			got := nf.run(regs)
			if int64(got.HL) != want[0].I || keep && got.A != 0x5A {
				t.Fatalf("x=%04x n=%02x: VM %v native HL=%04x A=%02x\n%s", xv, nv, want, got.HL, got.A, nf.asm)
			}
		}
	}
}

// or(shl(ext c, 8), ext b) in either operand order: the shift becomes two byte
// moves and the OR a byte move into the low half, for result pairs equal to
// the shifted word, overlapping b, or unrelated.
func TestFusedExtShiftOrVMZ80(t *testing.T) {
	type homes struct{ c, b, w, r string }
	cases := []homes{
		{"A", "E", "BC", "BC"},   // checksum16: result in the shifted word's pair
		{"C", "A", "BC", "BC"},   // c is the low half of the shift destination
		{"B", "L", "BC", "HL"},   // c is the high half of the shift destination
		{"A", "D", "BC", "DE"},   // b is the result's high half: low written first
		{"E", "H", "IX", "HL"},   // index pair to HL (via shadow A)
		{"IXH", "C", "DE", "IY"}, // index half source, other index destination
		{"L", "IYL", "HL", "IX"}, // H/L source into IX, IY half as b
		{"A", "C", "DE", "mem"},  // spilled result: generic path
	}
	for _, h := range cases {
		for _, extFirst := range []bool{false, true} {
			for _, keepA := range []bool{false, true} {
				if keepA && (h.c == "A" || h.b == "A") {
					continue
				}
				t.Run(fmt.Sprintf("%+v/extFirst=%v/keepA=%v", h, extFirst, keepA), func(t *testing.T) {
					m := &Module{}
					f := m.AddFunc("fused_or")
					b := NewBuilder(f)
					b.SwitchToNewBlock("entry")
					c := b.Param("c", TyU8, ClassGeneral)
					lo := b.Param("b", TyU8, ClassGeneral)
					var keep Reg
					if keepA {
						keep = b.Param("keep", TyU8, ClassAcc)
					}
					e1 := b.Ext(c, TyU8, TyU16, ClassPair)
					k := b.Const(8, TyU16, ClassPair)
					w := b.Shl(e1, k, TyU16, ClassPair)
					e2 := b.Ext(lo, TyU8, TyU16, ClassPair)
					var r Reg
					if extFirst {
						r = b.Or(e2, w, TyU16, ClassPair)
					} else {
						r = b.Or(w, e2, TyU16, ClassPair)
					}
					f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
					if keepA {
						b.Ret(r, keep)
						f.Contract.Returns = append(f.Contract.Returns, Return{Ty: TyU8, Class: ClassAcc})
					} else {
						b.Ret(r)
					}
					ar := &AllocResult{Locs: map[Reg]PhysLoc{c: locOf(h.c), lo: locOf(h.b), w: locOf(h.w), r: locOf(h.r), k: {Kind: LocRemat, Name: RematLocName},
						e1: locOf(sparePair(h.b, h.c, h.w)), e2: locOf(sparePair(h.b, h.w))}}
					if keepA {
						ar.Locs[keep] = locOf("A")
					}
					nf := newNativeFn(t, m, ar, "fused_or")
					if h.r != "mem" && strings.Count(nf.asm, "zero-extension fused") != 2 {
						t.Fatalf("both extensions should fuse:\n%s", nf.asm)
					}
					vm := NewVM(m)
					check := func(cv, bv int) {
						args := []Value{{I: int64(cv)}, {I: int64(bv)}}
						if keepA {
							args = append(args, Value{I: 0xA5})
						}
						want, err := vm.CallFunc(f, args)
						if err != nil {
							t.Fatal(err)
						}
						regs := emulator.Registers{BC: 0x1111, DE: 0x2222, HL: 0x3333, IX: 0x4444, IY: 0x5555, A: 0x66}
						setReg(&regs, h.c, uint16(cv))
						setReg(&regs, h.b, uint16(bv))
						if keepA {
							regs.A = 0xA5
						}
						got := nf.run(regs)
						if int64(got.HL) != want[0].I || keepA && got.A != 0xA5 {
							t.Fatalf("c=%02x b=%02x: VM %v native HL=%04x A=%02x\n%s", cv, bv, want, got.HL, got.A, nf.asm)
						}
					}
					for v := 0; v < 256; v++ {
						for _, o := range []int{0x00, 0x01, 0x7F, 0x80, 0xFF} {
							check(v, o)
							check(o, v)
						}
					}
				})
			}
		}
	}
}

// add(acc, ext x) into a pair other than acc's: the high byte is hi(acc) plus
// the carry, computed with ADC/SUB.  Covers carry out of the low byte and the
// 16-bit wrap, IX/IY operands, x in A, a primary byte or an index half, a live
// accumulator, and a spilled sum (generic path).
func TestFusedExtWideningAddIntoOtherPairVMZ80(t *testing.T) {
	type homes struct{ acc, x, s, wide string }
	cases := []homes{
		{"IY", "A", "BC", "DE"}, // window_sum
		{"DE", "C", "HL", "IX"},
		{"BC", "L", "DE", "IY"},
		{"IX", "IYL", "HL", "BC"},
		{"HL", "E", "IX", "BC"},
		{"DE", "IXH", "BC", "HL"},
		{"BC", "A", "mem", "DE"},
	}
	for _, h := range cases {
		for _, extFirst := range []bool{false, true} {
			for _, keepA := range []bool{false, true} {
				if keepA && h.x == "A" {
					continue
				}
				wides := []string{h.wide}
				if keepA {
					wides = append(wides, "mem") // a costly temporary admits PUSH AF
				}
				for _, wide := range wides {
					t.Run(fmt.Sprintf("%+v/wide=%s/extFirst=%v/keepA=%v", h, wide, extFirst, keepA), func(t *testing.T) {
						m := &Module{}
						f := m.AddFunc("widen_add")
						b := NewBuilder(f)
						b.SwitchToNewBlock("entry")
						acc := b.Param("acc", TyU16, ClassPair)
						x := b.Param("x", TyU8, ClassGeneral)
						var keep Reg
						if keepA {
							keep = b.Param("keep", TyU8, ClassAcc)
						}
						e := b.Ext(x, TyU8, TyU16, ClassPair)
						var s Reg
						if extFirst {
							s = b.Add(e, acc, TyU16, ClassPair)
						} else {
							s = b.Add(acc, e, TyU16, ClassPair)
						}
						f.Contract.Returns = []Return{{Ty: TyU16, Class: ClassPointer}}
						if keepA {
							b.Ret(s, keep)
							f.Contract.Returns = append(f.Contract.Returns, Return{Ty: TyU8, Class: ClassAcc})
						} else {
							b.Ret(s)
						}
						ar := &AllocResult{Locs: map[Reg]PhysLoc{acc: locOf(h.acc), x: locOf(h.x), s: locOf(h.s), e: locOf(wide)}}
						if keepA {
							ar.Locs[keep] = locOf("A")
						}
						nf := newNativeFn(t, m, ar, "widen_add")
						fired := strings.Contains(nf.asm, "widening byte add")
						if want := h.s != "mem" && (!keepA || wide == "mem" || wide == "IX" || wide == "IY"); fired != want {
							t.Fatalf("widening add fired=%v want %v:\n%s", fired, want, nf.asm)
						}
						vm := NewVM(m)
						for _, av := range wordBoundaries {
							for xv := 0; xv < 256; xv++ {
								args := []Value{{I: int64(av)}, {I: int64(xv)}}
								if keepA {
									args = append(args, Value{I: 0xC3})
								}
								want, err := vm.CallFunc(f, args)
								if err != nil {
									t.Fatal(err)
								}
								regs := emulator.Registers{BC: 0x1111, DE: 0x2222, HL: 0x3333, IX: 0x4444, IY: 0x5555, A: 0x66}
								setReg(&regs, h.acc, av)
								setReg(&regs, h.x, uint16(xv))
								if keepA {
									regs.A = 0xC3
								}
								got := nf.run(regs)
								if int64(got.HL) != want[0].I || keepA && got.A != 0xC3 {
									t.Fatalf("acc=%04x x=%02x: VM %v native HL=%04x A=%02x\n%s", av, xv, want, got.HL, got.A, nf.asm)
								}
							}
						}
					})
				}
			}
		}
	}
}
