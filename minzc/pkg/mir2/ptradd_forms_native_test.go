package mir2

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
)

// ptr_add(base, off) for the forms in z80codegen_ptradd.go, checked against
// the MIR2 VM on every byte offset and the 16-bit carry boundaries.
//
// base is either an addr_of of a global (whose home may be a pair or a spill,
// in which case the symbol is used as an immediate) or a word parameter.
// "live" keeps the offset live after the ptr_add (returned in DE); "hold"
// keeps an unrelated word live in HL; keepA keeps an accumulator live.
func TestPtrAddFormsVMZ80(t *testing.T) {
	type tc struct {
		name              string
		symBase           bool
		base, off, dst    string
		byteOff           bool
		live, hold, keepA bool
		busy              bool   // DE and BC hold live words (returned in DE and IX)
		form              string // expected comment, "" for the generic path
	}
	cases := []tc{
		{name: "checksum16", symBase: true, base: "BC", off: "HL", dst: "BC", live: true, form: "byte-wise"},
		{name: "index-base", symBase: true, base: "IX", off: "HL", dst: "IX", live: true, form: "byte-wise"},
		{name: "commute-BC", base: "BC", off: "HL", dst: "HL", form: "offset in HL"},
		{name: "commute-DE-to-IX", base: "DE", off: "HL", dst: "IX", form: "offset in HL"},
		{name: "const-base-HL", symBase: true, base: "mem", off: "DE", dst: "HL", form: "constant base"},
		{name: "const-base-to-BC", symBase: true, base: "mem", off: "BC", dst: "DE", form: "constant base"},
		{name: "const-base-busy-HL", symBase: true, base: "mem", off: "HL", dst: "DE", live: true, form: "byte-wise"},
		{name: "const-base-hold", symBase: true, base: "mem", off: "BC", dst: "BC", hold: true, form: "byte-wise"},
		{name: "byte-C", symBase: true, base: "mem", off: "C", dst: "DE", byteOff: true, form: "byte-wise"},
		{name: "byte-A-IY", base: "IY", off: "A", dst: "HL", byteOff: true, form: "byte-wise"},
		{name: "byte-IXL-into-IX", symBase: true, base: "mem", off: "IXL", dst: "IX", byteOff: true, form: "byte-wise"},
		{name: "byte-C-hold", base: "DE", off: "C", dst: "DE", byteOff: true, hold: true, form: "byte-wise"},
		{name: "keepA", symBase: true, base: "BC", off: "HL", dst: "BC", live: true, keepA: true, form: "byte-wise"},
		{name: "keepA-byte", symBase: true, base: "mem", off: "C", dst: "IY", byteOff: true, keepA: true, form: "byte-wise"},
		{name: "dst-overlaps-off", base: "IY", off: "BC", dst: "BC", hold: true, form: "byte-wise"},
		{name: "dst-overlaps-base", base: "IX", off: "DE", dst: "IX", hold: true, form: "byte-wise"},
		{name: "spill-dst", symBase: true, base: "BC", off: "HL", dst: "mem", live: true},
		// A rematerialised address (no home at all): only the immediate exists.
		{name: "remat-offset-HL", symBase: true, base: "remat", off: "HL", dst: "HL", form: "constant base, offset in HL"},
		{name: "remat-offset-HL-to-DE", symBase: true, base: "remat", off: "HL", dst: "DE", form: "constant base, offset in HL"},
		{name: "remat-no-free-pair", symBase: true, base: "remat", off: "HL", dst: "HL", busy: true, form: "byte-wise"},
		{name: "remat-offset-DE", symBase: true, base: "remat", off: "DE", dst: "IY", form: "constant base"},
		{name: "remat-busy-HL", symBase: true, base: "remat", off: "IX", dst: "BC", hold: true, form: "byte-wise"},
		{name: "remat-byte", symBase: true, base: "remat", off: "B", dst: "HL", byteOff: true, keepA: true, form: "byte-wise"},
		{name: "remat-spill-dst", symBase: true, base: "remat", off: "HL", dst: "mem", live: true},
	}
	for _, c := range cases {
		for _, at := range []bool{false, true} {
			if at && !c.symBase {
				continue
			}
			t.Run(fmt.Sprintf("%s/at=%v", c.name, at), func(t *testing.T) {
				m := &Module{}
				addr := uint16(0x92FE)
				g := Global{Name: "tab", Ty: TyU8, Init: []byte{0}}
				if at {
					g.At, g.Init = &addr, nil
				}
				m.AddGlobal(g)
				f := m.AddFunc("ptr_add_form")
				b := NewBuilder(f)
				b.SwitchToNewBlock("entry")
				var base Reg
				if !c.symBase {
					base = b.Param("base", TyU16, ClassPair)
				}
				offTy := TyU16
				if c.byteOff {
					offTy = TyU8
				}
				off := b.Param("off", offTy, ClassGeneral)
				var hold, keep, busyDE, busyBC Reg
				if c.hold {
					hold = b.Param("hold", TyU16, ClassPair)
				}
				if c.busy {
					busyDE = b.Param("busy_de", TyU16, ClassPair)
					busyBC = b.Param("busy_bc", TyU16, ClassPair)
				}
				if c.keepA {
					keep = b.Param("keep", TyU8, ClassAcc)
				}
				if c.symBase {
					base = b.AddrOf("tab", ClassPointer)
				}
				p := b.PtrAdd(base, off, ClassPointer)
				vals := []Reg{p}
				f.Contract.Returns = []Return{{Ty: TyPtr, Class: ClassPointer}}
				switch {
				case c.live:
					vals = append(vals, off)
					f.Contract.Returns = append(f.Contract.Returns, Return{Ty: offTy, Class: ClassIndex})
				case c.hold:
					vals = append(vals, hold)
					f.Contract.Returns = append(f.Contract.Returns, Return{Ty: TyU16, Class: ClassIndex})
				}
				if c.busy {
					vals = append(vals, busyDE, busyBC)
					f.Contract.Returns = append(f.Contract.Returns, Return{Ty: TyU16, Class: ClassIndex}, Return{Ty: TyU16, Class: ClassIX})
				}
				if c.keepA {
					vals = append(vals, keep)
					f.Contract.Returns = append(f.Contract.Returns, Return{Ty: TyU8, Class: ClassAcc})
				}
				b.Ret(vals...)
				ar := &AllocResult{Locs: map[Reg]PhysLoc{base: locOf(c.base), off: locOf(c.off), p: locOf(c.dst)}}
				if c.hold {
					ar.Locs[hold] = locOf("HL")
				}
				if c.keepA {
					ar.Locs[keep] = locOf("A")
				}
				if c.busy {
					ar.Locs[busyDE], ar.Locs[busyBC] = locOf("DE"), locOf("BC")
				}
				if c.dst == "mem" {
					ar.Locs[p] = PhysLoc{Kind: LocMem, Name: "mem", Offset: 0xF050}
				}
				nf := newNativeFn(t, m, ar, "ptr_add_form")
				if c.form != "" && !strings.Contains(nf.asm, "ptr_add: "+c.form) {
					t.Fatalf("expected the %q form:\n%s", c.form, nf.asm)
				}
				if c.form == "" && strings.Contains(nf.asm, "ptr_add: ") {
					t.Fatalf("expected the generic path:\n%s", nf.asm)
				}
				vm := NewVM(m)
				symVM := uint16(vm.GlobalAddr("tab"))
				run := func(bv, ov uint16) emulator.Registers {
					regs := emulator.Registers{BC: 0x1111, DE: 0x2222, HL: 0x3333, IX: 0x4444, IY: 0x5555, A: 0x66}
					if !c.symBase {
						setReg(&regs, c.base, bv)
					}
					setReg(&regs, c.off, ov)
					if c.hold {
						regs.HL = 0xBEEF
					}
					if c.keepA {
						regs.A = 0x5A
					}
					if c.busy {
						regs.DE, regs.BC = 0xD1D1, 0xB2B2
					}
					return nf.run(regs)
				}
				// The native symbol address: the base itself at offset 0.
				symZ80 := uint16(0)
				if c.symBase {
					symZ80 = run(0, 0).HL
					if at && symZ80 != addr {
						t.Fatalf("At symbol at %04x, want %04x\n%s", symZ80, addr, nf.asm)
					}
				}
				offs := []uint16{}
				if c.byteOff {
					for v := 0; v < 256; v++ {
						offs = append(offs, uint16(v))
					}
				} else {
					offs = append(offs, wordBoundaries...)
					for v := 0; v < 256; v++ {
						offs = append(offs, uint16(v)<<8|uint16(v), uint16(0xFF-v))
					}
				}
				bases := []uint16{0}
				if !c.symBase {
					bases = append([]uint16{}, wordBoundaries...)
				}
				for _, bv := range bases {
					for _, ov := range offs {
						args := []Value{}
						if !c.symBase {
							args = append(args, Value{I: int64(bv)})
						}
						args = append(args, Value{I: int64(ov)})
						if c.hold {
							args = append(args, Value{I: 0xBEEF})
						}
						if c.busy {
							args = append(args, Value{I: 0xD1D1}, Value{I: 0xB2B2})
						}
						if c.keepA {
							args = append(args, Value{I: 0x5A})
						}
						want, err := vm.CallFunc(f, args)
						if err != nil {
							t.Fatal(err)
						}
						got := run(bv, ov)
						// Compare offsets from the base symbol: the VM and the
						// Z80 place the global at different addresses.
						if uint16(got.HL)-symZ80 != uint16(want[0].I)-symVM {
							t.Fatalf("base=%04x off=%04x: VM %04x (sym %04x) native %04x (sym %04x)\n%s", bv, ov, want[0].I, symVM, got.HL, symZ80, nf.asm)
						}
						if c.live && int64(got.DE) != want[1].I || c.hold && got.DE != 0xBEEF || c.keepA && got.A != 0x5A ||
							c.busy && (got.DE != 0xD1D1 || got.IX != 0xB2B2) {
							t.Fatalf("base=%04x off=%04x: live value lost: %+v VM %v\n%s", bv, ov, got, want, nf.asm)
						}
					}
				}
			})
		}
	}
}

// ptr_add(@tab, add(i, k)) with the add right before the ptr_add and used
// only there: the add emits nothing and the ptr_add adds tab+k to i.  Covers
// i live or dead afterwards (a dead i may share the result's pair), i in a
// primary pair, HL or IX, the 16-bit wrap, and constants outside 1..255 or a
// second use of the sum, which keep the add.
func TestPtrAddConstantFoldVMZ80(t *testing.T) {
	type tc struct {
		i, dst  string
		k       int64
		iLive   bool
		sumUsed bool // the sum has a second use: no fold
	}
	cases := []tc{
		{i: "DE", dst: "BC", k: 4, iLive: true}, // window_sum
		{i: "DE", dst: "DE", k: 4},
		{i: "HL", dst: "HL", k: 1},
		{i: "HL", dst: "IX", k: 255, iLive: true},
		{i: "IX", dst: "IX", k: 128},
		{i: "BC", dst: "HL", k: 7},
		{i: "DE", dst: "BC", k: 0, iLive: true},
		{i: "DE", dst: "BC", k: 256, iLive: true},
		{i: "DE", dst: "BC", k: 4, sumUsed: true},
	}
	for _, c := range cases {
		t.Run(fmt.Sprintf("%+v", c), func(t *testing.T) {
			m := &Module{}
			m.AddGlobal(Global{Name: "tab", Ty: TyU8, Init: []byte{0}})
			f := m.AddFunc("fold")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			i := b.Param("i", TyU16, ClassPair)
			base := b.AddrOf("tab", ClassPointer)
			k := b.Const(c.k, TyU16, ClassPair)
			sum := b.Add(i, k, TyU16, ClassPair)
			p := b.PtrAdd(base, sum, ClassPointer)
			vals := []Reg{p}
			f.Contract.Returns = []Return{{Ty: TyPtr, Class: ClassPointer}}
			switch {
			case c.iLive:
				vals = append(vals, i)
				f.Contract.Returns = append(f.Contract.Returns, Return{Ty: TyU16, Class: ClassIndex})
			case c.sumUsed:
				vals = append(vals, sum)
				f.Contract.Returns = append(f.Contract.Returns, Return{Ty: TyU16, Class: ClassIndex})
			}
			b.Ret(vals...)
			sumHome := "IY"
			if c.sumUsed {
				sumHome = "HL"
			}
			ar := &AllocResult{Locs: map[Reg]PhysLoc{i: locOf(c.i), base: locOf("remat"), k: locOf("mem"), sum: locOf(sumHome), p: locOf(c.dst)}}
			folded := c.k >= 1 && c.k <= 255 && !c.sumUsed
			if dead := computeDeadConsts(f, ar); dead[k] != folded || !dead[base] {
				t.Fatalf("dead constants %v: k folded=%v, address dead=%v", dead, dead[k], dead[base])
			}
			if !folded {
				ar.Locs[k] = locOf(sparePair(c.i, c.dst, sumHome))
			}
			nf := newNativeFn(t, m, ar, "fold")
			if strings.Contains(nf.asm, fmt.Sprintf("tab+%d", c.k)) != folded {
				t.Fatalf("fold=%v:\n%s", folded, nf.asm)
			}
			vm := NewVM(m)
			symVM := uint16(vm.GlobalAddr("tab"))
			// tab's native address: the result for i = -k.
			regs := emulator.Registers{BC: 0x1111, DE: 0x2222, HL: 0x3333, IX: 0x4444, IY: 0x5555, A: 0x66}
			setReg(&regs, c.i, uint16(-c.k))
			symZ80 := nf.run(regs).HL
			for _, iv := range append(append([]uint16{}, wordBoundaries...), 0x00FB, 0x00FC, 0xFFFB, 0xFFFC, 0x7F01) {
				want, err := vm.CallFunc(f, []Value{{I: int64(iv)}})
				if err != nil {
					t.Fatal(err)
				}
				regs := emulator.Registers{BC: 0x1111, DE: 0x2222, HL: 0x3333, IX: 0x4444, IY: 0x5555, A: 0x66}
				setReg(&regs, c.i, iv)
				got := nf.run(regs)
				if got.HL-symZ80 != uint16(want[0].I)-symVM || len(want) > 1 && int64(got.DE) != want[1].I {
					t.Fatalf("i=%04x: VM %v (sym %04x) native HL=%04x DE=%04x (sym %04x)\n%s", iv, want, symVM, got.HL, got.DE, symZ80, nf.asm)
				}
			}
		})
	}
}

// An address is an immediate only when every use is a ptr_add base.  A
// LocRemat promise that does not hold (the address is also dereferenced, or
// is a ptr_add offset) is repaired with a real home before emission.
func TestAddressConstantRematRepair(t *testing.T) {
	for _, asOffset := range []bool{false, true} {
		m := &Module{}
		m.AddGlobal(Global{Name: "tab", Ty: TyU8, Init: []byte{0x5C}})
		f := m.AddFunc("addr_use")
		b := NewBuilder(f)
		b.SwitchToNewBlock("entry")
		i := b.Param("i", TyU16, ClassPair)
		a := b.AddrOf("tab", ClassPointer)
		var p, v Reg
		if asOffset {
			p = b.PtrAdd(i, a, ClassPointer)
			v = b.Load(p, TyU8, ClassAcc)
		} else {
			p = b.PtrAdd(a, i, ClassPointer)
			v = b.Load(a, TyU8, ClassAcc)
		}
		b.Ret(v, p)
		f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}, {Ty: TyPtr, Class: ClassIndex}}
		if deadAddressConstants(f)[a] {
			t.Fatalf("asOffset=%v: address read out of a home was classed as an immediate", asOffset)
		}
		ar := &AllocResult{Locs: map[Reg]PhysLoc{i: locOf("HL"), a: locOf("remat"), p: locOf("DE"), v: locOf("A")}}
		nf := newNativeFn(t, m, ar, "addr_use")
		if strings.Contains(nf.asm, RematLocName) {
			t.Fatalf("remat name emitted:\n%s", nf.asm)
		}
		regs := emulator.Registers{HL: 0}
		got := nf.run(regs)
		if !asOffset && got.A != 0x5C || got.DE == 0 {
			t.Fatalf("asOffset=%v: %+v\n%s", asOffset, got, nf.asm)
		}
	}
}

// A predicate still held in flags is turned into a byte before a ptr_add.
// With a folded add the ptr_add reads the index i, which is not one of its
// operands and is dead after the add: the byte must not land in a half of i.
func TestPtrAddConstantFoldKeepsIndexAcrossPendingFlag(t *testing.T) {
	half := func(regs emulator.Registers, name string) byte {
		pair := map[byte]uint16{'B': regs.BC, 'C': regs.BC, 'D': regs.DE, 'E': regs.DE, 'H': regs.HL, 'L': regs.HL}[name[0]]
		if name == "IXL" {
			return byte(regs.IX)
		}
		if strings.ContainsAny(name, "BDH") {
			return byte(pair >> 8)
		}
		return byte(pair)
	}
	for _, iLoc := range []string{"DE", "BC", "HL"} {
		for _, cLoc := range []string{"B", "C", "D", "E", "H", "L", "IXL"} {
			if cLoc == lowByte(iLoc) || cLoc == highByte(iLoc) {
				continue
			}
			t.Run(iLoc+"/"+cLoc, func(t *testing.T) {
				m := &Module{}
				init := make([]byte, 64)
				for n := range init {
					init[n] = byte(n + 10)
				}
				m.AddGlobal(Global{Name: "tab", Ty: NewArray(TyU8, 64), Init: init})
				f := m.AddFunc("pf")
				b := NewBuilder(f)
				b.SwitchToNewBlock("entry")
				x := b.Param("x", TyU8, ClassAcc)
				y := b.Param("y", TyU8, ClassGeneral)
				i := b.Param("i", TyU16, ClassPair)
				c := b.Cmp(CmpUlt, x, y, ClassGeneral, false)
				base := b.AddrOf("tab", ClassPointer)
				k := b.Const(4, TyU16, ClassPair)
				sum := b.Add(i, k, TyU16, ClassPair)
				p := b.PtrAdd(base, sum, ClassPointer)
				v := b.Load(p, TyU8, ClassAcc)
				b.Ret(v, c)
				f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}, {Ty: TyBool, Class: ClassGeneral}}
				pLoc := "HL"
				if iLoc == "HL" {
					pLoc = "DE"
				}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{x: locOf("A"), y: locOf("IXH"), i: locOf(iLoc), c: locOf(cLoc),
					base: locOf("remat"), k: locOf("mem"), sum: locOf("IY"), p: locOf(pLoc), v: locOf("A")}}
				nf := newNativeFn(t, m, ar, "pf")
				vm := NewVM(m)
				for _, xy := range [][2]uint16{{1, 2}, {2, 1}} {
					for _, iv := range []uint16{0, 3, 17, 40} {
						want, err := vm.CallFunc(f, []Value{{I: int64(xy[0])}, {I: int64(xy[1])}, {I: int64(iv)}})
						if err != nil {
							t.Fatal(err)
						}
						regs := emulator.Registers{BC: 0x1111, DE: 0x2222, HL: 0x3333, IX: xy[1] << 8, IY: 0x5555, A: byte(xy[0])}
						setReg(&regs, iLoc, iv)
						got := nf.run(regs)
						if int64(got.A) != want[0].I || int64(half(got, cLoc)) != want[1].I {
							t.Fatalf("x=%d y=%d i=%d: VM %v, native A=%d %s=%d\n%s",
								xy[0], xy[1], iv, want, got.A, cLoc, half(got, cLoc), nf.asm)
						}
					}
				}
			})
		}
	}
}
