package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestOriginalStructInterfaceNative(t *testing.T) {
	sources := []struct {
		key, path, global string
		entries           []string
	}{{"ex1_struct", "reports/showcase-src/2026-03-10/ex1_struct.nanz", "palette", []string{"set_rgb", "get_r", "get_g", "get_b"}}, {"ex2_ufcs", "reports/showcase-src/2026-03-10/ex2_ufcs.nanz", "acc_g", []string{"Acc_add", "Acc_reset", "sum_two"}}, {"ex3_iface", "reports/showcase-src/2026-03-10/ex3_iface.nanz", "", []string{"Dog_speak", "Cat_speak", "demo"}}, {"ex3b_iface_param", "reports/showcase-src/2026-03-10/ex3b_iface_param.nanz", "", []string{"Dog_speak", "feed"}}, {"ex0_test_struct", "reports/showcase-src/2026-03-11/ex0_test_struct.nanz", "acc_g", []string{"Acc_add", "Acc_reset", "sum_two"}}}
	save := func(name string, data []byte) {
		if root := os.Getenv("MINZ_STRUCT_INTERFACE_ARTIFACTS"); root != "" {
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
			frozen, e := os.ReadFile("../../../reports/struct-interface-native/" + source.key + ".nanz")
			if e != nil || !bytes.Equal(data, frozen) {
				t.Fatal("freeze", e)
			}
			parse := func() *hir.Module {
				h, e := nanz.Parse(string(data), source.path)
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			original := hir.LowerModule(parse())
			if len(original.Funcs) != len(source.entries) {
				t.Fatal("originalentries")
			}
			save(source.key+"/raw.mir", []byte(original.Dump()))
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
					for _, name := range source.entries {
						assembly += fmt.Sprintf("\noracle_%s:\n CALL %s\n HALT\n", name, name)
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
					meta, _ := json.MarshalIndent(map[string]any{"options": mode.o, "functions": s.MIR2Module.Funcs, "allocation": s.Allocation, "symbols": image.Symbols}, "", "  ")
					save(prefix+"/abi.json", meta)
					z := emulator.NewRemogattoZ80()
					mem := z.Memory()
					for i := range mem {
						mem[i] = byte(i*73 + 29)
					}
					z.LoadMemory(0x8000, image.Binary)
					pristine := append([]byte(nil), mem...)
					raw, final := mir2.NewVM(original), mir2.NewVM(s.MIR2Module)
					raw.EnsureHeap(65536)
					final.EnsureHeap(65536)
					counts := map[string]int{}
					cycles := map[string]int64{}
					depths := map[string]int{}
					minSP := uint16(0xf000)
					mutable := map[int]bool{}
					globalNative := uint16(0)
					if source.global != "" {
						g, ok := image.Symbols[source.global]
						if !ok {
							t.Fatal("global", source.global)
						}
						globalNative = uint16(g)
					}
					run := func(name string, args []int64, input, output []byte, ptr uint16, want []int64) {
						counts[name]++
						f := s.MIR2Module.FuncByName(name)
						if f == nil {
							t.Fatal("entry removed", name)
						}
						// Address-independent source effect oracle; loads may be legally forwarded,
						// while all ordered stores and their values must survive exactly.
						stores := []byte{}
						switch name {
						case "set_rgb":
							stores = append(stores, output...)
						case "Acc_add", "Acc_reset":
							stores = append(stores, output...)
						case "sum_two":
							stores = []byte{0, byte(args[0]), byte(args[0] + args[1])}
						}
						for _, vm := range []*mir2.VM{raw, final} {
							addr := ptr
							if source.global != "" && (name == "set_rgb" || name == "get_r" || name == "get_g" || name == "get_b" || name == "sum_two") {
								addr = uint16(vm.GlobalAddr(source.global))
							}
							vm.WriteHeapBytes(int64(addr), input)
							vm.WriteHeap(int64(uint16(addr-1)), 0xa7)
							vm.WriteHeap(int64(uint16(addr+uint16(len(input)))), 0xb9)
							vals := make([]mir2.Value, len(args))
							for i, a := range args {
								vals[i] = mir2.Value{I: a}
							}
							var events []mir2.VMDataAccess
							vm.ObserveDataAccess = func(a mir2.VMDataAccess) { events = append(events, a) }
							got, e := vm.Call(name, vals)
							if e != nil || len(got) != len(want) {
								t.Fatal("VMcall", name, args, got, want, e)
							}
							for i, w := range want {
								if got[i].I != w {
									t.Fatal("VMresult", name, args, got, want)
								}
							}
							gotStores := []byte{}
							for _, event := range events {
								off := uint16(event.Addr) - addr
								if int(off) >= len(input) {
									t.Fatal("VMobjecteffectaddress", name, event, addr)
								}
								if event.Op == mir2.OpStore {
									gotStores = append(gotStores, event.Value)
								}
							}
							if !reflect.DeepEqual(gotStores, stores) {
								t.Fatal("VMstoreorder", name, args, gotStores, stores)
							}
							// With plain mutable RAM (no volatile/call observer), redundant reads
							// may disappear; every read that remains must observe the current
							// independently simulated field value and every store its source lane.
							state := append([]byte(nil), input...)
							for _, event := range events {
								off := int(uint16(event.Addr) - addr)
								if event.Op == mir2.OpLoad && event.Value != state[off] {
									t.Fatal("VM read value/order", name, args, event, state)
								}
								if event.Op == mir2.OpStore {
									state[off] = event.Value
								}
							}
							if !bytes.Equal(vm.ReadHeap(int64(addr), len(output)), output) || vm.ReadHeap(int64(uint16(addr-1)), 1)[0] != 0xa7 || vm.ReadHeap(int64(uint16(addr+uint16(len(input)))), 1)[0] != 0xb9 {
								t.Fatal("VMwholeobjectguard", name, args)
							}
						}
						addr := ptr
						if source.global != "" && (name == "set_rgb" || name == "get_r" || name == "get_g" || name == "get_b" || name == "sum_two") {
							addr = globalNative
						}
						for i, v := range input {
							mem[uint16(addr+uint16(i))] = v
							mutable[int(uint16(addr+uint16(i)))] = true
						}
						guardBefore := mem[uint16(addr-1)]
						guardAfter := mem[uint16(addr+uint16(len(input)))]
						driver := image.Symbols["oracle_"+name]
						z.SetRegisters(emulator.Registers{PC: uint16(driver), SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
						for i, p := range f.Contract.Params {
							v := uint16(args[i])
							loc := assertParamLocation(p, s.Allocation)
							if loc.Kind == mir2.LocMem {
								if a, ok := image.Symbols[mir2.Z80SpillLabel(name, p.Reg)]; ok {
									mem[a] = byte(v)
									mutable[a] = true
									if mir2.NativeSpillWidth(p.Ty) > 1 {
										mem[a+1] = byte(v >> 8)
										mutable[a+1] = true
									}
								} else if st, ok := p.Ty.(*mir2.StructTy); !ok || mir2.ByteWidth(st) != 0 {
									t.Fatal("missing nonemptyparamhome", p)
								}
							} else if mir2.NativeSpillWidth(p.Ty) > 1 {
								halves := map[string][2]string{"HL": {"H", "L"}, "DE": {"D", "E"}, "BC": {"B", "C"}, "IX": {"IXH", "IXL"}, "IY": {"IYH", "IYL"}}[loc.Name]
								if halves[0] == "" {
									t.Fatal(loc)
								}
								z.SetRegister8(halves[0], byte(v>>8))
								z.SetRegister8(halves[1], byte(v))
							} else {
								z.SetRegister8(loc.Name, byte(v))
							}
						}
						nativeStores := []byte{}
						nativeState := append([]byte(nil), input...)
						callMin := uint16(0xf000)
						start := z.GetCycles()
						done := false
						for step := 0; step < 10000; step++ {
							if z.GetPC() == uint16(driver+3) && z.GetSP() == 0xf000 {
								done = true
								break
							}
							pc, sp := z.GetPC(), z.GetSP()
							op := mem[pc]
							events := widemathNativeByteAccess(mem, pc, z.GetRegisters())
							if op == 0x3a || op == 0x32 {
								a := uint16(mem[pc+1]) | uint16(mem[pc+2])<<8
								o := mir2.OpLoad
								v := mem[a]
								if op == 0x32 {
									o = mir2.OpStore
									v = z.GetRegisters().A
								}
								events = []mir2.VMDataAccess{{Op: o, Addr: int64(a), Value: v}}
							}
							for _, event := range events {
								off := int(uint16(event.Addr) - addr)
								if off >= len(input) {
									t.Fatal("nativeobjecteffectaddress", name, args, event, addr)
								}
								if event.Op == mir2.OpLoad && event.Value != nativeState[off] {
									t.Fatal("native read value/order", name, args, event, nativeState)
								}
								if event.Op == mir2.OpStore {
									nativeState[off] = event.Value
									nativeStores = append(nativeStores, event.Value)
								}
							}
							z.Step()
							if op == 0xcd && (z.GetSP() != sp-2 || mem[sp-2] != byte(pc+3) || mem[sp-1] != byte((pc+3)>>8)) {
								t.Fatal("CALLword", name)
							}
							if z.GetSP() < callMin {
								callMin = z.GetSP()
							}
						}
						if !done {
							t.Fatal("nativePC/SP", name, args, z.GetRegisters())
						}
						if !reflect.DeepEqual(nativeStores, stores) {
							t.Fatal("nativestoreorder", name, args, nativeStores, stores)
						}
						for i, v := range output {
							if mem[uint16(addr+uint16(i))] != v {
								t.Fatal("nativewholeobject", name, args, i, mem[uint16(addr+uint16(i))], v)
							}
						}
						if mem[uint16(addr-1)] != guardBefore || mem[uint16(addr+uint16(len(input)))] != guardAfter {
							t.Fatal("nativeadjacentguard", name, args)
						}
						if len(f.Contract.Returns) != len(want) {
							t.Fatal("returnarity", name)
						}
						if len(want) > 0 {
							ret := mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
							r := z.GetRegisters()
							got := int64(r.A)
							if ret != "A" {
								t.Fatal("scalarreturnhome", ret)
							}
							if got != want[0] {
								t.Fatal("nativeresult", name, args, got, want)
							}
						}
						if callMin < minSP {
							minSP = callMin
						}
						if d := int(0xf000 - callMin); d > depths[name] {
							depths[name] = d
						}
						cycles[name] += int64(z.GetCycles() - start)
					}
					if source.key == "ex1_struct" {
						for lane := 0; lane < 3; lane++ {
							for x := 0; x < 256; x++ {
								rgb := []byte{17, 91, 203}
								rgb[lane] = byte(x)
								run("set_rgb", []int64{int64(rgb[0]), int64(rgb[1]), int64(rgb[2])}, []byte{9, 19, 29}, rgb, 0, nil)
								for j, name := range []string{"get_r", "get_g", "get_b"} {
									run(name, nil, rgb, rgb, 0, []int64{int64(rgb[j])})
								}
							}
						}
						for _, r := range []byte{0, 1, 127, 128, 254, 255} {
							for _, g := range []byte{0, 1, 127, 128, 254, 255} {
								for _, b := range []byte{0, 1, 127, 128, 254, 255} {
									run("set_rgb", []int64{int64(r), int64(g), int64(b)}, []byte{9, 19, 29}, []byte{r, g, b}, 0, nil)
								}
							}
						}
					} else if source.global == "acc_g" {
						for _, ptr := range []uint16{0x6000, 0xffff} {
							for x := 0; x < 256; x++ {
								run("Acc_reset", []int64{int64(ptr)}, []byte{byte(x)}, []byte{0}, ptr, nil)
								for y := 0; y < 256; y++ {
									v := byte(x + y)
									run("Acc_add", []int64{int64(ptr), int64(y)}, []byte{byte(x)}, []byte{v}, ptr, []int64{int64(v)})
								}
							}
						}
						for x := 0; x < 256; x++ {
							for y := 0; y < 256; y++ {
								v := byte(x + y)
								run("sum_two", []int64{int64(x), int64(y)}, []byte{173}, []byte{v}, 0, []int64{int64(v)})
							}
						}
					} else {
						for _, name := range source.entries {
							if name == "demo" {
								run(name, nil, nil, nil, 0x6000, []int64{1})
								continue
							}
							for _, ptr := range []uint16{0x6000, 0xffff} {
								v := int64(1)
								if name == "Cat_speak" {
									v = 2
								}
								run(name, []int64{int64(ptr)}, nil, nil, ptr, []int64{v})
							}
						}
					}
					for a, v := range pristine {
						if mutable[a] || a >= int(minSP) && a < 0xf000 {
							continue
						}
						if mem[a] != v {
							t.Fatalf("nativefullRAM/code%04x", a)
						}
					}
					result, _ := json.MarshalIndent(map[string]any{"source_sha256": fmt.Sprintf("%x", sha256.Sum256(data)), "calls": counts, "cycles_including_CALL": cycles, "stack_depths": depths, "module_bytes": len(module.Binary), "mutable_object_guard_addresses": mutable, "execution_sha256": fmt.Sprintf("%x", sha256.Sum256(image.Binary))}, "", "  ")
					save(prefix+"/results.json", result)
					t.Log(string(result))
				})
			}
		})
	}
}
