package pipeline

import (
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestDWordCycleOrdinarySource(t *testing.T) {
	const src = `global ina:u32 global inb:u32 global count:u8 global outx:u32 global outy:u32
fun swap(){var x:u32=ina var y:u32=inb var i:u8=0 while i<count {let old:u32=x x=y y=old i=i+1} outx=x outy=y}`
	parse := func() *hir.Module {
		h, e := nanz.Parse(src, "dword-cycle.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	raw := mir2.NewVM(hir.LowerModule(parse()))
	s, e := CompileHIRSteps(parse(), Options{ContractOpt: true, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	final := mir2.NewVM(s.MIR2Module)
	bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if e != nil {
		t.Fatal(e)
	}
	if len(bin.Errors) > 0 {
		t.Fatal(bin.Errors)
	}
	z := emulator.NewRemogattoZ80()
	z.LoadMemory(0x8000, bin.Binary)
	z.LoadMemory(0xeffe, []byte{0, 0xf1})
	values := []uint32{0, 1, 0xffff, 0x10000, 0x80000000, 0xffffffff, 0x12345678, 0xabcdef01}
	for _, a := range values {
		for _, b := range values {
			for _, n := range []uint8{0, 1, 2, 3, 7, 8, 254, 255} {
				x, y := a, b
				if n&1 != 0 {
					x, y = b, a
				}
				for _, vm := range []*mir2.VM{raw, final} {
					buf := make([]byte, 4)
					binary.LittleEndian.PutUint32(buf, a)
					vm.WriteHeapSlice(vm.GlobalAddr("ina"), buf)
					binary.LittleEndian.PutUint32(buf, b)
					vm.WriteHeapSlice(vm.GlobalAddr("inb"), buf)
					vm.WriteHeapSlice(vm.GlobalAddr("count"), []byte{n})
					if _, e := vm.Call("swap", nil); e != nil {
						t.Fatal(e)
					}
					if binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("outx"), 4)) != x || binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("outy"), 4)) != y {
						t.Fatal("VM mismatch")
					}
				}
				binary.LittleEndian.PutUint32(z.Memory()[bin.Symbols["ina"]:bin.Symbols["ina"]+4], a)
				binary.LittleEndian.PutUint32(z.Memory()[bin.Symbols["inb"]:bin.Symbols["inb"]+4], b)
				z.Memory()[bin.Symbols["count"]] = n
				z.SetRegisters(emulator.Registers{PC: uint16(bin.Symbols["swap"]), SP: 0xeffe, A: 0x5a, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
				for i := 0; i < 100000 && z.GetPC() != 0xf100; i++ {
					z.Step()
				}
				gotx := binary.LittleEndian.Uint32(z.Memory()[bin.Symbols["outx"] : bin.Symbols["outx"]+4])
				goty := binary.LittleEndian.Uint32(z.Memory()[bin.Symbols["outy"] : bin.Symbols["outy"]+4])
				if gotx != x || goty != y || z.GetPC() != 0xf100 || z.GetSP() != 0xf000 {
					t.Fatalf("a=%08x b=%08x n=%d got=%08x/%08x want=%08x/%08x\n%s", a, b, n, gotx, goty, x, y, s.Assembly)
				}
			}
		}
	}
	for i, v := range bin.Binary {
		addr := bin.Origin + i
		mutable := false
		for _, name := range []string{"ina", "inb", "count", "outx", "outy"} {
			size := 4
			if name == "count" {
				size = 1
			}
			start := bin.Symbols[name]
			if addr >= start && addr < start+size {
				mutable = true
			}
		}
		for name, start := range bin.Symbols {
			if strings.HasPrefix(name, "_spill_swap_r") && addr >= start && addr < start+4 {
				mutable = true
			}
		}
		if !mutable && z.Memory()[addr] != v {
			t.Fatalf("code mutation at %04x", addr)
		}
	}
	if dir := os.Getenv("DWORD_CYCLE_EXPORT"); dir != "" {
		os.MkdirAll(dir, 0755)
		os.WriteFile(dir+"/swap.nanz", []byte(src), 0644)
		os.WriteFile(dir+"/swap.a80", []byte(s.Assembly), 0644)
		os.WriteFile(dir+"/swap.bin", bin.Binary, 0644)
	}
}
