package mir2

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// Consume a carry-derived source predicate after the multiply. Native execution
// must preserve source data, not inherit either input carry or mul's SUB flags.
func TestMulLivePredicateNative(t *testing.T) {
	for _, coeff := range []int{3, 11, 17} {
		t.Run(fmt.Sprintf("coeff%d", coeff), func(t *testing.T) {
			m := &Module{}
			f := m.AddFunc("carry_live")
			b := NewBuilder(f)
			b.SwitchToNewBlock("entry")
			x := b.Param("x", TyU8, ClassAcc)
			y := b.Param("y", TyU8, ClassGeneral)
			pred := b.Cmp(CmpLt, x, y, ClassGeneral, false)
			result := b.Mul(x, b.Const(int64(coeff), TyU8, ClassGeneral), TyU8, ClassAcc)
			out := b.Add(result, b.Ext(pred, TyBool, TyU8, ClassGeneral), TyU8, ClassAcc)
			b.Ret(out)
			f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
			ar := PBQPAllocate(f, ComputeLiveness(f), Z80CostTable{})
			asm, e := Z80Codegen(m, ar)
			if e != nil {
				t.Fatal(e)
			}
			bin, e := z80asm.NewAssembler().AssembleString("ORG 32768\n" + asm)
			if e != nil || len(bin.Errors) > 0 {
				t.Fatal(e, bin.Errors)
			}
			for x := 0; x < 256; x++ {
				for _, y := range []int{0, 1, 127, 128, 254, 255} {
					for _, flags := range []byte{0, 1, 0x40, 0xff} {
						z := emulator.NewRemogattoZ80()
						z.LoadMemory(0x8000, bin.Binary)
						z.LoadMemory(0xf100, []byte{0xcd, 0, 0x80, 0xf3, 0x76})
						r := emulator.Registers{PC: 0xf100, SP: 0xf000, F: flags}
						set := func(name string, v byte) {
							switch name {
							case "A":
								r.A = v
							case "B":
								r.BC = (r.BC & 255) | uint16(v)<<8
							case "C":
								r.BC = (r.BC & 0xff00) | uint16(v)
							case "D":
								r.DE = (r.DE & 255) | uint16(v)<<8
							case "E":
								r.DE = (r.DE & 0xff00) | uint16(v)
							case "H":
								r.HL = (r.HL & 255) | uint16(v)<<8
							case "L":
								r.HL = (r.HL & 0xff00) | uint16(v)
							default:
								t.Fatal("unexpected fixture parameter", name)
							}
						}
						set(ar.Locs[f.Contract.Params[0].Reg].Name, byte(x))
						set(ar.Locs[f.Contract.Params[1].Reg].Name, byte(y))
						z.SetRegisters(r)
						z.MaxCycles = 1000
						if e := z.Run(); e != nil {
							t.Fatal(e)
						}
						want := byte(x * coeff)
						if x < y {
							want++
						}
						got := z.GetRegisters()
						if got.A != want || got.SP != 0xf000 {
							t.Fatal(fmt.Sprintf("x%d y%d F%02x got%02x want%02x SP%04x\n%s", x, y, flags, got.A, want, got.SP, asm))
						}
						if !bytes.Equal(z.Memory()[0x8000:0x8000+len(bin.Binary)], bin.Binary) {
							t.Fatal("image mutation")
						}
					}
				}
			}
		})
	}
}
