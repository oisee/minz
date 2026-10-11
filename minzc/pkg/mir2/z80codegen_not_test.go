package mir2

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestZ80NotAllocatedByteAndBool(t *testing.T) {
	for _, ty := range []Ty{TyBool, TyU8} {
		for _, srcHome := range []string{"A", "B", "C", "D", "E", "H", "L", "IXH", "IXL", "IYH", "IYL", "spill"} {
			for _, dstHome := range []string{"A", "C", "D", "IXH", "IYL", "spill"} {
				for _, value := range []int64{0, 1, 127, 128, 255} {
					if ty == TyBool && value > 1 {
						continue
					}
					t.Run(fmt.Sprintf("%s/%s/%s/%d", ty, srcHome, dstHome, value), func(t *testing.T) {
						m := &Module{}
						f := m.AddFunc("invert")
						f.Contract.Returns = []Return{{Ty: ty, Class: ClassAcc}}
						b := NewBuilder(f)
						b.SwitchToNewBlock("entry")
						input := b.Param("x", ty, ClassGeneral)
						out := b.Not(input, ty, ClassGeneral)
						b.Ret(out)
						ar := &AllocResult{Locs: map[Reg]PhysLoc{input: {Kind: LocReg, Name: srcHome}, out: {Kind: LocReg, Name: dstHome}}}
						if srcHome == "spill" {
							ar.Locs[input] = PhysLoc{Kind: LocMem}
						}
						if dstHome == "spill" {
							ar.Locs[out] = PhysLoc{Kind: LocMem}
						}
						code, err := Z80Codegen(m, ar)
						if err != nil {
							t.Fatal(err)
						}
						init := fmt.Sprintf("LD %s,%d\n", srcHome, value)
						if srcHome == "spill" {
							init = fmt.Sprintf("LD A,%d\nLD (%s),A\n", value, Z80SpillLabel("invert", input))
						}
						asm := "ORG 0x8000\nLD SP,0xfff0\n" + init + "CALL invert\nDI\nHALT\n" + code
						res, err := z80asm.NewAssembler().AssembleString(asm)
						if err != nil || len(res.Errors) > 0 {
							t.Fatalf("assemble %v %v\n%s", err, res.Errors, asm)
						}
						z := emulator.NewRemogattoZ80()
						z.LoadMemory(0x8000, res.Binary)
						z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xfff0})
						for i := 0; !z.IsHalted(); i++ {
							if i > 10000 {
								t.Fatal("no halt")
							}
							z.Step()
						}
						mask := int64(255)
						if ty == TyBool {
							mask = 1
						}
						want := (^value) & mask
						got := int64(z.GetRegisters().A)
						vm, e := NewVM(m).Call("invert", []Value{{I: value}})
						if got != want || e != nil || len(vm) != 1 || vm[0].I != want || z.GetSP() != 0xfff0 {
							t.Fatalf("Z80=%d VM=%v %v want=%d SP=%04x\n%s", got, vm, e, want, z.GetSP(), asm)
						}
					})
				}
			}
		}
	}
}

func TestZ80NotWideAllocatedAndSpill(t *testing.T) {
	for _, ty := range []Ty{TyU16, TyU24, TyU32} {
		for _, srcHome := range []string{"HL", "DE", "BC", "spill"} {
			for _, dstHome := range []string{"HL", "DE", "BC", "spill"} {
				for _, value := range []uint32{0, 1, 0x12345678, 0x80000000, 0xffffffff} {
					t.Run(fmt.Sprintf("%s/%s/%s/%08x", ty, srcHome, dstHome, value), func(t *testing.T) {
						m := &Module{}
						f := m.AddFunc("invert")
						class := ClassDWord
						if ty.Width() == 16 {
							class = ClassPair
						}
						f.Contract.Returns = []Return{{Ty: ty, Class: class}}
						b := NewBuilder(f)
						b.SwitchToNewBlock("entry")
						input := b.Const(int64(value), ty, class)
						out := b.Not(input, ty, class)
						b.Ret(out)
						ar := &AllocResult{Locs: map[Reg]PhysLoc{input: {Kind: LocDWord, Name: srcHome}, out: {Kind: LocDWord, Name: dstHome}}}
						if ty.Width() == 16 {
							ar.Locs[input] = PhysLoc{Kind: LocReg, Name: srcHome}
							ar.Locs[out] = PhysLoc{Kind: LocReg, Name: dstHome}
						}
						if srcHome == "spill" {
							ar.Locs[input] = PhysLoc{Kind: LocMem}
						}
						if dstHome == "spill" {
							ar.Locs[out] = PhysLoc{Kind: LocMem}
						}
						code, err := Z80Codegen(m, ar)
						if err != nil {
							t.Fatal(err)
						}
						asm := "ORG 0x8000\nLD SP,0xfff0\nCALL invert\nLD (0x6100),HL\nEXX\nLD (0x6102),HL\nEXX\nDI\nHALT\n" + code
						res, err := z80asm.NewAssembler().AssembleString(asm)
						if err != nil || len(res.Errors) > 0 {
							t.Fatalf("assemble %v %v\n%s", err, res.Errors, asm)
						}
						z := emulator.NewRemogattoZ80()
						z.LoadMemory(0x8000, res.Binary)
						z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xfff0})
						for i := 0; !z.IsHalted(); i++ {
							if i > 10000 {
								t.Fatal("no halt")
							}
							z.Step()
						}
						mask := uint32((uint64(1) << ty.Width()) - 1)
						want := (^value) & mask
						data := z.Memory()[0x6100:0x6104]
						got := (uint32(data[0]) | uint32(data[1])<<8 | uint32(data[2])<<16 | uint32(data[3])<<24) & mask
						vm, e := NewVM(m).Call("invert", nil)
						if got != want || e != nil || len(vm) != 1 || uint32(vm[0].I) != want || z.GetSP() != 0xfff0 {
							t.Fatalf("Z80=%08x VM=%v %v want=%08x SP=%04x\n%s", got, vm, e, want, z.GetSP(), asm)
						}
					})
				}
			}
		}
	}
}

func TestZ80NotPreservesGenuinelyLiveAccumulator(t *testing.T) {
	for _, ty := range []Ty{TyBool, TyU8, TyU16, TyU24, TyU32} {
		for _, input := range []int64{0, 1, 127, 255} {
			if ty == TyBool && input > 1 {
				continue
			}
			t.Run(fmt.Sprintf("%s/%d", ty, input), func(t *testing.T) {
				m := &Module{}
				f := m.AddFunc("invert_live")
				f.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
				b := NewBuilder(f)
				b.SwitchToNewBlock("entry")
				keep := b.Param("keep", TyU8, ClassAcc)
				x := b.Param("x", ty, ClassGeneral)
				inv := b.Not(x, ty, ClassGeneral)
				byteInv := inv
				if ty.Width() > 8 {
					byteInv = b.Trunc(inv, ty, TyU8, ClassGeneral)
				}
				result := b.Add(keep, byteInv, TyU8, ClassAcc)
				b.Ret(result)
				kind, home := LocReg, "C"
				outHome := "D"
				if ty.Width() > 8 {
					home = "HL"
					outHome = "DE"
				}
				if ty.Width() > 16 {
					kind = LocDWord
				}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{keep: {Kind: LocReg, Name: "A"}, x: {Kind: kind, Name: home}, inv: {Kind: kind, Name: outHome}, byteInv: {Kind: LocReg, Name: "D"}, result: {Kind: LocReg, Name: "A"}}}
				ar.Locs[inv] = PhysLoc{Kind: kind, Name: outHome}
				code, err := Z80Codegen(m, ar)
				if err != nil {
					t.Fatal(err)
				}
				init := fmt.Sprintf("LD %s,%d\n", home, input)
				if ty.Width() > 16 {
					init += "EXX\nLD HL,0\nEXX\n"
				}
				asm := "ORG 0x8000\nLD SP,0xfff0\n" + init + "LD A,37\nCALL invert_live\nDI\nHALT\n" + code
				res, err := z80asm.NewAssembler().AssembleString(asm)
				if err != nil || len(res.Errors) > 0 {
					t.Fatalf("assemble %v %v\n%s", err, res.Errors, asm)
				}
				z := emulator.NewRemogattoZ80()
				z.LoadMemory(0x8000, res.Binary)
				z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xfff0})
				for i := 0; !z.IsHalted(); i++ {
					if i > 10000 {
						t.Fatal("nohalt")
					}
					z.Step()
				}
				mask := int64(255)
				if ty == TyBool {
					mask = 1
				}
				want := (37 + ((^input) & mask)) & 255
				vm, e := NewVM(m).Call("invert_live", []Value{{I: 37}, {I: input}})
				if int64(z.GetRegisters().A) != want || e != nil || len(vm) != 1 || vm[0].I != want || z.GetSP() != 0xfff0 {
					t.Fatalf("live-A Z80=%d VM=%v %v want=%d SP=%04x\n%s", z.GetRegisters().A, vm, e, want, z.GetSP(), asm)
				}
			})
		}
	}
}
