package mir2_test

import (
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
)

// A field-store chain introduces an implicit HL pointer. MIR liveness cannot
// protect that pointer across value computations, reversed stores, or a live
// unrelated HL value. Compare actual memory and returns against the MIR VM.
func TestZ80GlobalFieldChainNativeVM(t *testing.T) {
	for _, kind := range []string{"adjacent", "multiply_between", "reverse", "nonzero_start", "live_hl"} {
		t.Run(kind, func(t *testing.T) {
			m := &mir2.Module{Name: "field_chain"}
			ty := &mir2.StructTy{Name: "Record", Fields: []mir2.StructField{
				{Name: "first", Ty: mir2.TyU8}, {Name: "second", Ty: mir2.TyU8},
				{Name: "third", Ty: mir2.TyU8}, {Name: "guard", Ty: mir2.TyU8},
			}}
			m.AddGlobal(mir2.Global{Name: "record", Ty: ty, Init: []byte{0xa5, 0xa5, 0xa5, 0xa5}})
			f := m.AddFunc("write_fields")
			f.Contract.Returns = []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}
			b := mir2.NewBuilder(f)
			b.SwitchToNewBlock("entry")
			acc := b.Param("acc", mir2.TyU8, mir2.ClassAcc)
			x := b.Param("x", mir2.TyU8, mir2.ClassGeneral)
			locs := map[mir2.Reg]mir2.PhysLoc{acc: {Name: "A"}, x: {Name: "C"}}
			var raw mir2.Reg
			if kind == "live_hl" {
				raw = b.Param("raw", mir2.TyU16, mir2.ClassPointer)
				locs[raw] = mir2.PhysLoc{Name: "HL"}
				f.Contract.Returns = append(f.Contract.Returns, mir2.Return{Ty: mir2.TyU16, Class: mir2.ClassPointer})
			}
			var y mir2.Reg
			if kind != "multiply_between" {
				y = b.Param("y", mir2.TyU8, mir2.ClassGeneral)
				locs[y] = mir2.PhysLoc{Name: "D"}
			}
			store := func(offset int, value mir2.Reg) {
				addrLoc := "HL"
				if raw != mir2.NoReg {
					addrLoc = "IX"
				}
				addr := b.AddrOf("record", mir2.ClassPointer)
				locs[addr] = mir2.PhysLoc{Name: addrLoc}
				if offset != 0 {
					addr = b.Field(addr, int64(offset), mir2.ClassPointer)
					locs[addr] = mir2.PhysLoc{Name: addrLoc}
				}
				b.Store(addr, value, mir2.TyU8)
			}
			start := 0
			if kind == "nonzero_start" {
				start = 1
			}
			if kind == "reverse" {
				store(1, y)
				store(0, x)
			} else {
				store(start, x)
				if kind == "multiply_between" {
					eight := b.Const(8, mir2.TyU8, mir2.ClassGeneral)
					locs[eight] = mir2.PhysLoc{Name: "E"}
					y = b.Mul(acc, eight, mir2.TyU8, mir2.ClassGeneral)
					locs[y] = mir2.PhysLoc{Name: "D"}
				}
				store(start+1, y)
			}
			if raw != mir2.NoReg {
				b.Ret(acc, raw)
			} else {
				b.Ret(acc)
			}
			if err := mir2.Verify(m); err != nil {
				t.Fatal(err)
			}
			asm := mustZ80Asm(mir2.Z80Codegen(m, &mir2.AllocResult{Locs: locs}))
			for _, input := range []byte{0, 1, 0x65, 0xff} {
				args := []mir2.Value{{I: int64(input)}, {I: 0x35}}
				if raw != mir2.NoReg {
					args = append(args, mir2.Value{I: 0xdabc})
				}
				if kind != "multiply_between" {
					args = append(args, mir2.Value{I: 0x76})
				}
				vm := mir2.NewVM(m)
				result, err := vm.Call(f.Name, args)
				if err != nil || len(result) != len(f.Contract.Returns) || result[0].I != int64(input) || raw != mir2.NoReg && result[1].I != 0xdabc {
					t.Fatalf("VM input=%02x: %v %v", input, result, err)
				}
				want := []byte{0xa5, 0xa5, 0xa5, 0xa5}
				want[start], want[start+1] = 0x35, 0x76
				if kind == "multiply_between" {
					want[start+1] = input * 8
				}
				vmBytes := vm.ReadHeap(vm.GlobalAddr("record"), len(want))
				for i, v := range want {
					if vmBytes[i] != v {
						t.Fatalf("VM record[%d]=%02x want %02x", i, vmBytes[i], v)
					}
				}
				boot := fmt.Sprintf("ORG 0x8000\nLD SP,0xff00\nLD HL,0xdabc\nLD C,0x35\nLD D,0x76\nLD A,%d\nCALL write_fields\nDI\nHALT\n", input)
				image, err := z80asm.NewAssembler().AssembleString(boot + asm)
				if err != nil || len(image.Errors) != 0 {
					t.Fatalf("assemble: %v %v\n%s", err, image.Errors, asm)
				}
				platform := emulator.NewRemogattoZ80WithScreen()
				platform.SetROMEnd(0)
				z := platform.RemogattoZ80
				z.SetSMCTracker(nil)
				if err := z.LoadMemory(0x8000, image.Binary); err != nil {
					t.Fatal(err)
				}
				z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xff00})
				for steps := 0; !z.IsHalted(); steps++ {
					if steps >= 1000 {
						t.Fatalf("native did not halt\n%s", asm)
					}
					z.Step()
				}
				regs := z.GetRegisters()
				if regs.A != input || regs.SP != 0xff00 || raw != mir2.NoReg && regs.HL != uint16(result[1].I) {
					t.Fatalf("native input=%02x: A=%02x HL=%04x SP=%04x VM=%v\n%s", input, regs.A, regs.HL, regs.SP, result, asm)
				}
				address := uint16(image.Symbols["record"])
				for i, v := range want {
					if got := z.GetMemory(address + uint16(i)); got != v {
						t.Fatalf("native input=%02x record[%d]=%02x want %02x\n%s", input, i, got, v, asm)
					}
				}
			}
		})
	}
}
