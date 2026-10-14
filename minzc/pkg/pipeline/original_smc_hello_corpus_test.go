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
	"io"
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalSMCHelloCorpus(t *testing.T) {
	for _, src := range []struct {
		key, sha string
		entries  []string
	}{{"ex15_smc_sprite", "ebf231db98b4ec793a186188138b8b7a3202e0d749e079cd3914877e44d22781", []string{"draw_row", "draw_row4"}}, {"ex16_hello_mze", "adacf7829f2a7b20db2d9d5e1a8701826633e6e0de6ac1e22bcd62e5a5526082", []string{"abs_diff", "main"}}} {
		t.Run(src.key, func(t *testing.T) {
			path := "reports/showcase-src/2026-03-11/" + src.key + ".nanz"
			data, e := os.ReadFile("../../../" + path)
			if e != nil {
				t.Fatal(e)
			}
			other, e := os.ReadFile("../../../reports/showcase-src/2026-03-13/" + src.key + ".nanz")
			if e != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != src.sha || fmt.Sprintf("%x", sha256.Sum256(other)) != src.sha {
				t.Fatal("SHA", e)
			}
			export := func(n string, b []byte) {
				if d := os.Getenv("ASSERT_SMC_HELLO_EXPORT"); d != "" {
					p := filepath.Join(d, src.key, n)
					if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
						t.Fatal(e)
					}
					if e := os.WriteFile(p, b, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			parse := func() *hir.Module {
				h, e := nanz.Parse(string(data), path)
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			original := hir.LowerModule(parse())
			expectedFuncs := append([]string(nil), src.entries...)
			if src.key == "ex15_smc_sprite" {
				// Native source-call support lowers typed SMC setters into MIR2,
				// alongside the two original drawing procedures.
				expectedFuncs = append(expectedFuncs, "draw_row_set_r0", "draw_row4_set_r0")
			}
			if len(original.Funcs) != len(expectedFuncs) {
				t.Fatalf("callable count %d, want exact entries %v", len(original.Funcs), expectedFuncs)
			}
			for _, name := range expectedFuncs {
				if original.FuncByName(name) == nil {
					t.Fatalf("missing callable %s", name)
				}
			}
			export("source.nanz", data)
			export("march13-source.nanz", other)
			export("raw.mir", []byte(original.Dump()))
			for _, mode := range []struct {
				name string
				o    Options
			}{{"ordinary", Options{ContractOpt: true}}, {"grace-size", Options{ContractOpt: true, UseHIRGrace: true, UseGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, UseHIRGrace: true, UseGrace: true}}} {
				t.Run(mode.name, func(t *testing.T) {
					s, e := CompileHIRSteps(parse(), mode.o)
					if e != nil {
						export(mode.name+"-refusal.txt", []byte(e.Error()))
						t.Fatalf("original compile refusal:%v", e)
					}
					export(mode.name+".a80", []byte(s.Assembly))
					export(mode.name+".mir", []byte(s.MIR2Module.Dump()))
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
					if e != nil || a == nil || len(a.Errors) > 0 {
						t.Fatalf("assemble %v %+v", e, a)
					}
					export(mode.name+".bin", a.Binary)
					info, _ := json.MarshalIndent(map[string]any{"functions": s.MIR2Module.Funcs, "options": mode.o, "allocation": s.Allocation, "symbols": a.Symbols}, "", "  ")
					export(mode.name+"-abi.json", info)
					z := emulator.NewRemogattoZ80WithScreen()
					z.SetROMEnd(0)
					p := emulator.NewProfiler()
					z.SetProfiler(p)
					for i := range z.Memory() {
						z.Memory()[i] = byte(i*73 + 29)
					}
					z.LoadMemory(0x8000, a.Binary)
					before := append([]byte(nil), z.Memory()...)
					allowed := map[int]bool{}
					minSP := uint16(0xf000)
					count := 0
					exportedDrivers := map[string]bool{}
					call := func(name string, args []mir2.Value, canonicalHL *uint16) {
						target, ok := a.Symbols[name]
						if !ok {
							t.Fatal("missingentry", name)
						}
						driver := uint16(0x7000)
						driverBytes := []byte{0xcd, byte(target), byte(target >> 8), 0x76}
						z.LoadMemory(driver, driverBytes)
						if !exportedDrivers[name] {
							export(mode.name+"-call-"+name+".bin", driverBytes)
							exportedDrivers[name] = true
						}
						for i := 0; i < 4; i++ {
							allowed[int(driver)+i] = true
						}
						z.SetRegisters(emulator.Registers{PC: driver, SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
						if canonicalHL != nil {
							z.SetRegister8("H", byte(*canonicalHL>>8))
							z.SetRegister8("L", byte(*canonicalHL))
						} else {
							f := s.MIR2Module.FuncByName(name)
							for i, par := range f.Contract.Params {
								loc := s.Allocation.Locs[par.Reg]
								if loc.Kind == mir2.LocMem {
									t.Fatal("unexpected paramspill")
								}
								if e := z.SetRegister8(loc.Name, byte(args[i].I)); e != nil {
									t.Fatal(e)
								}
							}
						}
						done := false
						for step := 0; step < 200000; step++ {
							if z.GetPC() == driver+3 && z.GetSP() == 0xf000 {
								done = true
								break
							}
							z.Step()
							if z.GetSP() < minSP {
								minSP = z.GetSP()
							}
						}
						if !done {
							t.Fatalf("CALL/SP%s %+v", name, z.GetRegisters())
						}
						count++
					}
					if src.key == "ex15_smc_sprite" {
						phaseWrites := map[int]bool{}
						checking := false
						z.SetSMCTracker(func(addr uint16, old, new byte) {
							if checking && !phaseWrites[int(addr)] && !(addr >= 0xeffe && addr < 0xf000) {
								t.Fatalf("unexpected SMC phase write%x", addr)
							}
						})
						for _, name := range src.entries {
							bytesWanted := []byte{195, 60}
							if name == "draw_row4" {
								bytesWanted = []byte{255, 129, 129, 255}
							}
							slot, ok := a.Symbols[name+"$r0$imm"]
							if !ok {
								t.Fatal("SMC slot missing", a.Symbols)
							}
							for _, ptr := range []uint16{0, 1, 0x4000, 0xfffc, 0xfffd, 0xfffe, 0xffff} {
								allowed[slot] = true
								allowed[slot+1] = true
								for i := range bytesWanted {
									addr := uint16(int(ptr) + i)
									allowed[int(addr)] = true
								}
								phaseWrites = map[int]bool{slot: true, slot + 1: true}
								checking = true
								call(name+"_set_r0", nil, &ptr)
								if z.Memory()[slot] != byte(ptr) || z.Memory()[slot+1] != byte(ptr>>8) {
									t.Fatal("SMC patch model")
								}
								phaseWrites = map[int]bool{}
								for i := range bytesWanted {
									phaseWrites[int(uint16(int(ptr)+i))] = true
								}
								call(name, nil, nil)
								for i, w := range bytesWanted {
									addr := uint16(int(ptr) + i)
									if z.Memory()[addr] != w {
										t.Fatalf("sprite%s ptr%x byte%d got%x want%x", name, ptr, i, z.Memory()[addr], w)
									}
								}
							}
							checking = false
							for vi, m := range []*mir2.Module{hir.LowerModule(parse()), s.MIR2Module} {
								vm := mir2.NewVM(m)
								vm.EnsureHeap(65536)
								out, e := vm.Call(name, nil)
								note, _ := json.Marshal(map[string]any{"function": name, "returns": out, "error": fmt.Sprint(e), "scope": "VM patch-slot heap abstraction; not native physical SMC equivalence"})
								export(fmt.Sprintf("%s-vm%d-%s.json", mode.name, vi, name), note)
							}
						}
					} else {
						raw, final := mir2.NewVM(hir.LowerModule(parse())), mir2.NewVM(s.MIR2Module)
						raw.MaxSteps = 0
						final.MaxSteps = 0
						for x := 0; x < 256; x++ {
							for y := 0; y < 256; y++ {
								w := x - y
								if w < 0 {
									w = -w
								}
								args := []mir2.Value{{I: int64(x)}, {I: int64(y)}}
								for _, vm := range []*mir2.VM{raw, final} {
									out, e := vm.Call("abs_diff", args)
									if e != nil || len(out) != 1 || out[0].I != int64(w) {
										t.Fatal("VMabs", x, y, out, e)
									}
								}
								call("abs_diff", args, nil)
								ret := s.MIR2Module.FuncByName("abs_diff").Contract.Returns[0]
								home := mir2.ReturnLocation(ret.Class, ret.Ty)
								r := z.GetRegisters()
								got := r.A
								if home == "C" {
									got = byte(r.BC)
								}
								if int(got) != w {
									t.Fatalf("nativeabs %d,%d got%d", x, y, got)
								}
							}
						}
						want := []byte("=== Nanz Z80 ===\nabs_diff(10, 3) = 7\nabs_diff(200, 55) = 145\nDone!\n")
						for i, vm := range []*mir2.VM{raw, final} {
							r, w, e := os.Pipe()
							if e != nil {
								t.Fatal(e)
							}
							old := os.Stdout
							os.Stdout = w
							out, err := vm.Call("main", nil)
							w.Close()
							os.Stdout = old
							printed, e := io.ReadAll(r)
							r.Close()
							export(fmt.Sprintf("%s-vm%d-output.bin", mode.name, i), printed)
							if err != nil || e != nil || len(out) != 0 || !bytes.Equal(printed, want) {
								t.Fatalf("VMmain output%q err%v/%v", printed, err, e)
							}
						}
						beforeOut := len(z.GetOutput())
						call("main", nil, nil)
						printed := z.GetOutput()[beforeOut:]
						export(mode.name+"-native-output.bin", printed)
						if !bytes.Equal(printed, want) {
							t.Fatalf("nativeoutput%q want%q", printed, want)
						}
					}
					for addr, v := range before {
						if allowed[addr] || addr >= int(minSP) && addr < 0xf000 {
							continue
						}
						if z.Memory()[addr] != v {
							t.Fatalf("unexpectedRAM%s addr%x", src.key, addr)
						}
					}
					for addr, writes := range p.WriteCount {
						if writes > 0 && !allowed[addr] && !(addr >= int(minSP) && addr < 0xf000) {
							t.Fatalf("unexpectedwrite%x", addr)
						}
					}
					note, _ := json.MarshalIndent(map[string]any{"source_sha256": src.sha, "native_calls": count, "module_bytes": len(a.Binary), "max_stack_bytes": 0xf000 - int(minSP), "model": "SMC explicit patcher/bytes or console OUT23; no CP/M runtime claim"}, "", "  ")
					export(mode.name+"-results.json", note)
					t.Log(string(note))
				})
			}
		})
	}
}
