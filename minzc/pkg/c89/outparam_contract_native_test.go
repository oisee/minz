package c89_test

import (
	"encoding/binary"
	"encoding/json"
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

// Compare against a correct, caller-owned memory return ABI. Disabling only
// C-specific scalar eligibility does not revert the ownership repair.
func TestC89OutParamContractVMZ80(t *testing.T) {
	for _, family := range []string{"local_pair", "repeated_pair", "mixed_pair"} {
		t.Run(family, func(t *testing.T) {
			src, e := os.ReadFile(filepath.Join("testdata/outparam", family+".c"))
			if e != nil {
				t.Fatal(e)
			}
			inputs := [][2]byte{}
			for x := 0; x < 256; x++ {
				for _, y := range []byte{0, 1, 2, 63, 127, 128, 254, 255} {
					inputs = append(inputs, [2]byte{byte(x), y})
				}
			}
			if family == "repeated_pair" {
				inputs = nil
				state := uint32(0x51de)
				for i := 0; i < 73; i++ {
					state = 1664525*state + 1013904223
					x := byte(state >> 16)
					state = 1664525*state + 1013904223
					inputs = append(inputs, [2]byte{x, byte(state >> 16)})
				}
			}
			want := func(x, y byte) uint16 {
				if family == "mixed_pair" {
					return uint16(int16(x)-int16(y)) ^ uint16(x^y)
				}
				if family == "repeated_pair" {
					y++
					return uint16(byte(x+y))<<8 | uint16(y^0xa5)
				}
				hi := byte(x + y)
				x += 3
				lo := byte(x + 7)
				if x >= 128 {
					lo = x ^ y
				}
				return uint16(hi)<<8 | uint16(lo)
			}
			wantEvent := byte(16)
			if family == "repeated_pair" {
				wantEvent = 20
			}
			for _, mode := range []string{"default", "grace", "size"} {
				t.Run(mode, func(t *testing.T) {
					rows := []map[string]any{}
					for _, abi := range []string{"memory", "scalar"} {
						m, e := c89.CompileWithOpts(string(src), family+".c", c89.CompileOpts{NoLibc: true, DeferAggregateABI: true})
						if e != nil {
							t.Fatal(e)
						}
						if abi == "memory" {
							for _, f := range m.Funcs {
								f.CValueABI = false
							}
						} else {
							hir.OptimizeCAggregateABI(m)
						}
						raw := hir.LowerModule(m)
						if e := mir2.Verify(raw); e != nil {
							t.Fatal(e)
						}
						s, e := pipeline.CompileHIRSteps(m, pipeline.Options{ContractOpt: true, AssertMode: "none", UseGrace: mode != "default", UseHIRGrace: mode != "default", OptSize: mode == "size"})
						if e != nil {
							t.Fatal(e)
						}
						fill := s.MIR2Module.FuncByName("fill")
						if abi == "scalar" {
							if len(fill.Contract.Returns) != 2 || len(fill.Contract.Params) != 2 {
								t.Fatal("out-param scalar ABI not selected")
							}
						} else {
							if len(fill.Contract.Params) != 3 || len(fill.Contract.Returns) != 0 {
								t.Fatal("out-param memory ABI not retained")
							}
						}
						vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
						bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
						if e != nil {
							t.Fatal(e)
						}
						if len(bin.Errors) > 0 {
							t.Fatal(bin.Errors)
						}
						symbol := func(name string) int {
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
						total := 0
						observations := []map[string]any{}
						for _, xy := range inputs {
							x, y := xy[0], xy[1]
							expected := want(x, y)
							for _, vm := range vms {
								vm.WriteHeapSlice(vm.GlobalAddr("left"), []byte{x})
								vm.WriteHeapSlice(vm.GlobalAddr("right"), []byte{y})
								vm.WriteHeapSlice(vm.GlobalAddr("events"), []byte{5})
								if _, e := vm.Call("execute", nil); e != nil {
									t.Fatal(e)
								}
								if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2)); got != expected {
									t.Fatalf("VM %s %d/%d got%d want%d", abi, x, y, got, expected)
								}
								if vm.ReadHeap(vm.GlobalAddr("events"), 1)[0] != wantEvent || vm.ReadHeap(vm.GlobalAddr("left"), 1)[0] != x || vm.ReadHeap(vm.GlobalAddr("right"), 1)[0] != y {
									t.Fatal("VM effect/input memory")
								}
							}
							z.Memory()[symbol("left")] = x
							z.Memory()[symbol("right")] = y
							z.Memory()[symbol("events")] = 5
							z.Memory()[0xeffe] = 0
							z.Memory()[0xefff] = 0xf1
							z.SetRegisters(emulator.Registers{PC: uint16(symbol("execute")), SP: 0xeffe})
							start := z.GetCycles()
							for n := 0; n < 100000 && z.GetRegisters().PC != 0xf100; n++ {
								z.Step()
							}
							r := z.GetRegisters()
							got := binary.LittleEndian.Uint16(z.Memory()[symbol("result") : symbol("result")+2])
							if got != expected || r.PC != 0xf100 || r.SP != 0xf000 || z.Memory()[symbol("events")] != wantEvent || z.Memory()[symbol("left")] != x || z.Memory()[symbol("right")] != y {
								t.Fatal(fmt.Sprintf("Z80 %s %d/%d got%d want%d PC=%04x SP=%04x", abi, x, y, got, expected, r.PC, r.SP), s.Assembly)
							}
							cycles := z.GetCycles() - start
							total += cycles
							observations = append(observations, map[string]any{"x": x, "y": y, "result": got, "T": cycles})
						}
						rows = append(rows, map[string]any{"abi": abi, "bytes": len(bin.Binary), "T": total, "calls": observations})
						if root := os.Getenv("MINZ_C89_OUTPARAM_ARTIFACT_DIR"); root != "" {
							dir := filepath.Join(root, family, mode, abi)
							if e := os.MkdirAll(dir, 0755); e != nil {
								t.Fatal(e)
							}
							for name, data := range map[string][]byte{"source.c": src, "raw.mir": []byte(raw.Dump()), "final.mir": []byte(s.MIR2Module.Dump()), "target.a80": []byte(s.Assembly), "target.bin": bin.Binary} {
								if e := os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
									t.Fatal(e)
								}
							}
						}
					}
					t.Logf("%s %s memory %dB/%dT scalar %dB/%dT", family, mode, rows[0]["bytes"], rows[0]["T"], rows[1]["bytes"], rows[1]["T"])
					if root := os.Getenv("MINZ_C89_OUTPARAM_ARTIFACT_DIR"); root != "" {
						data, e := json.MarshalIndent(rows, "", "  ")
						if e != nil {
							t.Fatal(e)
						}
						if e := os.WriteFile(filepath.Join(root, family, mode, "cost.json"), data, 0644); e != nil {
							t.Fatal(e)
						}
					}
				})
			}
		})
	}
}
