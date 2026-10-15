package mir2

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

// Promoted byte operands occupy one byte of spill memory, even when the
// multiply produces a word. Poison adjacent bytes to detect widened reads.
func TestMul16PromotedByteSpillNative(t *testing.T) {
	cases := 0
	for _, ty := range []Ty{TyU8, TyI8} {
		for _, factor := range []int{2, 3, -1} { // -1 selects the runtime multiplier path.
			m := &Module{}
			f := m.AddFunc("multiply_bytes")
			b := f.NewBlock("entry")
			x, y, out := f.AllocReg(), f.AllocReg(), f.AllocReg()
			var rhsTy Ty = TyU16
			if factor < 0 {
				rhsTy = ty
			}
			f.Contract.Params = []Param{{Name: "x", Reg: x, Ty: ty, Class: ClassGeneral}, {Name: "y", Reg: y, Ty: rhsTy, Class: ClassGeneral}}
			inst := &Inst{Op: OpMul, Dst: out, Src: [2]Reg{x, y}, Ty: TyU16, Cls: ClassPointer}
			b.Append(inst)
			b.Seal(&TermRet{Vals: []Reg{out}})
			ar := &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocMem, Offset: 0xa100}, y: {Kind: LocMem, Offset: 0xa110}, out: {Kind: LocReg, Name: "HL"}}}
			var body strings.Builder
			g := &z80cg{mod: m, fn: f, ar: ar, sb: &body, curBlock: b, liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), constVals: map[Reg]int64{}, holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, pendingAccReg: NoReg, pendingFlagReg: NoReg}
			if factor >= 0 {
				g.constVals[y] = int64(factor)
			}
			g.genMul16(inst)
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\nmultiply_bytes:\n" + body.String() + "\nRET\n" + g.spillLabel(x) + " EQU $A100\n" + g.spillLabel(y) + " EQU $A110\n")
			if e != nil {
				t.Fatal(e, body.String())
			}
			if len(a.Errors) > 0 {
				t.Fatal(a.Errors, body.String())
			}
			for _, poison := range []byte{0, 1, 128, 255} {
				for value := 0; value < 256; value++ {
					xInt, yInt := value, factor
					rhsByte := byte(factor)
					if ty == TyI8 {
						xInt = int(int8(byte(value)))
					}
					if factor < 0 {
						rhsByte = 239
						yInt = 239
						if ty == TyI8 {
							yInt = -17
						}
					}
					want := uint16(xInt * yInt)
					if ty == TyU8 {
						vm, e := NewVM(m).Call(f.Name, []Value{{I: int64(xInt)}, {I: int64(yInt)}})
						if e != nil || len(vm) != 1 || uint16(vm[0].I) != want {
							t.Fatalf("VM %v %v want%x", vm, e, want)
						}
					}
					z := emulator.NewRemogattoZ80()
					if e := z.LoadMemory(0x8000, a.Binary); e != nil {
						t.Fatal(e)
					}
					ram := z.Memory()
					ram[0xa100] = byte(value)
					ram[0xa101] = poison
					ram[0xa110] = rhsByte
					ram[0xa111] = poison
					ram[0xff00], ram[0xff01] = 0, 0x70
					before := emulator.Registers{PC: 0x8000, SP: 0xff00, HL: 0xabcd, BC: 0x1234, DE: 0x5678, IX: 0x9abc, IY: 0xcdef, A: 0x37, F: 0xa5}
					z.SetRegisters(before)
					for n := 0; (z.GetPC() != 0x7000 || z.GetSP() != 0xff02) && n < 2000; n++ {
						z.Step()
					}
					got := z.GetRegisters()
					if got.HL != want || got.A != before.A || got.BC != before.BC || got.DE != before.DE || got.IX != before.IX || got.IY != before.IY || got.PC != 0x7000 || got.SP != 0xff02 || ram[0xa100] != byte(value) || ram[0xa101] != poison || ram[0xa110] != rhsByte || ram[0xa111] != poison {
						t.Fatalf("type%v factor%d value%d poison%d got%+v want%x\n%s", ty, factor, value, poison, got, want, body.String())
					}
					cases++
				}
			}
		}
	}
	t.Logf("%d native executions, %d unsigned VM executions; signed native promotion uses the host oracle; poisoned neighbours and PC/SP checked", cases, cases/2)
}
