package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/plm"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalPLMSumCorpusComplete(t *testing.T) {
	// Domains and independent BYTE sum oracle frozen before execution.
	for si, src := range []struct{ path, sha string }{
		{"examples/nanz/01_sum_array.plm", "fa0911df63dbc5bb16f169e1392f92be3f64e09c299156127ef37a7ff9d20c8e"},
		{"examples/nanz/01_sum_plm_origin.plm", "6564b22b460943d7b8ac088ec482f05857f752eb40f5c6103534fcaf2a0ec023"},
		{"examples/plm/sum_array.plm", "6564b22b460943d7b8ac088ec482f05857f752eb40f5c6103534fcaf2a0ec023"},
	} {
		t.Run(fmt.Sprintf("source-%d", si), func(t *testing.T) {
			data, e := os.ReadFile("../../../" + src.path)
			if e != nil {
				t.Fatal(e)
			}
			if fmt.Sprintf("%x", sha256.Sum256(data)) != src.sha {
				t.Fatal("source SHA")
			}
			export := func(n string, b []byte) {
				if d := os.Getenv("PLM_SUM_EXPORT"); d != "" {
					p := filepath.Join(d, fmt.Sprintf("source-%d", si), n)
					if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
						t.Fatal(e)
					}
					if e := os.WriteFile(p, b, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			parse := func() *hir.Module {
				h, e := plm.Compile(string(data))
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			original := hir.LowerModule(parse())
			if len(original.Funcs) != 1 {
				t.Fatal("callable count", len(original.Funcs))
			}
			name := original.Funcs[0].Name
			export("source.plm", data)
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
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
					if e != nil || a == nil || len(a.Errors) > 0 {
						t.Fatalf("assemble %v %+v", e, a)
					}
					export(mode.name+".a80", []byte(s.Assembly))
					export(mode.name+".mir", []byte(s.MIR2Module.Dump()))
					export(mode.name+".bin", a.Binary)
					info, _ := json.MarshalIndent(map[string]any{"options": mode.o, "functions": s.MIR2Module.Funcs, "allocation": s.Allocation, "symbols": a.Symbols}, "", "  ")
					export(mode.name+"-abi.json", info)
					f := s.MIR2Module.FuncByName(name)
					if f == nil || len(f.Contract.Params) != 2 || len(f.Contract.Returns) != 1 {
						t.Fatal("ABI")
					}
					// This exact leaf is expected not to spill; refuse unknown homes rather than exempt arbitrary RAM.
					for reg, loc := range s.Allocation.Locs {
						if loc.Kind == mir2.LocMem {
							if _, ok := a.Symbols[mir2.Z80SpillLabel(name, reg)]; ok {
								t.Fatal("unexpected spill", reg, loc)
							}
						}
					}
					raw, final := mir2.NewVM(hir.LowerModule(parse())), mir2.NewVM(s.MIR2Module)
					raw.MaxSteps = 0
					final.MaxSteps = 0
					raw.EnsureHeap(65536)
					final.EnsureHeap(65536)
					z := emulator.NewRemogattoZ80WithScreen()
					z.SetROMEnd(0) // Wrapped low addresses are RAM, not silently protected ROM.
					profiler := emulator.NewProfiler()
					z.SetProfiler(profiler)
					for i := range z.Memory() {
						v := byte(i*41 + 29)
						z.Memory()[i] = v
						raw.WriteHeap(int64(i), v)
						final.WriteHeap(int64(i), v)
					}
					z.LoadMemory(0x8000, a.Binary)
					target, ok := a.Symbols[name]
					if !ok {
						t.Fatal(name)
					}
					driver := uint16(0x7000)
					call := []byte{0xcd, byte(target), byte(target >> 8), 0x76}
					z.LoadMemory(driver, call)
					export(mode.name+"-call.bin", call)
					before := append([]byte(nil), z.Memory()...)
					allowed := map[int]bool{}
					minSP := uint16(0xf000)
					counts := 0
					reads, tstates := int64(0), int64(0)
					run := func(ptr uint16, seed, n int) {
						counts++
						want := byte(0)
						for i := -1; i <= 255; i++ {
							addr := uint16(int(ptr) + i)
							v := byte(seed + 73*i)
							z.Memory()[addr] = v
							allowed[int(addr)] = true
							raw.WriteHeap(int64(addr), v)
							final.WriteHeap(int64(addr), v)
							if i >= 0 && i < n {
								want += v
							}
						}
						args := []mir2.Value{{I: int64(ptr)}, {I: int64(n)}}
						for vi, vm := range []*mir2.VM{raw, final} {
							events := []mir2.VMDataAccess{}
							vm.ObserveDataAccess = func(v mir2.VMDataAccess) { events = append(events, v) }
							out, e := vm.Call(name, args)
							if e != nil || len(out) != 1 || out[0].I != int64(want) {
								t.Fatalf("VM%d ptr%x seed%d n%d got%v want%d err%v", vi, ptr, seed, n, out, want, e)
							}
							if len(events) != n {
								t.Fatalf("VM%d count ptr%x n%d got%d", vi, ptr, n, len(events))
							}
							for i, v := range events {
								if v.Op != mir2.OpLoad || v.Addr != int64(uint16(int(ptr)+i)) || v.Value != byte(seed+73*i) {
									t.Fatalf("VM%d read%d %+v", vi, i, v)
								}
							}
						}
						z.SetRegisters(emulator.Registers{PC: driver, SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
						for i, p := range f.Contract.Params {
							loc := s.Allocation.Locs[p.Reg]
							v := uint16(args[i].I)
							if mir2.NativeSpillWidth(p.Ty) > 1 {
								h := map[string][2]string{"HL": {"H", "L"}, "DE": {"D", "E"}, "BC": {"B", "C"}, "IX": {"IXH", "IXL"}, "IY": {"IYH", "IYL"}}[loc.Name]
								if h[0] == "" {
									t.Fatal(loc)
								}
								if e := z.SetRegister8(h[0], byte(v>>8)); e != nil {
									t.Fatal(e)
								}
								if e := z.SetRegister8(h[1], byte(v)); e != nil {
									t.Fatal(e)
								}
							} else if e := z.SetRegister8(loc.Name, byte(v)); e != nil {
								t.Fatal(e)
							}
						}
						seen := 0
						done := false
						for step := 0; step < 200000; step++ {
							if z.GetPC() == driver+3 && z.GetSP() == 0xf000 {
								done = true
								break
							}
							r := z.GetRegisters()
							op := z.Memory()[r.PC]
							addr := uint16(0)
							read := false
							switch {
							case op&0xc7 == 0x46 && op != 0x76 || op >= 0x80 && op <= 0xbf && op&7 == 6:
								addr = r.HL
								read = true
							case op == 0x1a:
								addr = r.DE
								read = true
							case op == 0x0a:
								addr = r.BC
								read = true
							case op == 0x3a:
								addr = uint16(z.Memory()[r.PC+1]) | uint16(z.Memory()[r.PC+2])<<8
								read = true
							case op == 0xdd || op == 0xfd:
								sub := z.Memory()[r.PC+1]
								if sub&0xc7 == 0x46 && sub != 0x76 || sub >= 0x80 && sub <= 0xbf && sub&7 == 6 {
									base := r.IX
									if op == 0xfd {
										base = r.IY
									}
									addr = uint16(int(base) + int(int8(z.Memory()[r.PC+2])))
									read = true
								}
							}
							if read && uint16(addr-ptr+1) <= 256 {
								if seen >= n || addr != uint16(int(ptr)+seen) || z.Memory()[addr] != byte(seed+73*seen) {
									t.Fatalf("native read ptr%x seed%d n%d index%d addr%x pc%x", ptr, seed, n, seen, addr, r.PC)
								}
								seen++
							}
							tstates += int64(z.Step())
							if z.GetSP() < minSP {
								minSP = z.GetSP()
							}
						}
						if !done {
							t.Fatalf("native CALL/SP ptr%x n%d %+v", ptr, n, z.GetRegisters())
						}
						if seen != n {
							t.Fatalf("native readcount ptr%x n%d got%d", ptr, n, seen)
						}
						reads += int64(seen)
						r := z.GetRegisters()
						ret := mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
						got := byte(0)
						switch ret {
						case "A":
							got = r.A
						case "B":
							got = byte(r.BC >> 8)
						case "C":
							got = byte(r.BC)
						case "D":
							got = byte(r.DE >> 8)
						case "E":
							got = byte(r.DE)
						default:
							t.Fatal("returnhome", ret)
						}
						if got != want {
							t.Fatalf("native ptr%x seed%d n%d got%d want%d", ptr, seed, n, got, want)
						}
						for i := -1; i <= 255; i++ {
							addr := uint16(int(ptr) + i)
							v := byte(seed + 73*i)
							if z.Memory()[addr] != v || raw.ReadHeap(int64(addr), 1)[0] != v || final.ReadHeap(int64(addr), 1)[0] != v {
								t.Fatalf("input/guard changed ptr%x index%d", ptr, i)
							}
						}
					}
					for seed := 0; seed < 256; seed++ {
						for n := 0; n < 256; n++ {
							run(0x4000, seed, n)
						}
					}
					for _, ptr := range []uint16{0xff00, 0xfffe, 0xffff} {
						for seed := 0; seed < 256; seed++ {
							for _, n := range []int{0, 1, 2, 127, 128, 255} {
								run(ptr, seed, n)
							}
						}
					}
					for addr, writes := range profiler.WriteCount {
						if writes != 0 && !(addr >= int(minSP) && addr < 0xf000) {
							t.Fatalf("native unexpected write addr%x count%d", addr, writes)
						}
					}
					for addr := 0; addr < 65536; addr++ {
						if !allowed[addr] {
							want := byte(addr*41 + 29)
							if raw.ReadHeap(int64(addr), 1)[0] != want || final.ReadHeap(int64(addr), 1)[0] != want {
								t.Fatalf("full VM RAM changed%x", addr)
							}
						}
					}
					for addr, v := range before {
						if allowed[addr] || addr >= int(minSP) && addr < 0xf000 {
							continue
						}
						if z.Memory()[addr] != v {
							t.Fatalf("fullRAM changed%x", addr)
						}
					}
					summary, _ := json.MarshalIndent(map[string]any{"source_path": src.path, "source_sha256": src.sha, "entry": name, "calls": counts, "ordered_native_reads": reads, "total_native_tstates": tstates, "module_bytes": len(a.Binary), "max_stack_bytes": 0xf000 - int(minSP), "spill_bytes": 0, "native_write_trace": "only observed CALL/push stack; all low wrapped addresses RAM", "full_native_ram": "unchanged except exact harness input setup and observed CALL stack"}, "", "  ")
					export(mode.name+"-results.json", summary)
					t.Log(string(summary))
				})
			}
		})
	}
}
