package c89_test

import (
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// ISO C90 6.3.2.2 and 6.2.1.2: unsigned long arguments/results retain 32
// target bits. Caller-live operands survive CALL; arithmetic wraps modulo 2^32.
func TestC89WideCallLiveVMZ80(t *testing.T) {
	src, e := os.ReadFile(filepath.Join("testdata/call_abi", "wide.c"))
	if e != nil {
		t.Fatal(e)
	}
	parse := func(t *testing.T) *hir.Module {
		t.Helper()
		m, e := c89.CompileWithOpts(string(src), "wide.c", c89.CompileOpts{NoLibc: true})
		if e != nil {
			t.Fatal(e)
		}
		if e = hir.Verify(m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	values := []uint32{0, 1, 2, 127, 128, 255, 256, 32767, 32768, 65535, 65536, 0x12345678, 0x80000000, 0xfffffffe, 0xffffffff}
	step := func(x uint32) uint32 { return (x << 7) ^ (x >> 3) ^ 0x12345678 }
	model := func(f string, x, y uint32) uint32 {
		switch f {
		case "joined":
			return step(x) + y
		case "live":
			return (step(x) + y) ^ x
		case "forwarded":
			return ((step(y) + x) ^ y) + (step(x) + y)
		}
		panic(f)
	}
	checkVM := func(t *testing.T, m *mir2.Module) {
		t.Helper()
		if e := mir2.Verify(m); e != nil {
			t.Fatal(e)
		}
		vm := mir2.NewVM(m)
		for _, f := range []string{"joined", "live", "forwarded"} {
			for _, x := range values {
				for _, y := range values {
					var data [4]byte
					binary.LittleEndian.PutUint32(data[:], x)
					vm.WriteHeapSlice(vm.GlobalAddr("a"), data[:])
					binary.LittleEndian.PutUint32(data[:], y)
					vm.WriteHeapSlice(vm.GlobalAddr("b"), data[:])
					if _, e := vm.Call("execute_"+f, nil); e != nil {
						t.Fatal(f, e)
					}
					got := binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("result"), 4))
					if got != model(f, x, y) {
						t.Fatalf("VM %s(%08x,%08x)=%08x want=%08x", f, x, y, got, model(f, x, y))
					}
					if binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("a"), 4)) != x || binary.LittleEndian.Uint32(vm.ReadHeap(vm.GlobalAddr("b"), 4)) != y {
						t.Fatal("call overwrote input globals")
					}
				}
			}
		}
	}
	t.Run("raw", func(t *testing.T) { checkVM(t, hir.LowerModule(parse(t))) })
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			s, e := pipeline.CompileHIRSteps(parse(t), pipeline.Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if e != nil {
				t.Fatal(e)
			}
			checkVM(t, s.MIR2Module)
			for _, name := range []string{"joined", "live", "forwarded"} {
				f := s.MIR2Module.FuncByName(name)
				if f == nil || len(f.Contract.Params) != 2 || len(f.Contract.Returns) != 1 || f.Contract.Params[0].Ty != mir2.TyU32 || f.Contract.Params[1].Ty != mir2.TyU32 || f.Contract.Returns[0].Ty != mir2.TyU32 {
					t.Fatal("wide source ABI narrowed", name)
				}
			}
			bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil {
				t.Fatal(e)
			}
			if len(bin.Errors) > 0 {
				t.Fatal(bin.Errors)
			}
			sym := func(name string) int {
				p := bin.Symbols[name]
				if p == 0 {
					p = bin.Symbols["v_"+name]
				}
				if p == 0 {
					t.Fatal("missing symbol", name)
				}
				return p
			}
			z := emulator.NewRemogattoZ80()
			if e := z.LoadMemory(0x8000, bin.Binary); e != nil {
				t.Fatal(e)
			}
			for _, f := range []string{"joined", "live", "forwarded"} {
				for _, x := range values {
					for _, y := range values {
						binary.LittleEndian.PutUint32(z.Memory()[sym("a"):sym("a")+4], x)
						binary.LittleEndian.PutUint32(z.Memory()[sym("b"):sym("b")+4], y)
						z.Memory()[0xeffe] = 0
						z.Memory()[0xefff] = 0xf1
						z.SetRegisters(emulator.Registers{PC: uint16(sym("execute_" + f)), SP: 0xeffe})
						for n := 0; n < 100000 && z.GetRegisters().PC != 0xf100; n++ {
							z.Step()
						}
						r := z.GetRegisters()
						got := binary.LittleEndian.Uint32(z.Memory()[sym("result") : sym("result")+4])
						if got != model(f, x, y) || r.PC != 0xf100 || r.SP != 0xf000 {
							t.Fatal(fmt.Sprintf("Z80 %s(%08x,%08x)=%08x want=%08x PC=%04x SP=%04x", f, x, y, got, model(f, x, y), r.PC, r.SP), s.Assembly)
						}
						if binary.LittleEndian.Uint32(z.Memory()[sym("a"):sym("a")+4]) != x || binary.LittleEndian.Uint32(z.Memory()[sym("b"):sym("b")+4]) != y {
							t.Fatal("call overwrote input globals")
						}
					}
				}
			}
		})
	}
}
