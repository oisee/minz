package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestOriginalAdvantageNative(t *testing.T) {
	sources := []struct {
		key, path string
		entries   []string
		asserts   int
	}{{"advantage", "examples/tests/nanz_advantage_showcase.nanz", []string{"double", "inc", "square", "pipeline_demo", "add_acc", "sum_all", "sum_plus_offset", "fib", "fib8", "fib12", "opposite", "board_idx", "main"}, 9}}
	save := func(name string, data []byte) {
		if root := os.Getenv("MINZ_ADVANTAGE_ARTIFACTS"); root != "" {
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
			frozen, e := os.ReadFile("../../../reports/advantage-native/source.nanz")
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
							if name != "main" {
								assembly += fmt.Sprintf("\noracle_%s:\n CALL %s\n HALT\n", name, name)
							}
							nativeEntries[name] = true
						} else {
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
					var resource []byte
					run := func(name string, args []int64, want int64) {
						values := make([]mir2.Value, len(args))
						for i, v := range args {
							values[i] = mir2.Value{I: v}
						}
						var rawEvents, finalEvents []mir2.VMDataAccess
						raw.ObserveDataAccess = func(e mir2.VMDataAccess) { rawEvents = append(rawEvents, e) }
						final.ObserveDataAccess = func(e mir2.VMDataAccess) { finalEvents = append(finalEvents, e) }
						if len(resource) > 0 {
							raw.WriteHeapBytes(raw.GlobalAddr("vals"), resource)
							final.WriteHeapBytes(final.GlobalAddr("vals"), resource)
						}
						v, e := raw.Call(name, values)
						if e != nil || (name != "main" && (len(v) != 1 || v[0].I != want)) || (name == "main" && len(v) != 0) {
							t.Fatal("raw", name, args, v, want, e)
						}
						if !nativeEntries[name] {
							rawOnly[name]++
							return
						}
						v, e = final.Call(name, values)
						if e != nil || (name != "main" && (len(v) != 1 || v[0].I != want)) || (name == "main" && len(v) != 0) {
							t.Fatal("final", name, args, v, want, e)
						}

						for _, vmcase := range []struct {
							vm     *mir2.VM
							events []mir2.VMDataAccess
						}{{raw, rawEvents}, {final, finalEvents}} {
							if len(resource) > 0 {
								if len(vmcase.events) != 5 {
									t.Fatal("foldVMaccesscount", name, vmcase.events)
								}
								addr := vmcase.vm.GlobalAddr("vals")
								for i, event := range vmcase.events {
									if event.Op != mir2.OpLoad || event.Addr != addr+int64(i) || event.Value != resource[i] {
										t.Fatal("foldVMorder", name, vmcase.events)
									}
								}
								if !bytes.Equal(vmcase.vm.ReadHeap(addr, 5), resource) {
									t.Fatal("foldVMchangedresource")
								}
							} else if len(vmcase.events) != 0 {
								t.Fatal("unexpectedVMeffect", name, vmcase.events)
							}
						}
						counts[name]++
						f := s.MIR2Module.FuncByName(name)
						driver := image.Symbols["oracle_"+name]
						if name == "main" {
							driver = image.Symbols["main"]
						}
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
						if len(resource) > 0 {
							addr := image.Symbols["vals"]
							copy(mem[addr:addr+5], resource)
						}
						var nativeEvents []mir2.VMDataAccess
						start := z.GetCycles()
						callMin := uint16(0xf000)
						done := false
						for step := 0; step < 10000; step++ {
							if (name != "main" && z.GetPC() == uint16(driver+3) && z.GetSP() == 0xf000) || (name == "main" && z.GetPC() == uint16(driver+2) && z.GetSP() == 0xf000 && mem[z.GetPC()] == 0x76) {
								done = true
								break
							}
							// Observe source data reads separately from stack saves; validate
							// physical PUSH/CALL bytes before later saves can reuse them.
							pc, sp := z.GetPC(), z.GetSP()
							op := mem[pc]
							nativeEvents = append(nativeEvents, widemathNativeByteAccess(mem, pc, z.GetRegisters())...)
							r := z.GetRegisters()
							stackOp := op
							if op == 0xdd || op == 0xfd {
								stackOp = mem[pc+1]
							}
							z.Step()
							if stackOp == 0xc5 || stackOp == 0xd5 || stackOp == 0xe5 || stackOp == 0xf5 {
								word := r.BC
								switch stackOp {
								case 0xd5:
									word = r.DE
								case 0xe5:
									word = r.HL
									if op == 0xdd {
										word = r.IX
									}
									if op == 0xfd {
										word = r.IY
									}
								case 0xf5:
									word = uint16(r.A)<<8 | uint16(r.F)
								}
								if z.GetSP() != sp-2 || mem[sp-2] != byte(word) || mem[sp-1] != byte(word>>8) {
									t.Fatal("scratch PUSH word", name, args, word)
								}
							}
							if op == 0xcd && (z.GetSP() != sp-2 || mem[sp-2] != byte(pc+3) || mem[sp-1] != byte((pc+3)>>8)) {
								t.Fatal("CALL return-word access", name, args, pc, sp)
							}
							if z.GetSP() < callMin {
								callMin = z.GetSP()
							}
						}
						if len(resource) > 0 {
							addr := image.Symbols["vals"]
							if len(nativeEvents) != 5 {
								t.Fatal("foldnativeaccesscount", name, nativeEvents)
							}
							for i, event := range nativeEvents {
								if event.Op != mir2.OpLoad || event.Addr != int64(addr+i) || event.Value != resource[i] {
									t.Fatal("foldnativeorder", name, nativeEvents)
								}
							}
							if !bytes.Equal(mem[addr:addr+5], resource) {
								t.Fatal("foldnativechangedresource")
							}
						} else if len(nativeEvents) != 0 {
							t.Fatal("unexpectednativeeffect", name, nativeEvents)
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
						if name != "main" {
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
						} else if len(f.Contract.Returns) != 0 {
							t.Fatal("mainvoidABI")
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
					unary("double", 256, func(x int) int64 { return int64(byte(x * 2)) })
					unary("inc", 256, func(x int) int64 { return int64(byte(x + 1)) })
					unary("square", 256, func(x int) int64 { return int64(byte(x * x)) })
					pair("add_acc", func(x, y int) int64 { return int64(byte(x + y)) })
					pair("board_idx", func(x, y int) int64 { return int64(byte(10*y + x)) })
					run("pipeline_demo", nil, 49)
					run("fib8", nil, 21)
					run("fib12", nil, 144)
					unary("fib", 255, func(n int) int64 {
						a, b := byte(0), byte(1)
						for i := 0; i < n; i++ {
							a, b = b, a+b
						}
						return int64(a)
					})
					unary("opposite", 4, func(x int) int64 { return int64(x ^ 1) })
					run("main", nil, 0)
					resources := [][]byte{{10, 20, 30, 40, 50}}
					for lane := 0; lane < 5; lane++ {
						for x := 0; x < 256; x++ {
							a := []byte{17, 91, 203, 7, 31}
							a[lane] = byte(x)
							resources = append(resources, a)
						}
					}
					bounds := []byte{0, 1, 127, 128, 254, 255}
					for _, a := range bounds {
						for _, b := range bounds {
							for _, c := range bounds {
								for _, d := range bounds {
									for _, e := range bounds {
										resources = append(resources, []byte{a, b, c, d, e})
									}
								}
							}
						}
					}
					for _, a := range resources {
						resource = a
						sum := byte(0)
						for _, v := range a {
							sum += v
						}
						run("sum_all", nil, int64(sum))
						run("sum_plus_offset", nil, int64(byte(sum+5)))
					}
					resource = nil
					// n255 is a defined non-returning source path: byte i wraps while
					// i<=255 remains true. Do not manufacture a Fibonacci return value.
					for _, m := range []*mir2.Module{original, s.MIR2Module} {
						vm := mir2.NewVM(m)
						vm.MaxSteps = 10000
						v, e := vm.Call("fib", []mir2.Value{{I: 255}})
						if e == nil || !strings.Contains(e.Error(), "gas limit reached") || len(v) != 0 {
							t.Fatal("fib255 expected budget nonreturn", v, e)
						}
					}
					head, ok := image.Symbols["fib.fib_loop_head3"]
					if !ok {
						t.Fatal("fib loophead symbol")
					}
					fib := s.MIR2Module.FuncByName("fib")
					driver := image.Symbols["oracle_fib"]
					z.SetRegisters(emulator.Registers{PC: uint16(driver), SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
					loc := assertParamLocation(fib.Contract.Params[0], s.Allocation)
					if loc.Kind == mir2.LocMem {
						t.Fatal("fib255 unexpected spilled entry")
					}
					if e := z.SetRegister8(loc.Name, 255); e != nil {
						t.Fatal(e)
					}
					seen := map[emulator.Registers]bool{}
					cyclic := false
					visits := 0
					for step := 0; step < 100000; step++ {
						if z.GetPC() == uint16(driver+3) {
							t.Fatal("fib255 native unexpectedly returns")
						}
						if int(z.GetPC()) == head {
							r := z.GetRegisters()
							visits++
							if seen[r] {
								cyclic = true
								break
							}
							seen[r] = true
						}
						z.Step()
					}
					if !cyclic || visits < 257 || z.GetSP() != 0xeffe {
						t.Fatal("fib255 no complete stable state cycle", visits, z.GetRegisters())
					}
					nonreturn := map[string]any{"input": 255, "vm_max_steps": 10000, "native_loop_head_visits": visits, "complete_register_state_cycle": cyclic, "native_sp": z.GetSP()}
					proof, _ := json.MarshalIndent(nonreturn, "", "  ")
					save(prefix+"/fib255-nonreturn.json", proof)
					for _, name := range source.entries {
						if counts[name]+rawOnly[name] == 0 {
							t.Fatal("missingcoverage", name)
						}
					}
					for addr, v := range before {
						if source.key == "advantage" && addr >= image.Symbols["vals"] && addr < image.Symbols["vals"]+5 {
							continue
						}
						if allowed[addr] || addr >= int(minSP) && addr < 0xf000 {
							continue
						}
						if mem[addr] != v {
							t.Fatalf("code/non-stack RAM changed%04x", addr)
						}
					}
					result, _ := json.MarshalIndent(map[string]any{"source_sha256": fmt.Sprintf("%x", sha256.Sum256(data)), "calls": counts, "raw_only_lifted": rawOnly, "source_assertions": source.asserts, "module_bytes": len(module.Binary), "cycles_including_CALL": cycles, "stack_depths": depths, "spill_bytes": allowed, "execution_sha256": fmt.Sprintf("%x", sha256.Sum256(image.Binary))}, "", "  ")
					save(prefix+"/results.json", result)
					t.Log(string(result))
				})
			}
		})
	}
}
