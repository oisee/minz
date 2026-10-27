package pipeline

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestCheckedByteMemoryComposedSpriteNative(t *testing.T) {
	tc := hardwareSpriteCase{name: "masked-row", layout: "linear", base: 0x6000, cols: 6, rows: 1, stride: 6, x: 0, y: 0, width: 6, height: 1, masks: []string{"fff00faa5500"}, graphics: []string{"00050340aaff"}}
	source := hardwareSpriteSource(tc.body("pointer"))
	for _, strategy := range []string{"greedy", "pbqp"} {
		for _, grace := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-grace%v", strategy, grace), func(t *testing.T) {
				options := Options{OptSize: true, AssertMode: "none", ContractOpt: true, ContractStrategy: strategy, UseGrace: grace, UseHIRGrace: grace}
				// A checked local table is an exact configuration, which variant
				// search never touches; compare it with the same unsearched pipeline.
				baselineOptions := options
				baselineOptions.DisableVariantSearch = true
				baseline, e := CompileHIRSteps(hardwareSpriteParse(t, source), baselineOptions)
				if e != nil {
					t.Fatal(e)
				}
				p, e := vir.PrepareLocalTableProblem(baseline.MIR2Module.FuncByName("sprite_test"), baseline.Allocation)
				if e != nil {
					t.Fatal(e)
				}
				var pool, target vir.RegAllocTable
				pool.Init()
				target.Init()
				widths := map[int]int{}
				for _, op := range p.Ops {
					if op.Dst > 0 {
						widths[op.Dst] = op.Width
					}
				}
				begin := 0
				for i, op := range p.Ops {
					if op.Op != vir.OpStore && op.Op != vir.OpRet {
						continue
					}
					if op.Op == vir.OpStore && i+1 < len(p.Ops) && p.Ops[i+1].Op == vir.OpRet {
						continue // Keep the final store and its void return together.
					}
					segment := p.Ops[begin : i+1]
					assignment := map[int]int{}
					for _, node := range segment {
						for _, v := range append([]int{node.Dst}, node.Src[:]...) {
							if v > 0 {
								name := "A"
								if widths[v] == 16 {
									name = "HL"
								}
								assignment[v] = vir.Z80.LocByName(name)
							}
						}
					}
					if e = pool.AddChecked(segment, assignment, vir.Z80, p.Options); e != nil {
						t.Fatal("fragment", begin, i, e)
					}
					begin = i + 1
				}
				if begin != len(p.Ops) {
					t.Fatal("incomplete fragments")
				}
				count, e := pool.ComposeCheckedAutomatically(p, &target, 64)
				if e != nil || count == 0 {
					t.Fatal("composition", count, e)
				}
				path := filepath.Join(t.TempDir(), "table.json")
				if e = target.Save(path); e != nil {
					t.Fatal(e)
				}
				options.CheckedLocalTablePath = path
				selected, e := CompileHIRSteps(hardwareSpriteParse(t, source), options)
				if e != nil {
					t.Fatal(e)
				}
				candidate, e := p.Emit(&target)
				if e != nil {
					t.Fatal(e)
				}
				start, end, ok := checkedFunctionSpan(baseline.Assembly, "sprite_test")
				if !ok {
					t.Fatal("function span")
				}
				rawCandidate := baseline
				rawCandidate.Assembly = baseline.Assembly[:start] + candidate + baseline.Assembly[end:]
				sizes, timings := []int{}, []int{}
				binaries := [][]byte{}
				for _, program := range []Steps{baseline, rawCandidate, selected} {
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + program.Assembly)
					if e != nil {
						t.Fatal(e)
					}
					if len(a.Errors) > 0 {
						t.Fatal(a.Errors)
					}
					cycles := 0
					for seed := 0; seed < 256; seed++ {
						z := emulator.NewRemogattoZ80()
						memory := z.Memory()
						for address := range memory {
							memory[address] = byte(address*37 + address/256*11 + seed)
						}
						if e = z.LoadMemory(0x8000, a.Binary); e != nil {
							t.Fatal(e)
						}
						memory[0xeff0], memory[0xeff1] = 0, 0x70
						expected := append([]byte(nil), memory...)
						masks := []byte{255, 240, 15, 170, 85, 0}
						graphics := []byte{0, 5, 3, 64, 170, 255}
						for i := range masks {
							expected[0x6000+i] = (expected[0x6000+i] & masks[i]) | graphics[i]
						}
						z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["sprite_test"]), SP: 0xeff0, A: 0xa5, F: byte(seed), BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
						for steps := 0; steps < 2000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); steps++ {
							z.Step()
						}
						if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
							t.Fatal("return PC/SP", z.GetRegisters())
						}
						for address, want := range expected {
							if memory[address] != want {
								t.Fatalf("seed%d RAM[%04x]=%02x want%02x", seed, address, memory[address], want)
							}
						}
						cycles = z.GetCycles()
						if program.Assembly == selected.Assembly {
							vm := mir2.NewVM(hir.LowerModule(hardwareSpriteParse(t, source)))
							vm.EnsureHeap(65536)
							vm.WriteHeapSlice(0, expected)
							// Reset only the screen to the pre-draw contents; all other bytes stay.
							for i := range masks {
								vm.WriteHeapSlice(int64(0x6000+i), []byte{byte((0x6000+i)*37 + (0x6000+i)/256*11 + seed)})
							}
							if _, e = vm.Call("sprite_test", nil); e != nil {
								t.Fatal(e)
							}
							got := vm.ReadHeap(0, 65536)
							for address, want := range expected {
								if got[address] != want {
									t.Fatalf("VM RAM[%04x]", address)
								}
							}
						}
					}
					binaries = append(binaries, append([]byte(nil), a.Binary...))
					sizes = append(sizes, len(a.Binary))
					timings = append(timings, cycles)
				}
				if sizes[2] >= sizes[0] || timings[2] >= timings[0] {
					t.Fatal("real sprite did not improve", sizes, timings)
				}
				t.Logf("baseline/candidate/selected: %v B, %v T; %d admitted compositions; 3 x 256 full-RAM/PC/SP native cases", sizes, timings, count)
				if root := os.Getenv("MINZ_CHECKED_BYTE_MEMORY_EXPORT"); root != "" {
					dir := filepath.Join(root, strategy+fmt.Sprint(grace))
					if e = os.MkdirAll(dir, 0755); e != nil {
						t.Fatal(e)
					}
					for name, data := range map[string][]byte{"baseline.bin": binaries[0], "candidate.bin": binaries[1], "selected.bin": binaries[2], "source.nanz": []byte(source), "baseline.a80": []byte(baseline.Assembly), "candidate.a80": []byte(rawCandidate.Assembly), "selected.a80": []byte(selected.Assembly)} {
						if e = os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
							t.Fatal(e)
						}
					}
					if e = target.Save(filepath.Join(dir, "table.json")); e != nil {
						t.Fatal(e)
					}
					if e = pool.Save(filepath.Join(dir, "pool.json")); e != nil {
						t.Fatal(e)
					}
					data, e := json.MarshalIndent(map[string]any{"sizes": sizes, "T_states": timings, "compositions": count}, "", "  ")
					if e != nil {
						t.Fatal(e)
					}
					if e = os.WriteFile(filepath.Join(dir, "metrics.json"), data, 0644); e != nil {
						t.Fatal(e)
					}
				}
			})
		}
	}
}
