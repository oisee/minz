package pipeline

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
)

// Saved pool was prepared only for the original six-byte linear descriptor.
// These inputs change masks, immediates, addresses, shape length and function
// name. A current complete MIR/ABI certificate is rebuilt before selection.
func TestCheckedByteMemoryFragmentsHeldoutDescriptors(t *testing.T) {
	cases := []hardwareSpriteCase{
		{name: "zx-last-row", layout: "zx_bitmap_bytes", base: 0x4000, cols: 32, rows: 192, stride: 32, x: 29, y: 191, width: 3, height: 1, masks: []string{"f05500"}, graphics: []string{"03aa77"}},
		{name: "shifted-linear", layout: "linear", base: 0x6200, cols: 8, rows: 3, stride: 12, x: 2, y: 1, width: 6, height: 1, masks: []string{"ffaaf0550f00"}, graphics: []string{"005509aa2088"}},
		{name: "shorter-linear", layout: "linear", base: 0x6100, cols: 7, rows: 2, stride: 9, x: 1, y: 1, width: 4, height: 1, masks: []string{"f00f5500"}, graphics: []string{"0780aa10"}},
	}
	for _, strategy := range []string{"greedy", "pbqp"} {
		for _, grace := range []bool{false, true} {
			poolPath := fmt.Sprintf("../../../reports/checked-local-byte-memory/cases/%s%v/pool.json", strategy, grace)
			poolBytes, e := os.ReadFile(poolPath)
			if e != nil {
				t.Fatal(e)
			}
			for _, tc := range cases {
				t.Run(fmt.Sprintf("%s-grace%v/%s", strategy, grace, tc.name), func(t *testing.T) {
					var pool vir.RegAllocTable
					pool.Init()
					if e = pool.LoadJSON(poolBytes); e != nil {
						t.Fatal(e)
					}
					source := strings.Replace(hardwareSpriteSource(tc.body("pointer")), "sprite_test", "heldout_draw", 1)
					options := Options{OptSize: true, AssertMode: "none", ContractOpt: true, ContractStrategy: strategy, UseGrace: grace, UseHIRGrace: grace}
					// A checked local table is an exact configuration, which variant
					// search never touches; compare it with the same unsearched pipeline.
					baselineOptions := options
					baselineOptions.DisableVariantSearch = true
					baseline, e := CompileHIRSteps(hardwareSpriteParse(t, source), baselineOptions)
					if e != nil {
						t.Fatal(e)
					}
					// Exact preparation uses the saved assignment pool; no new assignment is
					// derived from this descriptor, and no native outcome guides preparation.
					target, notes, e := ComposeCheckedLocalTables(hardwareSpriteParse(t, source), options, &pool, 64)
					if e != nil || target.Size() == 0 {
						t.Fatal("saved pool did not transfer", notes, e)
					}
					path := filepath.Join(t.TempDir(), "heldout-table.json")
					if e = target.Save(path); e != nil {
						t.Fatal(e)
					}
					options.CheckedLocalTablePath = path
					selected, e := CompileHIRSteps(hardwareSpriteParse(t, source), options)
					if e != nil {
						t.Fatal(e)
					}
					writes := map[int][2]byte{}
					for row := 0; row < tc.height; row++ {
						masks, e := hex.DecodeString(tc.masks[row])
						if e != nil {
							t.Fatal(e)
						}
						graphics, e := hex.DecodeString(tc.graphics[row])
						if e != nil {
							t.Fatal(e)
						}
						for col := 0; col < tc.width; col++ {
							writes[tc.address(tc.x+col, tc.y+row)] = [2]byte{masks[col], graphics[col]}
						}
					}
					sizes, cycles := []int{}, []int{}
					binaries := [][]byte{}
					raw := hir.LowerModule(hardwareSpriteParse(t, source))
					for variant, steps := range []Steps{baseline, selected} {
						a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
						if e != nil {
							t.Fatal(e)
						}
						if len(a.Errors) > 0 {
							t.Fatal(a.Errors)
						}
						entry, ok := a.Symbols["heldout_draw"]
						if !ok {
							t.Fatal("missing entry")
						}
						measured := 0
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
							before := append([]byte(nil), memory...)
							expected := append([]byte(nil), before...)
							for address, op := range writes {
								expected[address] = (expected[address] & op[0]) | op[1]
							}
							z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeff0, A: 0x5a, F: byte(seed), BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
							for count := 0; count < 2000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); count++ {
								z.Step()
							}
							if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
								t.Fatal("heldout PC/SP", z.GetRegisters())
							}
							for address, want := range expected {
								if memory[address] != want {
									t.Fatalf("variant%d seed%d RAM[%04x]=%02x want%02x", variant, seed, address, memory[address], want)
								}
							}
							measured = z.GetCycles()
							if variant == 1 {
								for index, module := range []*mir2.Module{raw, baseline.MIR2Module} {
									vm := mir2.NewVM(module)
									vm.EnsureHeap(65536)
									vm.WriteHeapSlice(0, before)
									if _, e = vm.Call("heldout_draw", nil); e != nil {
										t.Fatal(e)
									}
									got := vm.ReadHeap(0, 65536)
									for address, want := range expected {
										if got[address] != want {
											t.Fatalf("VM%d seed%d RAM[%04x]", index, seed, address)
										}
									}
								}
							}
						}
						sizes = append(sizes, len(a.Binary))
						cycles = append(cycles, measured)
						binaries = append(binaries, a.Binary)
					}
					if sizes[1] >= sizes[0] || cycles[1] >= cycles[0] {
						t.Fatal("heldout selection did not improve", sizes, cycles)
					}
					after, e := os.ReadFile(poolPath)
					if e != nil || string(after) != string(poolBytes) {
						t.Fatal("saved pool changed", e)
					}
					t.Logf("saved fragments: %v B, %v T; 512 native full-memory/PC/SP cases; raw and optimized VM; %v", sizes, cycles, notes)
					if root := os.Getenv("MINZ_CHECKED_BYTE_HELDOUT_EXPORT"); root != "" {
						dir := filepath.Join(root, strategy+fmt.Sprint(grace), tc.name)
						if e = os.MkdirAll(dir, 0755); e != nil {
							t.Fatal(e)
						}
						for name, data := range map[string][]byte{"source.nanz": []byte(source), "pool.json": poolBytes, "baseline.a80": []byte(baseline.Assembly), "selected.a80": []byte(selected.Assembly), "baseline.bin": binaries[0], "selected.bin": binaries[1]} {
							if e = os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
								t.Fatal(e)
							}
						}
						if e = target.Save(filepath.Join(dir, "table.json")); e != nil {
							t.Fatal(e)
						}
						data, e := json.MarshalIndent(map[string]any{"sizes": sizes, "T_states": cycles, "notes": notes}, "", "  ")
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
}
