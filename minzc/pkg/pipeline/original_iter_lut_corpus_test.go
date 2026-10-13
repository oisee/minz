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
	"math/bits"
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalIteratorLUTCorpusComplete(t *testing.T) {
	// Source/oracle domains were frozen before execution in frozen-domain.json.
	for _, src := range []struct {
		name, sha string
		entries   []string
	}{{"ex5_lut", "d33826eba80cfd940d02c844ba58a0b085251763552b31de104564009b8c36c4", []string{"popcount"}}, {"ex6_foreach", "e1e53370ff161285310b11fd2a0ad573f96cef85193532c0f1ebe4c9217641eb", []string{"sum_chain", "max_chain"}}, {"ex7_mapinplace", "779c02d554372699c21d41ea3488a05eed304f8cbc55a6e3eca026fee316b780", []string{"add2_inplace"}}} {
		t.Run(src.name, func(t *testing.T) {
			export := func(name string, b []byte) {
				if dir := os.Getenv("ITER_LUT_EXPORT"); dir != "" {
					p := filepath.Join(dir, src.name, name)
					if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
						t.Fatal(e)
					}
					if e := os.WriteFile(p, b, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			path := "reports/showcase-src/2026-03-10/" + src.name + ".nanz"
			data, e := os.ReadFile("../../../" + path)
			if e != nil {
				t.Fatal(e)
			}
			other, e := os.ReadFile("../../../reports/showcase-src/2026-03-13/" + src.name + ".nanz")
			if e != nil || fmt.Sprintf("%x", sha256.Sum256(data)) != src.sha || fmt.Sprintf("%x", sha256.Sum256(other)) != src.sha {
				t.Fatal("source identity", e)
			}
			export("source.nanz", data)
			export("march13-source.nanz", other)
			parse := func() *hir.Module {
				h, e := nanz.Parse(string(data), path)
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			original := hir.LowerModule(parse())
			if len(original.Funcs) != len(src.entries)+map[string]int{"ex7_mapinplace": 1}[src.name] {
				t.Fatal("source callable count", len(original.Funcs))
			}
			export("raw.mir", []byte(original.Dump()))
			for _, mode := range []struct {
				name string
				o    Options
			}{{"ordinary", Options{ContractOpt: true, AssertMode: "none"}}, {"grace-size", Options{ContractOpt: true, AssertMode: "none", UseHIRGrace: true, UseGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, AssertMode: "none", UseHIRGrace: true, UseGrace: true}}} {
				t.Run(mode.name, func(t *testing.T) {
					s, e := CompileHIRSteps(parse(), mode.o)
					if e != nil {
						t.Fatal(e)
					}
					export(mode.name+".a80", []byte(s.Assembly))
					export(mode.name+".mir", []byte(s.MIR2Module.Dump()))
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
					if e != nil || a == nil || len(a.Errors) > 0 {
						t.Fatalf("assemble %v %+v", e, a)
					}
					export(mode.name+".bin", a.Binary)
					info, _ := json.MarshalIndent(map[string]any{"functions": s.MIR2Module.Funcs, "allocation": s.Allocation, "options": mode.o, "symbols": a.Symbols}, "", "  ")
					export(mode.name+"-abi.json", info)
					raw, final := mir2.NewVM(hir.LowerModule(parse())), mir2.NewVM(s.MIR2Module)
					raw.MaxSteps = 0
					final.MaxSteps = 0
					raw.EnsureHeap(65536)
					final.EnsureHeap(65536)
					z := emulator.NewRemogattoZ80()
					for i := range z.Memory() {
						z.Memory()[i] = byte(i*41 + 29)
					}
					z.LoadMemory(0x8000, a.Binary)
					drivers := map[string]uint16{}
					for n, name := range src.entries {
						addr, ok := a.Symbols[name]
						if !ok {
							t.Fatal("missing sourceentry", name)
						}
						entry := uint16(0x7000 + n*4)
						code := []byte{0xcd, byte(addr), byte(addr >> 8), 0x76}
						z.LoadMemory(entry, code)
						drivers[name] = entry
						export(mode.name+"-call-"+name+".bin", code)
					}
					before := append([]byte(nil), z.Memory()...)
					allowed := map[int]bool{}
					for _, f := range s.MIR2Module.Funcs {
						for r, loc := range s.Allocation.Locs {
							if loc.Kind == mir2.LocMem {
								if addr, ok := a.Symbols[mir2.Z80SpillLabel(f.Name, r)]; ok {
									allowed[addr] = true
									allowed[addr+1] = true
								}
							}
						}
					}
					counts := map[string]int{}
					minSP := uint16(0xf000)
					depths := map[string]int{}
					run := func(name string, seed, n int) {
						counts[name]++
						f := s.MIR2Module.FuncByName(name)
						input := make([]byte, 257)
						for i := range input {
							input[i] = byte(seed + (i-1)*73)
						}
						wantMem := append([]byte(nil), input...)
						want := 0
						args := []mir2.Value{{I: 0x4000}, {I: int64(n)}}
						if src.name == "ex5_lut" {
							args = []mir2.Value{{I: int64(seed)}}
							want = bits.OnesCount8(byte(seed))
						} else {
							for i := 0; i < n; i++ {
								v := input[i+1]
								switch name {
								case "sum_chain":
									want = (want + int(v)) & 255
								case "max_chain":
									if int(v) > want {
										want = int(v)
									}
								case "add2_inplace":
									wantMem[i+1] = v + 2
								}
							}
							z.LoadMemory(0x3fff, input)
						}
						for vi, vm := range []*mir2.VM{raw, final} {
							if src.name != "ex5_lut" {
								vm.WriteHeapSlice(0x3fff, input)
							}
							events := []mir2.VMDataAccess{}
							vm.ObserveDataAccess = func(v mir2.VMDataAccess) { events = append(events, v) }
							out, e := vm.Call(name, args)
							if e != nil {
								t.Fatalf("VM%s seed%d n%d %v", name, seed, n, e)
							}
							if name == "add2_inplace" {
								if len(out) != 0 {
									t.Fatal(out)
								}
							} else if len(out) != 1 || out[0].I != int64(want) {
								t.Fatalf("VM%d %s seed%d n%d got%v want%d", vi, name, seed, n, out, want)
							}
							if src.name != "ex5_lut" {
								if !bytes.Equal(vm.ReadHeap(0x3fff, len(input)), wantMem) {
									t.Fatal("VM memory", name, seed, n)
								}
								expected := n
								if name == "add2_inplace" {
									expected *= 2
								}
								if len(events) != expected {
									t.Fatalf("VM access count %s n%d got%d", name, n, len(events))
								}
								for i := 0; i < n; i++ {
									ix := i
									if name == "add2_inplace" {
										ix *= 2
									}
									r := events[ix]
									if r.Op != mir2.OpLoad || r.Addr != int64(0x4000+i) || r.Value != input[i+1] {
										t.Fatal("VM read order", name, i, r)
									}
									if name == "add2_inplace" {
										w := events[ix+1]
										if w.Op != mir2.OpStore || w.Addr != r.Addr || w.Value != wantMem[i+1] {
											t.Fatal("VM write order", i, w)
										}
									}
								}
							}
						}
						z.SetRegisters(emulator.Registers{PC: drivers[name], SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
						for i, p := range f.Contract.Params {
							v := uint16(args[i].I)
							loc := s.Allocation.Locs[p.Reg]
							if loc.Kind == mir2.LocMem {
								addr, ok := a.Symbols[mir2.Z80SpillLabel(name, p.Reg)]
								if !ok {
									t.Fatal("spill", p)
								}
								z.Memory()[addr] = byte(v)
								if mir2.NativeSpillWidth(p.Ty) > 1 {
									z.Memory()[addr+1] = byte(v >> 8)
								}
								continue
							}
							if mir2.NativeSpillWidth(p.Ty) > 1 {
								halves := map[string][2]string{"HL": {"H", "L"}, "DE": {"D", "E"}, "BC": {"B", "C"}, "IX": {"IXH", "IXL"}, "IY": {"IYH", "IYL"}}[loc.Name]
								if halves[0] == "" {
									t.Fatal(loc)
								}
								z.SetRegister8(halves[0], byte(v>>8))
								z.SetRegister8(halves[1], byte(v))
							} else if e := z.SetRegister8(loc.Name, byte(v)); e != nil {
								t.Fatal(e)
							}
						}
						callMin := uint16(0xf000)
						nativeEvents := []mir2.VMDataAccess{}
						done := false
						for i := 0; i < 200000; i++ {
							if z.GetPC() == drivers[name]+3 && z.GetSP() == 0xf000 {
								done = true
								break
							}
							if src.name != "ex5_lut" {
								r := z.GetRegisters()
								op := z.Memory()[r.PC]
								if r.HL >= 0x4000 && r.HL < 0x40ff {
									if op&0xc7 == 0x46 && op != 0x76 {
										nativeEvents = append(nativeEvents, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(r.HL), Value: z.Memory()[r.HL]})
									}
									if op == 0x77 {
										nativeEvents = append(nativeEvents, mir2.VMDataAccess{Op: mir2.OpStore, Addr: int64(r.HL), Value: r.A})
									}
								}
							}
							z.Step()
							if z.GetSP() < callMin {
								callMin = z.GetSP()
							}
						}
						if !done {
							t.Fatalf("nativeCALL/SP %s seed%d n%d %+v", name, seed, n, z.GetRegisters())
						}
						if callMin < minSP {
							minSP = callMin
						}
						if d := int(0xf000 - callMin); d > depths[name] {
							depths[name] = d
						}
						if name != "add2_inplace" {
							ret := mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
							r := z.GetRegisters()
							got := 0
							switch ret {
							case "A":
								got = int(r.A)
							case "C":
								got = int(byte(r.BC))
							case "B":
								got = int(r.BC >> 8)
							default:
								t.Fatal("returnhome", ret)
							}
							if got != want {
								t.Fatalf("native%s seed%d n%d got%d want%d", name, seed, n, got, want)
							}
						}
						if src.name != "ex5_lut" {
							expected := n
							if name == "add2_inplace" {
								expected *= 2
							}
							if len(nativeEvents) != expected {
								t.Fatalf("native access count %s n%d got%d", name, n, len(nativeEvents))
							}
							for i := 0; i < n; i++ {
								ix := i
								if name == "add2_inplace" {
									ix *= 2
								}
								r := nativeEvents[ix]
								if r.Op != mir2.OpLoad || r.Addr != int64(0x4000+i) || r.Value != input[i+1] {
									t.Fatal("native read order", name, i, r)
								}
								if name == "add2_inplace" {
									w := nativeEvents[ix+1]
									if w.Op != mir2.OpStore || w.Addr != r.Addr || w.Value != wantMem[i+1] {
										t.Fatal("native write order", i, w)
									}
								}
							}
						}
						if src.name != "ex5_lut" && !bytes.Equal(z.Memory()[0x3fff:0x4100], wantMem) {
							t.Fatalf("native memory %s seed%d n%d", name, seed, n)
						}
					}
					for seed := 0; seed < 256; seed++ {
						if src.name == "ex5_lut" {
							run("popcount", seed, 0)
						} else {
							for n := 0; n < 256; n++ {
								for _, name := range src.entries {
									run(name, seed, n)
								}
							}
						}
					}
					for addr, v := range before {
						if allowed[addr] || addr >= int(minSP) && addr < 0xf000 || src.name != "ex5_lut" && addr >= 0x3fff && addr < 0x4100 {
							continue
						}
						if z.Memory()[addr] != v {
							t.Fatalf("fullRAM unexpected change %04x", addr)
						}
					}
					summary, _ := json.MarshalIndent(map[string]any{"entries": counts, "module_bytes": len(a.Binary), "source_sha256": src.sha, "stack_depths": depths, "allowed_spill_bytes": allowed}, "", "  ")
					export(mode.name+"-results.json", summary)
					t.Log(string(summary))
				})
			}
		})
	}
}
