package mir2_test

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestZ80ByteStorePreservesLiveAccumulator(t *testing.T) {
	for _, tc := range []struct {
		ptr, val string
		constant bool
	}{
		{"BC", "IXL", false}, {"DE", "IXL", false}, {"BC", "C", false}, {"DE", "E", false},
		{"IX", "H", false}, {"IY", "L", false}, {"IX", "IYL", false}, {"IY", "IXL", false},
		{"BC", "C", true}, {"DE", "E", true}, {"HL", "IXL", false}, {"IX", "B", false},
	} {
		for _, acc := range []byte{0, 1, 0xFE} {
			t.Run(fmt.Sprintf("%s/%s/constant%v/A%d", tc.ptr, tc.val, tc.constant, acc), func(t *testing.T) {
				m := &mir2.Module{Name: "store_live_acc"}
				f := m.AddFunc("store_step")
				f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
				b := mir2.NewBuilder(f)
				b.SwitchToNewBlock("entry")
				ptr := b.Param("ptr", mir2.TyPtr, mir2.ClassPointer)
				a := b.Param("acc", mir2.TyU8, mir2.ClassAcc)
				var val mir2.Reg
				if tc.constant {
					val = b.Const(0x35, mir2.TyU8, mir2.ClassGeneral)
				} else {
					val = b.Param("val", mir2.TyU8, mir2.ClassGeneral)
				}
				b.Store(ptr, val, mir2.TyU8)
				one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
				next := b.Add(a, one, mir2.TyU8, mir2.ClassAcc)
				b.Ret(next)
				scratch := "B"
				if tc.ptr == "BC" || tc.val == "B" {
					scratch = "D"
				}
				if tc.ptr == "DE" {
					scratch = "B"
				}
				ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{ptr: {Kind: mir2.LocReg, Name: tc.ptr}, a: {Kind: mir2.LocReg, Name: "A"}, val: {Kind: mir2.LocReg, Name: tc.val}, one: {Kind: mir2.LocReg, Name: scratch}, next: {Kind: mir2.LocReg, Name: "A"}}}
				setup := ""
				if !tc.constant {
					setup = fmt.Sprintf("LD %s,53\n", tc.val)
				}
				// Low-byte value may alias the pointer; set it before the pair, then
				// derive the actual input value from the pointer when applicable.
				addr := uint16(0xC03D)
				wantByte := byte(53)
				if tc.val == "C" && tc.ptr == "BC" || tc.val == "E" && tc.ptr == "DE" {
					wantByte = byte(addr)
				}
				if tc.constant {
					wantByte = 53
				}
				asm := fmt.Sprintf("ORG 0x8000\nLD SP,0xFF00\n%sLD %s,%d\nLD A,%d\nCALL store_step\nDI\nHALT\n", setup, tc.ptr, addr, acc) + mustZ80Asm(mir2.Z80Codegen(m, ar))
				res, err := z80asm.NewAssembler().AssembleString(asm)
				if err != nil || len(res.Errors) > 0 {
					t.Fatalf("assemble %v %v\n%s", err, res.Errors, asm)
				}
				z := emulator.NewRemogattoZ80()
				z.Reset()
				if e := z.LoadMemory(0x8000, res.Binary); e != nil {
					t.Fatal(e)
				}
				z.SetMemory(addr, 0xA5)
				z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
				for steps := 0; !z.IsHalted(); steps++ {
					if steps > 1000 {
						t.Fatalf("no halt\n%s", asm)
					}
					z.Step()
				}
				r := z.GetRegisters()
				if r.A != acc+1 || z.GetMemory(addr) != wantByte || r.SP != 0xFF00 {
					t.Fatalf("A=%d mem=%d SP=%04x want A=%d mem=%d\n%s", r.A, z.GetMemory(addr), r.SP, acc+1, wantByte, asm)
				}
			})
		}
	}
}

func TestZ80PointerAddStoreKeepsBaseAndAccumulator(t *testing.T) {
	m := &mir2.Module{Name: "ptr_store"}
	f := m.AddFunc("ptr_store")
	f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}, {Ty: mir2.TyPtr, Class: mir2.ClassPointer}}
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	base := b.Param("base", mir2.TyPtr, mir2.ClassPointer)
	acc := b.Param("acc", mir2.TyU8, mir2.ClassAcc)
	value := b.Param("value", mir2.TyU8, mir2.ClassGeneral)
	one := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	ptr := b.PtrAdd(base, acc, mir2.ClassPointer)
	b.Store(ptr, value, mir2.TyU8)
	next := b.Add(acc, one, mir2.TyU8, mir2.ClassAcc)
	b.Ret(next, base)
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{base: {Kind: mir2.LocReg, Name: "HL"}, acc: {Kind: mir2.LocReg, Name: "A"}, value: {Kind: mir2.LocIXY8, Name: "IXL"}, one: {Kind: mir2.LocReg, Name: "B"}, ptr: {Kind: mir2.LocReg, Name: "DE"}, next: {Kind: mir2.LocReg, Name: "A"}}}
	asm := "ORG 0x8000\nLD SP,0xFF00\nLD HL,49152\nLD A,7\nLD IXL,53\nCALL ptr_store\nDI\nHALT\n" + mustZ80Asm(mir2.Z80Codegen(m, ar))
	res, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil || len(res.Errors) != 0 {
		t.Fatalf("assemble %v %v\n%s", err, res.Errors, asm)
	}
	z := emulator.NewRemogattoZ80()
	z.Reset()
	z.LoadMemory(0x8000, res.Binary)
	z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xFF00})
	for steps := 0; !z.IsHalted(); steps++ {
		if steps > 1000 {
			t.Fatal("no halt")
		}
		z.Step()
	}
	r := z.GetRegisters()
	if r.A != 8 || r.HL != 49152 || r.SP != 0xFF00 || z.GetMemory(49159) != 53 {
		t.Fatalf("A=%d HL=%04x SP=%04x mem=%d\n%s", r.A, r.HL, r.SP, z.GetMemory(49159), asm)
	}
}
