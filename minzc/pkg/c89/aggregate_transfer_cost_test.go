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

// Compare against a correct, caller-owned memory return ABI. An explicit
// natural memory layout disables scalar eligibility while preserving ownership.
func TestC89AggregateTransferCostVMZ80(t *testing.T) {
	for _, family := range []string{"scalar_cost", "scalar_heldout"} {
		t.Run(family, func(t *testing.T) {
			src, e := os.ReadFile(filepath.Join("testdata/call_abi", family+".c"))
			if e != nil {
				t.Fatal(e)
			}
			inputs := [][2]byte{}
			for _, x := range []byte{0, 1, 2, 127, 128, 254, 255} {
				for _, y := range []byte{0, 1, 2, 127, 128, 254, 255} {
					inputs = append(inputs, [2]byte{x, y})
				}
			}
			if family == "scalar_heldout" {
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
				hi, lo := x, byte(x+1)
				if family == "scalar_heldout" {
					if x < 128 {
						hi = x + 1
					} else {
						hi = x - 1
					}
					lo = x ^ 0x5a
				}
				return (uint16(hi)*256 + uint16(lo) + uint16(x)) ^ uint16(y)
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
							// Source origin no longer disables scalarization.
							// Explicit layout is a real memory-ABI admission
							// boundary, while keeping the same byte offsets.
							for _, st := range m.Structs {
								offsets := make([]int, len(st.Fields))
								for i := range offsets {
									offsets[i] = st.ByteOffset(i)
								}
								st.FieldOffsets = offsets
							}
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
						maker, consumer := s.MIR2Module.FuncByName("make_pair"), s.MIR2Module.FuncByName("consume")
						if abi == "scalar" {
							if len(maker.Contract.Returns) != 2 || len(consumer.Contract.Params) != 2 {
								t.Fatal("scalar ABI not selected")
							}
						} else {
							if len(maker.Contract.StructReturnBuffers) != 1 || len(consumer.Contract.Params) != 1 {
								t.Fatal("memory baseline is not a correct caller-owned ABI")
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
								vm.WriteHeapSlice(vm.GlobalAddr("a"), []byte{x})
								vm.WriteHeapSlice(vm.GlobalAddr("b"), []byte{y})
								if _, e := vm.Call("execute", nil); e != nil {
									t.Fatal(e)
								}
								if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2)); got != expected {
									t.Fatalf("VM %s %d/%d got%d want%d", abi, x, y, got, expected)
								}
							}
							z.Memory()[symbol("a")] = x
							z.Memory()[symbol("b")] = y
							z.Memory()[0xeffe] = 0
							z.Memory()[0xefff] = 0xf1
							z.SetRegisters(emulator.Registers{PC: uint16(symbol("execute")), SP: 0xeffe})
							start := z.GetCycles()
							for n := 0; n < 100000 && z.GetRegisters().PC != 0xf100; n++ {
								z.Step()
							}
							r := z.GetRegisters()
							got := binary.LittleEndian.Uint16(z.Memory()[symbol("result") : symbol("result")+2])
							if got != expected || r.PC != 0xf100 || r.SP != 0xf000 {
								t.Fatal(fmt.Sprintf("Z80 %s %d/%d got%d want%d PC=%04x SP=%04x", abi, x, y, got, expected, r.PC, r.SP), s.Assembly)
							}
							cycles := z.GetCycles() - start
							total += cycles
							observations = append(observations, map[string]any{"x": x, "y": y, "result": got, "T": cycles})
						}
						rows = append(rows, map[string]any{"abi": abi, "bytes": len(bin.Binary), "T": total, "calls": observations})
						if root := os.Getenv("MINZ_C89_CALL_ARTIFACT_DIR"); root != "" {
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
					if rows[1]["bytes"].(int) >= rows[0]["bytes"].(int) || rows[1]["T"].(int) >= rows[0]["T"].(int) {
						t.Fatal("eligible scalar ABI lost its measured size/speed benefit", rows)
					}
					t.Logf("%s %s memory %dB/%dT scalar %dB/%dT", family, mode, rows[0]["bytes"], rows[0]["T"], rows[1]["bytes"], rows[1]["T"])
					if root := os.Getenv("MINZ_C89_CALL_ARTIFACT_DIR"); root != "" {
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
