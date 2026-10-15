package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestConditionalCallPhysicalFlagsNative(t *testing.T) {
	for _, physical := range []string{"A", "D"} {
		for _, threshold := range []int64{0, 127} {
			for _, predicate := range []CmpCond{CmpEq, CmpNe, CmpUlt, CmpUle, CmpUgt, CmpUge} {
				for _, inverse := range []bool{false, true} {
					t.Run(fmt.Sprintf("%s/%d/%d/%v", physical, threshold, predicate, inverse), func(t *testing.T) {
						m := &Module{Globals: []Global{{Name: "observed", Ty: TyU8}}}
						f := m.AddFunc("main")
						b := NewBuilder(f)
						b.SwitchToNewBlock("entry")
						x := b.Param("x", TyU8, ClassGeneral)
						y := b.Const(threshold, TyU8, ClassGeneral)
						condition := b.CmpWithSrcTy(predicate, x, y, ClassFlag, false, TyU8)
						cc := predicate
						if inverse {
							cc = invertCmpCond(cc)
						}
						b.Cur.Insts = append(b.Cur.Insts, &Inst{Op: OpCallCond, Src: [2]Reg{condition}, Sym: "observer", Cond: cc, Ty: TyVoid})
						b.Ret()
						helper := m.AddFunc("observer")
						hb := NewBuilder(helper)
						hb.SwitchToNewBlock("entry")
						at := hb.AddrOf("observed", ClassPointer)
						value := hb.Const(42, TyU8, ClassAcc)
						hb.Store(at, value, TyU8)
						hb.Ret()
						allocation := &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocReg, Name: physical}, y: {Kind: LocReg, Name: "C"}, condition: {Kind: LocFlag, Name: "F"}, at: {Kind: LocReg, Name: "HL"}, value: {Kind: LocReg, Name: "A"}}}
						assembly, e := Z80Codegen(m, allocation, Z80CodegenOptions{CalledMain: true})
						if e != nil {
							t.Fatal(e)
						}
						a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + assembly)
						if e != nil || len(a.Errors) != 0 {
							t.Fatal(e, a.Errors)
						}
						for _, input := range []int64{0, 1, 126, 127, 128, 254, 255} {
							expected := false
							switch predicate {
							case CmpEq:
								expected = input == threshold
							case CmpNe:
								expected = input != threshold
							case CmpUlt:
								expected = input < threshold
							case CmpUle:
								expected = input <= threshold
							case CmpUgt:
								expected = input > threshold
							case CmpUge:
								expected = input >= threshold
							}
							if inverse {
								expected = !expected
							}
							z := emulator.NewRemogattoZ80WithScreen()
							z.SetROMEnd(0)
							z.SetSMCTracker(nil)
							if e = z.LoadMemory(0x8000, a.Binary); e != nil {
								t.Fatal(e)
							}
							z.Memory()[0xf000] = 3
							z.Memory()[0xf001] = 0x70
							z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: 0xf000, A: uint8(input), DE: uint16(input) << 8})
							for n := 0; z.GetPC() != 0x7003 || z.GetSP() != 0xf002; n++ {
								if n > 10000 {
									t.Fatal("no balanced return", input)
								}
								z.Step()
							}
							want := byte(0)
							if expected {
								want = 42
							}
							if got := z.Memory()[a.Symbols["observed"]]; got != want {
								t.Fatalf("input%d observed%d want%d\n%s", input, got, want, assembly)
							}
							vm := NewVM(m)
							vm.WriteHeap(vm.GlobalAddr("observed"), 0)
							if _, e = vm.Call("main", []Value{{I: input}}); e != nil {
								t.Fatal(e)
							}
							if got := vm.ReadHeap(vm.GlobalAddr("observed"), 1)[0]; got != want {
								t.Fatal("VM oracle disagreement", input, got, want)
							}
						}
					})
				}
			}
		}
	}
}
