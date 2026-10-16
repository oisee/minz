package c89_test

import (
	"bytes"
	"encoding/binary"
	"os"
	"reflect"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
)

// ISO C90 6.6.5/6.6.6: loop controls, nearest continue and labeled jumps.
// Declared objects used after a jump are initialized before every path to it.
func controlClosureModel(f string, x int) (uint16, []byte) {
	var v uint16
	var log []byte
	mark := func(x int) { log = append(log, byte(x)) }
	switch f {
	case "loops":
		for i := 0; i < x&7; i++ {
			if i == 1 {
				continue
			}
			if i == 4 {
				break
			}
			mark(i + 1)
			v = v*3 + uint16(i)
		}
		for i := 1; i <= x&3; i++ {
			mark(i + 8)
			v += uint16(i)
		}
		for i := 1; i <= max(1, x&3); i++ {
			mark(i + 16)
			if i != 1 {
				v += uint16(i)
			}
		}
	case "cleanup":
		if x&1 != 0 {
			mark(15)
			return 255, log
		}
		mark(1)
		for i := 0; i < 4 && i != x&7; i++ {
			mark(i + 2)
			v += uint16(i)
		}
		mark(9)
		v += 100
	case "multi_entry":
		left := x&1 == 0
		for i := 0; i < (x&3)+2; i++ {
			if left {
				mark(16)
				v += uint16(i + 3)
			} else {
				mark(32)
				v += uint16(2*i + 5)
			}
			left = x&(1<<(i+1)) != 0
		}
		mark(48)
	case "scope_exit":
		if x&4 != 0 {
			mark(7)
			mark(48)
			return 112, log
		}
		limit := (x & 3) + 1
		for i := 0; i < limit; i++ {
			mark(7)
			mark(i)
			v += 7
		}
		mark(48)
		v = v*16 + uint16(limit)
	case "skip_nested":
		for i := 0; i < 3; i++ {
			for j := 1; j <= 3; j++ {
				if (x+i+j)%5 == 0 {
					break
				}
				mark(i*4 + j)
				v += uint16(i + j)
			}
			v += 10
		}
	default:
		panic(f)
	}
	return v, log
}

func TestC89ControlClosureVMZ80(t *testing.T) {
	source, err := os.ReadFile("testdata/control_closure/flow.c")
	if err != nil {
		t.Fatal(err)
	}
	parse := func(t *testing.T) *hir.Module {
		t.Helper()
		m, e := c89.CompileWithOpts(string(source), "flow.c", c89.CompileOpts{NoLibc: true})
		if e != nil {
			t.Fatal(e)
		}
		if e = hir.Verify(m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	names := []string{"loops", "cleanup", "multi_entry", "scope_exit", "skip_nested"}
	inputs := []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 15, 31, 127, 128, 254, 255}
	checkMemory := func(t *testing.T, stage, name string, x int, result uint16, count byte, input byte, trace []byte, events []volatileEvent) {
		t.Helper()
		want, log := controlClosureModel(name, x)
		expected := bytes.Repeat([]byte{0xa5}, 32)
		copy(expected, log)
		var writes []volatileEvent
		for i, v := range log {
			writes = append(writes, volatileEvent{Write: true, Value: v, Offset: i})
		}
		if result != want || count != byte(len(log)) || input != byte(x) || !bytes.Equal(trace, expected) || !reflect.DeepEqual(events, writes) {
			t.Fatalf("%s %s(%d): result %d want %d count %d trace %v want %v events %v want %v", stage, name, x, result, want, count, trace, expected, events, writes)
		}
	}
	checkVM := func(t *testing.T, mm *mir2.Module) {
		t.Helper()
		vm := mir2.NewVM(mm)
		for _, name := range names {
			for _, x := range inputs {
				vm.ObserveDataAccess = nil
				vm.WriteHeapSlice(vm.GlobalAddr("trace"), bytes.Repeat([]byte{0xa5}, 32))
				vm.WriteHeapSlice(vm.GlobalAddr("input"), []byte{byte(x)})
				vm.WriteHeapSlice(vm.GlobalAddr("count"), []byte{0})
				var events []volatileEvent
				start := vm.GlobalAddr("trace")
				vm.ObserveDataAccess = func(a mir2.VMDataAccess) {
					if a.Op == mir2.OpStore && a.Addr >= start && a.Addr < start+32 {
						events = append(events, volatileEvent{Write: true, Value: byte(a.Value), Offset: int(a.Addr - start)})
					}
				}
				if _, e := vm.Call("execute_"+name, nil); e != nil {
					t.Fatal(name, x, e)
				}
				checkMemory(t, "VM", name, x, binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2)), vm.ReadHeap(vm.GlobalAddr("count"), 1)[0], vm.ReadHeap(vm.GlobalAddr("input"), 1)[0], vm.ReadHeap(start, 32), events)
			}
		}
	}
	t.Run("raw", func(t *testing.T) {
		raw := hir.LowerModule(parse(t))
		if err := mir2.Verify(raw); err != nil {
			t.Fatal(err)
		}
		checkVM(t, raw)
	})
	for _, mode := range []string{"default", "grace", "size"} {
		t.Run(mode, func(t *testing.T) {
			steps, e := pipeline.CompileHIRSteps(parse(t), pipeline.Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
			if e != nil {
				t.Fatal(e)
			}
			// The pipeline verifies optimized SSA before PreallocCoalesce. Its final
			// executable IR intentionally shares vregs between noninterfering block
			// parameters; Verify's unique-definition SSA contract no longer applies.
			// Execute that actual final IR and compare exact writes and storage.
			checkVM(t, steps.MIR2Module)
			image, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if e != nil {
				t.Fatal(e)
			}
			if len(image.Errors) > 0 {
				t.Fatal(image.Errors)
			}
			sym := func(name string) uint16 {
				p := image.Symbols[name]
				if p == 0 {
					p = image.Symbols["v_"+name]
				}
				if p == 0 {
					t.Fatal("missing symbol", name)
				}
				return uint16(p)
			}
			mem := &volatileMemory{Memory: emulator.NewMemory(), start: sym("trace"), width: 32}
			copy(mem.Data()[0x8000:], image.Binary)
			cpu := z80.NewZ80(mem, emulator.NewPorts(nil))
			for _, name := range names {
				for _, x := range inputs {
					copy(mem.Data()[sym("trace"):sym("trace")+32], bytes.Repeat([]byte{0xa5}, 32))
					mem.Data()[sym("input")] = byte(x)
					mem.Data()[sym("count")] = 0
					mem.events = nil
					mem.Data()[0xeffe] = 0
					mem.Data()[0xefff] = 0xf1
					cpu.SetPC(sym("execute_" + name))
					cpu.SetSP(0xeffe)
					for n := 0; n < 100000 && cpu.PC() != 0xf100; n++ {
						cpu.DoOpcode()
					}
					if cpu.PC() != 0xf100 || cpu.SP() != 0xf000 {
						t.Fatalf("PC/SP %s(%d)=%04x/%04x", name, x, cpu.PC(), cpu.SP())
					}
					var writes []volatileEvent
					for _, a := range mem.events {
						if a.Write {
							writes = append(writes, a)
						}
					}
					checkMemory(t, "Z80", name, x, binary.LittleEndian.Uint16(mem.Data()[sym("result"):sym("result")+2]), mem.Data()[sym("count")], mem.Data()[sym("input")], mem.Data()[sym("trace"):sym("trace")+32], writes)
				}
			}
		})
	}
}
