package mir2

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

// A Then argument aliases the return ABI A, but must be copied only when that
// edge is taken. The original IRC prefix helper exposes this at its NUL byte.
func TestCondRetNativeSeparatesThenAndReturnCopies(t *testing.T) {
	for _, cmp := range []CmpCond{CmpEq, CmpNe, CmpUlt, CmpUle, CmpUgt, CmpUge} {
		for _, thenFT := range []bool{false, true} {
			for _, retHome := range []string{"A", "E"} {
				t.Run(fmt.Sprintf("%d/ft%v/ret%s", cmp, thenFT, retHome), func(t *testing.T) {
					m := &Module{}
					f := m.AddFunc("edge_return")
					f.Contract.Params = []Param{{Reg: 1, Ty: TyU8, Class: ClassGeneral}}
					f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
					entry := f.NewBlock("entry")
					if !thenFT {
						f.NewBlock("padding").Term = &TermUnreachable{}
					}
					yes := f.NewBlock("yes")
					yes.Params = []BlockParam{{Dst: 5, Ty: TyU8, Class: ClassAcc}}
					entry.Insts = []*Inst{{Op: OpConst, Dst: 2, Ty: TyU8, Imm: 128}, {Op: OpCmp, Dst: 3, Ty: TyBool, SrcTy: TyU8, Src: [2]Reg{1, 2}, Cond: cmp}, {Op: OpConst, Dst: 4, Ty: TyU8, Imm: 77}}
					entry.Term = &TermCondRet{Cond: 3, Vals: []Reg{4}, Then: "yes", ThenArgs: []Reg{1}}
					yes.Term = &TermRet{Vals: []Reg{5}}
					loc := func(name string) PhysLoc {
						k := LocReg
						if name == "F" {
							k = LocFlag
						}
						return PhysLoc{Kind: k, Name: name}
					}
					asm, e := Z80Codegen(m, &AllocResult{Locs: map[Reg]PhysLoc{1: loc("D"), 2: loc("C"), 3: loc("F"), 4: loc(retHome), 5: loc("A")}})
					if e != nil {
						t.Fatal(e)
					}
					bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + asm)
					if e != nil || len(bin.Errors) > 0 {
						t.Fatal(e, bin.Errors)
					}
					vm := NewVM(m)
					z := emulator.NewRemogattoZ80()
					base := bytes.Repeat([]byte{0xa5}, 65536)
					copy(base[0x8000:], bin.Binary)
					base[0xeff0] = 0
					base[0xeff1] = 0x70
					for x := 0; x < 256; x++ {
						take := false
						switch cmp {
						case CmpEq:
							take = x == 128
						case CmpNe:
							take = x != 128
						case CmpUlt:
							take = x < 128
						case CmpUle:
							take = x <= 128
						case CmpUgt:
							take = x > 128
						case CmpUge:
							take = x >= 128
						}
						want := byte(77)
						if take {
							want = byte(x)
						}
						out, e := vm.Call("edge_return", []Value{{I: int64(x)}})
						if e != nil || out[0].I != int64(want) {
							t.Fatal("VM", x, out, e, want)
						}
						copy(z.Memory(), base)
						z.SetRegisters(emulator.Registers{PC: uint16(bin.Symbols["edge_return"]), SP: 0xeff0, DE: uint16(x)<<8 | 0x33, A: 0x42, BC: 0x5555, HL: 0x7777})
						minSP := uint16(0xeff0)
						done := false
						for step := 0; step < 200; step++ {
							if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
								done = true
								break
							}
							z.Step()
							if z.GetSP() < minSP {
								minSP = z.GetSP()
							}
						}
						if !done || z.GetRegisters().A != want {
							t.Fatalf("x%d got%v want%d\n%s", x, z.GetRegisters(), want, asm)
						}
						before := append([]byte(nil), base...)
						copy(before[minSP:0xeff0], z.Memory()[minSP:0xeff0])
						if !bytes.Equal(before, z.Memory()) {
							t.Fatal("RAM/image", x)
						}
					}
				})
			}
		}
	}
}
