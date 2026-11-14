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
				var hold, keep Reg
				if c.hold {
					hold = b.Param("hold", TyU16, ClassPair)
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
						if c.live && int64(got.DE) != want[1].I || c.hold && got.DE != 0xBEEF || c.keepA && got.A != 0x5A {
							t.Fatalf("base=%04x off=%04x: live value lost: %+v VM %v\n%s", bv, ov, got, want, nf.asm)
						}
					}
				}
			})
		}
	}
}
