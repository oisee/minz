package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestOriginalTypedShowcaseNative(t *testing.T) {
	sources := []struct {
		key, path string
		entries   []string
		asserts   int
	}{
		{"ex21_enum", "reports/showcase-src/2026-03-14/ex21_enum.nanz", []string{"get_default_state", "mix_colors", "identity"}, 5},
		{"ex22_type_alias", "reports/showcase-src/2026-03-14/ex22_type_alias.nanz", []string{"damage", "double_score"}, 1},
		{"ex26_enum_assert", "reports/showcase-src/2026-03-14/ex26_enum_assert.nanz", []string{"identity", "add_prio"}, 6},
		{"ex32_value_pipe", "reports/showcase-src/2026-03-15/ex32_value_pipe.nanz", []string{"double", "inc", "add", "piped", "piped_args"}, 0},
		{"ex33_lambda_inference", "reports/showcase-src/2026-03-15/ex33_lambda_inference.nanz", []string{"add_acc", "sum_doubled", "lambda_0"}, 0},
	}
	save := func(name string, data []byte) {
		if root := os.Getenv("MINZ_TYPED_SHOWCASE_ARTIFACTS"); root != "" {
			p := filepath.Join(root, name)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, data, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, source := range sources {
		t.Run(source.key, func(t *testing.T) {
			data, e := os.ReadFile("../../../" + source.path)
			if e != nil {
				t.Fatal(e)
			}
			frozen, e := os.ReadFile("../../../reports/typed-showcase-native/" + source.key + ".nanz")
			if e != nil || !bytes.Equal(data, frozen) {
				t.Fatal("sourcefreeze", e)
			}
			parse := func() *hir.Module {
				h, e := nanz.Parse(string(data), source.path)
				if e != nil {
					t.Fatal(e)
				}
				if len(h.Asserts) != source.asserts {
					t.Fatal("asserts lost", len(h.Asserts))
				}
				return h
			}
			original := hir.LowerModule(parse())
			if len(original.Funcs) != len(source.entries) {
				t.Fatal("original callable count", len(original.Funcs))
			}
			save(source.key+"/raw.mir", []byte(original.Dump()))
			if e := RunAssertsMIR2(parse(), original); e != nil {
				t.Fatal("original asserts", e)
			}
			for _, mode := range []struct {
				name string
				o    Options
			}{{"ordinary", Options{ContractOpt: true}}, {"grace-size", Options{ContractOpt: true, UseHIRGrace: true, UseGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, UseHIRGrace: true, UseGrace: true}}} {
				t.Run(mode.name, func(t *testing.T) {
					s, e := CompileHIRSteps(parse(), mode.o)
					if e != nil {
						t.Fatal(e)
					}
					assembly := s.Assembly
					nativeEntries := map[string]bool{}
					for _, name := range source.entries {
						if s.MIR2Module.FuncByName(name) != nil {
							assembly += fmt.Sprintf("\noracle_%s:\n CALL %s\n HALT\n", name, name)
							nativeEntries[name] = true
						} else if name != "lambda_0" {
							t.Fatal("original entry removed", name)
						}
					}
					image, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + assembly)
					if e != nil || image == nil || len(image.Errors) != 0 {
						t.Fatal("assemble", e, image)
					}
					module, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
					if e != nil || len(module.Errors) != 0 {
						t.Fatal(e)
					}
					prefix := source.key + "/" + mode.name
					save(prefix+"/program.a80", []byte(s.Assembly))
					save(prefix+"/program.bin", module.Binary)
					save(prefix+"/execution.a80", []byte(assembly))
					save(prefix+"/execution.bin", image.Binary)
					save(prefix+"/final.mir", []byte(s.MIR2Module.Dump()))
					metadata, _ := json.MarshalIndent(map[string]any{"options": mode.o, "functions": s.MIR2Module.Funcs, "allocation": s.Allocation, "symbols": image.Symbols}, "", "  ")
					save(prefix+"/abi.json", metadata)
					z := emulator.NewRemogattoZ80()
					mem := z.Memory()
					for i := range mem {
						mem[i] = byte(i*73 + 29)
					}
					if e := z.LoadMemory(0x8000, image.Binary); e != nil {
						t.Fatal(e)
					}
					before := append([]byte(nil), mem...)
					allowed := map[int]bool{}
					for _, f := range s.MIR2Module.Funcs {
						types := map[mir2.Reg]mir2.Ty{}
						for _, p := range f.Contract.Params {
							types[p.Reg] = p.Ty
						}
						for _, b := range f.Blocks {
							for _, inst := range b.Insts {
								types[inst.Dst] = inst.Ty
							}
						}
						for reg, ty := range types {
							if addr, ok := image.Symbols[mir2.Z80SpillLabel(f.Name, reg)]; ok {
								for j := 0; j < mir2.NativeSpillWidth(ty); j++ {
									allowed[addr+j] = true
								}
							}
						}
					}
					raw, final := mir2.NewVM(original), mir2.NewVM(s.MIR2Module)
					effects := 0
					raw.ObserveDataAccess = func(mir2.VMDataAccess) { effects++ }
					final.ObserveDataAccess = func(mir2.VMDataAccess) { effects++ }
					counts, rawOnly := map[string]int{}, map[string]int{}
					cycles := map[string]int64{}
					depths := map[string]int{}
					minSP := uint16(0xf000)
					read := func(name string) uint16 {
						r := z.GetRegisters()
						switch name {
						case "A":
							return uint16(r.A)
						case "B":
							return r.BC >> 8
						case "C":
							return r.BC & 255
						case "D":
							return r.DE >> 8
						case "E":
							return r.DE & 255
						case "H":
							return r.HL >> 8
						case "L":
							return r.HL & 255
						case "HL":
							return r.HL
						case "DE":
							return r.DE
						case "BC":
							return r.BC
						case "IX":
							return r.IX
						case "IY":
							return r.IY
						default:
							t.Fatal("returnlocation", name)
							return 0
						}
					}
					run := func(name string, args []int64, want int64) {
						values := make([]mir2.Value, len(args))
						for i, v := range args {
							values[i] = mir2.Value{I: v}
						}
						v, e := raw.Call(name, values)
						if e != nil || len(v) != 1 || v[0].I != want {
							t.Fatal("raw", name, args, v, want, e)
						}
						if !nativeEntries[name] {
							rawOnly[name]++
							return
						}
						v, e = final.Call(name, values)
						if e != nil || len(v) != 1 || v[0].I != want {
							t.Fatal("final", name, args, v, want, e)
						}
						if effects != 0 {
							t.Fatal("unexpected VM memory effects", effects)
						}
						counts[name]++
						f := s.MIR2Module.FuncByName(name)
						driver := image.Symbols["oracle_"+name]
						z.SetRegisters(emulator.Registers{PC: uint16(driver), SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
						for i, p := range f.Contract.Params {
							loc := assertParamLocation(p, s.Allocation)
							v := uint16(args[i])
							if loc.Kind == mir2.LocMem {
								addr, ok := image.Symbols[mir2.Z80SpillLabel(name, p.Reg)]
								if !ok {
									t.Fatal("paramspill", p)
								}
								mem[addr] = byte(v)
								if mir2.NativeSpillWidth(p.Ty) > 1 {
									mem[addr+1] = byte(v >> 8)
								}
							} else if mir2.NativeSpillWidth(p.Ty) > 1 {
								halves := map[string][2]string{"HL": {"H", "L"}, "DE": {"D", "E"}, "BC": {"B", "C"}, "IX": {"IXH", "IXL"}, "IY": {"IYH", "IYL"}}[loc.Name]
								if halves[0] == "" {
									t.Fatal("wordparam", loc)
								}
								if e := z.SetRegister8(halves[0], byte(v>>8)); e != nil {
									t.Fatal(e)
								}
								if e := z.SetRegister8(halves[1], byte(v)); e != nil {
									t.Fatal(e)
								}
							} else if e := z.SetRegister8(loc.Name, byte(v)); e != nil {
								t.Fatal(e)
							}
						}
						start := z.GetCycles()
						callMin := uint16(0xf000)
						done := false
						for step := 0; step < 10000; step++ {
							if z.GetPC() == uint16(driver+3) && z.GetSP() == 0xf000 {
								done = true
								break
							}
							// Audited exact instruction set of these unchanged modules: only
							// CALL writes memory. Verify its two bytes at the time of access.
							pc, sp := z.GetPC(), z.GetSP()
							op := mem[pc]
							switch op {
							case 0xcd, 0xc9, 0x3e, 0x81, 0x87, 0x80, 0x3c, 0x29, 0x0e, 0x06, 0x5f, 0x78, 0x4f, 0x7b, 0x10, 0xc3:
							default:
								t.Fatalf("unreviewed native effect opcode at %04x: %02x", pc, op)
							}
							z.Step()
							if op == 0xcd && (z.GetSP() != sp-2 || mem[sp-2] != byte(pc+3) || mem[sp-1] != byte((pc+3)>>8)) {
								t.Fatal("CALL return-word access", name, args, pc, sp)
							}
							if z.GetSP() < callMin {
								callMin = z.GetSP()
							}
						}
						if !done {
							t.Fatal("nativePC/SP", name, args, z.GetRegisters())
						}
						if callMin < minSP {
							minSP = callMin
						}
						if d := int(0xf000 - callMin); d > depths[name] {
							depths[name] = d
						}
						if len(f.Contract.Returns) != 1 {
							t.Fatal("returnarity", name)
						}
						ret := f.Contract.Returns[0]
						got := int64(read(mir2.ReturnLocation(ret.Class, ret.Ty)))
						if mir2.NativeSpillWidth(ret.Ty) == 1 {
							got &= 255
						}
						if got != want {
							t.Fatal("native", name, args, got, want, z.GetRegisters())
						}
						cycles[name] += int64(z.GetCycles() - start)
					}
					unary := func(name string, n int, fn func(int) int64) {
						for x := 0; x < n; x++ {
							run(name, []int64{int64(x)}, fn(x))
						}
					}
					pair := func(name string, fn func(int, int) int64) {
						for x := 0; x < 256; x++ {
							for y := 0; y < 256; y++ {
								run(name, []int64{int64(x), int64(y)}, fn(x, y))
							}
						}
					}
					switch source.key {
					case "ex21_enum":
						run("get_default_state", nil, 3)
						run("mix_colors", nil, 5)
						unary("identity", 256, func(x int) int64 { return int64(x) })
					case "ex22_type_alias":
						pair("damage", func(x, y int) int64 { return int64(y) })
						unary("double_score", 65536, func(x int) int64 { return int64((2 * x) & 65535) })
					case "ex26_enum_assert":
						unary("identity", 256, func(x int) int64 { return int64(x) })
						pair("add_prio", func(x, y int) int64 { return int64((x + y) & 255) })
					case "ex32_value_pipe":
						unary("double", 256, func(x int) int64 { return int64((2 * x) & 255) })
						unary("inc", 256, func(x int) int64 { return int64((x + 1) & 255) })
						pair("add", func(x, y int) int64 { return int64((x + y) & 255) })
						run("piped", nil, 11)
						run("piped_args", nil, 16)
					case "ex33_lambda_inference":
						pair("add_acc", func(x, y int) int64 { return int64((x + y) & 255) })
						run("sum_doubled", nil, 30)
						unary("lambda_0", 256, func(x int) int64 { return int64((2 * x) & 255) })
					}
					for _, name := range source.entries {
						if counts[name]+rawOnly[name] == 0 {
							t.Fatal("missingcoverage", name)
						}
					}
					for addr, v := range before {
						if allowed[addr] || addr >= int(minSP) && addr < 0xf000 {
							continue
						}
						if mem[addr] != v {
							t.Fatalf("code/non-stack RAM changed%04x", addr)
						}
					}
					result, _ := json.MarshalIndent(map[string]any{"source_sha256": fmt.Sprintf("%x", sha256.Sum256(data)), "calls": counts, "raw_only_lifted": rawOnly, "source_assertions": source.asserts, "module_bytes": len(module.Binary), "cycles_including_CALL": cycles, "stack_depths": depths, "spill_bytes": allowed, "vm_data_effects": effects, "execution_sha256": fmt.Sprintf("%x", sha256.Sum256(image.Binary))}, "", "  ")
					save(prefix+"/results.json", result)
					t.Log(string(result))
				})
			}
		})
	}
}
