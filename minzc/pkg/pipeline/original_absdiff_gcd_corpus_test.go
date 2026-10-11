package pipeline

import (
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
	"strings"
	"testing"
)

func TestOriginalAbsDiffGCDCorpusComplete(t *testing.T) {
	// Frozen domain/oracles in reports/original-absdiff-gcd-corpus/frozen-domain.json.
	// Independent mathematical abs and remainder gcd; mixed zero source gcd never terminates.
	sources := []struct {
		key, path, sha string
		entries        []string
	}{
		{"ex4a_abs_diff", "reports/showcase-src/2026-03-10/ex4a_abs_diff.nanz", "d200af0eb58a60d00262b54224da749ecb07a25ec15638ba03e4bf3f308ccf90", []string{"abs_diff"}},
		{"ex4b_abs_diff_u16", "reports/showcase-src/2026-03-10/ex4b_abs_diff_u16.nanz", "9ef0622b17529e2989b36a749291ee50f6facfbff17f7818c7c001e4348ffbca", []string{"abs_diff_u16"}},
		{"ex8_gcd", "reports/showcase-src/2026-03-10/ex8_gcd.nanz", "88747063a59526e797061cf828911c417e661723efb3b614715f2d2109977832", []string{"gcd"}},
	}
	bounds := []uint16{0, 1, 2, 7, 127, 128, 254, 255, 256, 257, 32767, 32768, 65534, 65535}
	export := func(name string, b []byte) {
		if dir := os.Getenv("ABSDIFF_GCD_EXPORT"); dir != "" {
			p := filepath.Join(dir, name)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
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
			if fmt.Sprintf("%x", sha256.Sum256(data)) != source.sha {
				t.Fatal("source changed")
			}
			export(source.key+"/source.nanz", data)
			if source.key != "tuple" {
				other, e := os.ReadFile(filepath.Join("../../../reports/showcase-src/2026-03-13", filepath.Base(source.path)))
				if e != nil || fmt.Sprintf("%x", sha256.Sum256(other)) != source.sha {
					t.Fatal("March13 duplicate differs", e)
				}
				export(source.key+"/march13-source.nanz", other)
			}
			parse := func() *hir.Module {
				h, e := nanz.Parse(string(data), source.path)
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			original := hir.LowerModule(parse())
			export(source.key+"/raw.mir", []byte(original.Dump()))
			if len(original.Funcs) != len(source.entries) {
				t.Fatal("callable count", len(original.Funcs))
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
					prefix := source.key + "/" + mode.name
					export(prefix+".a80", []byte(s.Assembly))
					export(prefix+".mir", []byte(s.MIR2Module.Dump()))
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
					if e != nil || a == nil || len(a.Errors) != 0 {
						t.Fatalf("assemble %v %+v", e, a)
					}
					export(prefix+".bin", a.Binary)
					metadata, _ := json.MarshalIndent(map[string]any{"functions": s.MIR2Module.Funcs, "allocation": s.Allocation, "options": mode.o, "symbols": a.Symbols}, "", "  ")
					export(prefix+"-abi.json", metadata)
					raw, final := mir2.NewVM(hir.LowerModule(parse())), mir2.NewVM(s.MIR2Module)
					z := emulator.NewRemogattoZ80WithScreen()
					z.SetROMEnd(0)
					profiler := emulator.NewProfiler()
					z.SetProfiler(profiler)
					raw.MaxSteps = 0
					final.MaxSteps = 0
					raw.EnsureHeap(65536)
					final.EnsureHeap(65536)
					for i := range z.Memory() {
						v := byte(i*73 + 29)
						z.Memory()[i] = v
						raw.WriteHeap(int64(i), v)
						final.WriteHeap(int64(i), v)
					}
					z.LoadMemory(0x8000, a.Binary)
					drivers := map[string]uint16{}
					for n, name := range source.entries {
						entry := uint16(0x7000 + n*4)
						target, ok := a.Symbols[name]
						if !ok {
							t.Fatal("entry missing", name)
						}
						code := []byte{0xcd, byte(target), byte(target >> 8), 0x76}
						z.LoadMemory(entry, code)
						drivers[name] = entry
						export(prefix+"-call-"+name+".bin", code)
					}
					before := append([]byte(nil), z.Memory()...)
					allowed := map[int]bool{}
					for _, f := range s.MIR2Module.Funcs {
						types := map[mir2.Reg]mir2.Ty{}
						for _, p := range f.Contract.Params {
							types[p.Reg] = p.Ty
						}
						for _, b := range f.Blocks {
							for _, i := range b.Insts {
								if i.Dst != mir2.NoReg {
									types[i.Dst] = i.Ty
								}
								for n, r := range i.ExtraRets {
									if n < len(i.ExtraRetTys) {
										types[r] = i.ExtraRetTys[n]
									}
								}
							}
						}
						for r, ty := range types {
							if slot, ok := a.Symbols[mir2.Z80SpillLabel(f.Name, r)]; ok {
								for off := 0; off < mir2.NativeSpillWidth(ty); off++ {
									allowed[slot+off] = true
								}
							}
						}
					}
					counts := map[string]int{}
					tstates := int64(0)
					zeroControls := 0
					depths := map[string]int{}
					minSP := uint16(0xf000)
					readReg := func(name string) uint16 {
						r := z.GetRegisters()
						switch name {
						case "A":
							return uint16(r.A)
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
						default:
							t.Fatal("unsupported return", name)
							return 0
						}
					}
					run := func(name string, x, y uint16, want []uint16) {
						counts[name]++
						f := s.MIR2Module.FuncByName(name)
						args := []mir2.Value{{I: int64(x)}, {I: int64(y)}}
						for _, vm := range []*mir2.VM{raw, final} {
							vm.ObserveDataAccess = func(v mir2.VMDataAccess) { t.Fatalf("unexpected VM memory access: %+v", v) }
							v, e := vm.Call(name, args)
							if e != nil || len(v) != len(want) {
								t.Fatalf("VM %s(%d,%d) got%v want%v err%v", name, x, y, v, want, e)
							}
							for n, w := range want {
								if v[n].I != int64(w) {
									t.Fatalf("VM %s(%d,%d) slot%d got%d want%d", name, x, y, n, v[n].I, w)
								}
							}
						}
						z.SetRegisters(emulator.Registers{PC: drivers[name], SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
						for n, p := range f.Contract.Params {
							v := []uint16{x, y}[n]
							loc := s.Allocation.Locs[p.Reg]
							if loc.Kind == mir2.LocMem {
								slot, ok := a.Symbols[mir2.Z80SpillLabel(name, p.Reg)]
								if !ok {
									t.Fatal("spill missing", p)
								}
								z.Memory()[slot] = byte(v)
								if mir2.NativeSpillWidth(p.Ty) > 1 {
									z.Memory()[slot+1] = byte(v >> 8)
								}
								continue
							}
							if mir2.NativeSpillWidth(p.Ty) > 1 {
								halves := map[string][2]string{"HL": {"H", "L"}, "DE": {"D", "E"}, "BC": {"B", "C"}, "IX": {"IXH", "IXL"}, "IY": {"IYH", "IYL"}}[loc.Name]
								if halves[0] == "" {
									t.Fatal(loc)
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
						callMin := uint16(0xf000)
						done := false
						for n := 0; n < 200000; n++ {
							if z.GetPC() == drivers[name]+3 && z.GetSP() == 0xf000 {
								done = true
								break
							}
							tstates += int64(z.Step())
							if z.GetSP() < callMin {
								callMin = z.GetSP()
							}
						}
						if !done {
							t.Fatalf("native CALL/SP %s(%d,%d) %+v", name, x, y, z.GetRegisters())
						}
						if callMin < minSP {
							minSP = callMin
						}
						if d := int(0xf000 - callMin); d > depths[name] {
							depths[name] = d
						}
						if len(f.Contract.Returns) != len(want) {
							t.Fatal("tuple ABI count", name)
						}
						for n, w := range want {
							ret := f.Contract.Returns[n]
							got := readReg(mir2.ReturnLocation(ret.Class, ret.Ty))
							if mir2.NativeSpillWidth(ret.Ty) == 1 {
								got &= 255
							}
							if got != w {
								t.Fatalf("native %s(%d,%d) slot%d got%d want%d regs%+v", name, x, y, n, got, w, z.GetRegisters())
							}
						}
					}
					name := source.entries[0]
					pair := func(x, y uint16) {
						w := uint16(0)
						if name == "gcd" {
							a, b := x, y
							for b != 0 {
								a, b = b, a%b
							}
							w = a
						} else {
							if x >= y {
								w = x - y
							} else {
								w = y - x
							}
						}
						run(name, x, y, []uint16{w})
					}
					for x := 0; x < 256; x++ {
						for y := 0; y < 256; y++ {
							if name == "gcd" && (x == 0) != (y == 0) {
								continue
							}
							pair(uint16(x), uint16(y))
						}
					}
					if name == "abs_diff_u16" {
						for _, x := range bounds {
							for _, y := range bounds {
								pair(x, y)
							}
						}
					}
					if name == "gcd" {
						for v := 1; v < 256; v++ {
							for orientation := 0; orientation < 2; orientation++ {
								x, y := uint16(0), uint16(v)
								if orientation == 1 {
									x, y = y, x
								}
								zeroControls++
								for _, m := range []*mir2.Module{hir.LowerModule(parse()), s.MIR2Module} {
									vm := mir2.NewVM(m)
									vm.MaxSteps = 1000
									if out, e := vm.Call(name, []mir2.Value{{I: int64(x)}, {I: int64(y)}}); e == nil || !strings.Contains(e.Error(), "gas limit reached") {
										t.Fatalf("mixed-zero expected gas limit (%d,%d):%v err%v", x, y, out, e)
									}
								}
								f := s.MIR2Module.FuncByName(name)
								z.SetRegisters(emulator.Registers{PC: drivers[name], SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
								for n, p := range f.Contract.Params {
									loc := s.Allocation.Locs[p.Reg]
									if loc.Kind == mir2.LocMem {
										t.Fatal("zero control unexpected spill")
									}
									if e := z.SetRegister8(loc.Name, byte([]uint16{x, y}[n])); e != nil {
										t.Fatal(e)
									}
								}
								for i := 0; i < 1000; i++ {
									if z.GetPC() == drivers[name]+3 {
										t.Fatalf("mixed-zero native unexpectedly returned (%d,%d)", x, y)
									}
									z.Step()
									if z.GetSP() < minSP {
										minSP = z.GetSP()
									}
								}
								if z.GetSP() != 0xeffe {
									t.Fatalf("mixedzero SP %x", z.GetSP())
								}
							}
						}
					}
					for _, name := range source.entries {
						if counts[name] == 0 {
							t.Fatal("entry not covered", name)
						}
					}
					for addr, writes := range profiler.WriteCount {
						if writes != 0 && !allowed[addr] && !(addr >= int(minSP) && addr < 0xf000) {
							t.Fatalf("native write outside stack/spill%x", addr)
						}
					}
					for addr := 0; addr < 65536; addr++ {
						v := byte(addr*73 + 29)
						if raw.ReadHeap(int64(addr), 1)[0] != v || final.ReadHeap(int64(addr), 1)[0] != v {
							t.Fatalf("VM memory changed%x", addr)
						}
					}
					for addr, v := range before {
						if allowed[addr] || addr >= int(minSP) && addr < 0xf000 {
							continue
						}
						if z.Memory()[addr] != v {
							t.Fatalf("non-stack/spill RAM changed %04x", addr)
						}
					}
					summary, _ := json.MarshalIndent(map[string]any{"entries": counts, "module_bytes": len(a.Binary), "source_sha256": source.sha, "stack_depths": depths, "allowed_spill_bytes": allowed, "terminating_workload_tstates": tstates, "mixed_zero_nonreturn_controls": zeroControls}, "", "  ")
					export(prefix+"-results.json", summary)
					t.Log(string(summary))
				})
			}
		})
	}
}
