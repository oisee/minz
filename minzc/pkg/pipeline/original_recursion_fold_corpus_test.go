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
	"testing"
)

func TestOriginalRecursionFoldCorpusComplete(t *testing.T) {
	// Frozen domain/oracles in reports/original-recursion-fold-corpus/frozen-domain.json.
	// Independent mathematical abs and remainder gcd; mixed zero source gcd never terminates.
	sources := []struct {
		key, path, sha string
		entries        []string
	}{
		{"ex9a_factorial_rec", "reports/showcase-src/2026-03-11/ex9a_factorial_rec.nanz", "49418af6de858d7388edf81bb74eadd2beb608b15c6f45c0b0d5eeae34f663f1", []string{"factorial"}},
		{"ex9b_factorial_fold", "reports/showcase-src/2026-03-11/ex9b_factorial_fold.nanz", "d3dad2dc322fc522c86d14cf5fdb87897b0e5a102c594c686c19ae76357846de", []string{"factorial_fold"}},
		{"ex10a_fib_rec", "reports/showcase-src/2026-03-11/ex10a_fib_rec.nanz", "396e859e7c49c19cd59d95354672582ffe9d99046c1f87c81dea5ac738b9670c", []string{"fib"}},
		{"ex10b_fib_iter", "reports/showcase-src/2026-03-11/ex10b_fib_iter.nanz", "afb172839734878c9697ecc295e20a62ff1e658bcc7be994c38b1b8962fa9615", []string{"fib_iter"}},
		{"ex10c_fib_fold", "reports/showcase-src/2026-03-11/ex10c_fib_fold.nanz", "14fe7eed3447aa6928d72411cb91b1f56ffc55bd87cd61d9e49f52d24435a26a", []string{"fib_fold"}},
		{"ex14_fold_assert", "reports/showcase-src/2026-03-11/ex14_fold_assert.nanz", "362b9106276974042e33936b02203f10a48cf726d24bd5e48eec1d4f75bae1a7", []string{"sum_to", "factorial", "count_in_range"}},
	}
	export := func(name string, b []byte) {
		if dir := os.Getenv("RECURSION_FOLD_EXPORT"); dir != "" {
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
					globalWidths := map[string]int{}
					for _, g := range s.MIR2Module.Globals {
						w := mir2.NativeSpillWidth(g.Ty)
						globalWidths[g.Name] = w
						addr, ok := a.Symbols[g.Name]
						if !ok {
							t.Fatal("globalsymbol", g.Name)
						}
						for i := 0; i < w; i++ {
							allowed[addr+i] = true
						}
					}
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
					run := func(name string, args []mir2.Value, want uint16) {
						counts[name]++
						vmBeforeGlobals := map[string][]byte{}
						nativeBeforeGlobals := map[string][]byte{}
						for g, w := range globalWidths {
							vmBeforeGlobals[g] = append([]byte(nil), raw.ReadHeap(raw.GlobalAddr(g), w)...)
							a := a.Symbols[g]
							nativeBeforeGlobals[g] = append([]byte(nil), z.Memory()[a:a+w]...)
						}
						f := s.MIR2Module.FuncByName(name)
						globals := map[string]uint16{}
						if name == "factorial_fold" {
							globals["acc"] = want
						}
						if source.key == "ex14_fold_assert" {
							if name == "factorial" {
								globals["acc_u16"] = want
							} else {
								globals["acc_u8"] = want
							}
						}
						for _, vm := range []*mir2.VM{raw, final} {
							events := []mir2.VMDataAccess{}
							vm.ObserveDataAccess = func(e mir2.VMDataAccess) { events = append(events, e) }
							v, e := vm.Call(name, args)
							if e != nil || len(v) != 1 || v[0].I != int64(want) {
								t.Fatalf("VM %s%v got%v want%d err%v", name, args, v, want, e)
							}
							seen := 0
							for _, e := range events {
								global := false
								for g, w := range globalWidths {
									a := vm.GlobalAddr(g)
									if e.Addr >= a && e.Addr < a+int64(w) {
										global = true
									}
								}
								if global {
									continue
								}
								if name != "count_in_range" || e.Op != mir2.OpLoad || e.Addr != 0x4000+int64(seen) || seen >= int(args[1].I) {
									t.Fatalf("VM unexpected/order access %s%v %+v", name, args, e)
								}
								seen++
							}
							if name == "count_in_range" && seen != int(args[1].I) {
								t.Fatalf("VM count read%d want%d", seen, args[1].I)
							}
							for g, before := range vmBeforeGlobals {
								if _, changes := globals[g]; !changes {
									for i, b := range before {
										if vm.ReadHeap(vm.GlobalAddr(g)+int64(i), 1)[0] != b {
											t.Fatalf("untouched VM global%s", g)
										}
									}
								}
							}
							for g, w := range globals {
								addr := vm.GlobalAddr(g)
								v := vm.ReadHeap(addr, globalWidths[g])
								got := uint16(v[0])
								if len(v) > 1 {
									got |= uint16(v[1]) << 8
								}
								if got != w {
									t.Fatalf("VM global%s got%d want%d", g, got, w)
								}
							}
						}
						z.SetRegisters(emulator.Registers{PC: drivers[name], SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
						for n, p := range f.Contract.Params {
							v := uint16(args[n].I)
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
						nativeSeen := 0
						done := false
						for n := 0; n < 20000000; n++ {
							if z.GetPC() == drivers[name]+3 && z.GetSP() == 0xf000 {
								done = true
								break
							}
							r := z.GetRegisters()
							op := z.Memory()[r.PC]
							if name == "count_in_range" && op&0xc7 == 0x46 && op != 0x76 && r.HL >= 0x3fff && r.HL <= 0x4100 {
								if nativeSeen >= int(args[1].I) || r.HL != 0x4000+uint16(nativeSeen) {
									t.Fatalf("native count read order %+v index%d", r, nativeSeen)
								}
								nativeSeen++
							}
							tstates += int64(z.Step())
							if z.GetSP() < callMin {
								callMin = z.GetSP()
							}
						}
						if name == "count_in_range" && nativeSeen != int(args[1].I) {
							t.Fatalf("native count read%d want%d", nativeSeen, args[1].I)
						}
						if !done {
							t.Fatalf("native CALL/SP %s%v %+v", name, args, z.GetRegisters())
						}
						if callMin < minSP {
							minSP = callMin
						}
						if d := int(0xf000 - callMin); d > depths[name] {
							depths[name] = d
						}
						if len(f.Contract.Returns) != 1 {
							t.Fatal("return ABI")
						}
						ret := f.Contract.Returns[0]
						got := readReg(mir2.ReturnLocation(ret.Class, ret.Ty))
						if mir2.NativeSpillWidth(ret.Ty) == 1 {
							got &= 255
						}
						if got != want {
							t.Fatalf("native %s%v got%d want%d regs%+v", name, args, got, want, z.GetRegisters())
						}
						for g, before := range nativeBeforeGlobals {
							if _, changes := globals[g]; !changes {
								addr := a.Symbols[g]
								for i, b := range before {
									if z.Memory()[addr+i] != b {
										t.Fatalf("untouched native global%s", g)
									}
								}
							}
						}
						for g, w := range globals {
							addr := a.Symbols[g]
							got := uint16(z.Memory()[addr])
							if globalWidths[g] > 1 {
								got |= uint16(z.Memory()[addr+1]) << 8
							}
							if got != w {
								t.Fatalf("nativeglobal%s got%d want%d", g, got, w)
							}
						}
					}
					factorial := func(n int) uint16 {
						v := uint16(1)
						for i := 2; i <= n; i++ {
							v *= uint16(i)
						}
						return v
					}
					fib := func(n int) uint16 {
						a, b := uint16(0), uint16(1)
						for i := 0; i < n; i++ {
							a, b = b, a+b
						}
						return a
					}
					for _, name := range source.entries {
						if name == "count_in_range" {
							continue
						}
						max := 255
						if name == "fib" {
							max = 20
						}
						for n := 0; n <= max; n++ {
							w := uint16(0)
							switch name {
							case "factorial":
								w = factorial(n)
								if source.key == "ex14_fold_assert" && n == 255 {
									w = 1
								}
							case "factorial_fold":
								w = factorial(n)
								if n == 255 {
									w = 1
								}
							case "fib", "fib_iter":
								w = fib(n)
							case "fib_fold":
								w = fib(n)
								if n == 0 {
									w = 1
								}
							case "sum_to":
								w = uint16(n*(n+1)/2) & 255
								if n == 255 {
									w = 0
								}
							}
							run(name, []mir2.Value{{I: int64(n)}}, w)
						}
					}
					if source.key == "ex14_fold_assert" {
						for _, seed := range []int{0, 1, 127, 255} {
							for n := 0; n < 256; n++ {
								for _, bounds := range [][2]int{{0, 255}, {0, 0}, {255, 255}, {127, 128}, {128, 127}, {1, 254}, {254, 1}, {73, 146}, {146, 73}, {255, 0}} {
									want := uint16(0)
									for i := -1; i <= 256; i++ {
										v := byte(seed + 73*i)
										z.Memory()[0x4000+i] = v
										raw.WriteHeap(int64(0x4000+i), v)
										final.WriteHeap(int64(0x4000+i), v)
										if i >= 0 && i < n && int(v) >= bounds[0] && int(v) <= bounds[1] {
											want++
										}
									}
									run("count_in_range", []mir2.Value{{I: 0x4000}, {I: int64(n)}, {I: int64(bounds[0])}, {I: int64(bounds[1])}}, want)
									for i := -1; i <= 256; i++ {
										if z.Memory()[0x4000+i] != byte(seed+73*i) {
											t.Fatal("native array changed")
										}
									}
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
						isGlobal := false
						for g, w := range globalWidths {
							a := raw.GlobalAddr(g)
							if int64(addr) >= a && int64(addr) < a+int64(w) {
								isGlobal = true
							}
						}
						if source.key == "ex14_fold_assert" && addr >= 0x3fff && addr <= 0x4100 {
							continue
						}
						if !isGlobal && (raw.ReadHeap(int64(addr), 1)[0] != v || final.ReadHeap(int64(addr), 1)[0] != v) {
							t.Fatalf("VM memory changed%x", addr)
						}
					}
					for addr, v := range before {
						if allowed[addr] || addr >= int(minSP) && addr < 0xf000 || source.key == "ex14_fold_assert" && addr >= 0x3fff && addr <= 0x4100 {
							continue
						}
						if z.Memory()[addr] != v {
							t.Fatalf("non-stack/spill RAM changed %04x", addr)
						}
					}
					summary, _ := json.MarshalIndent(map[string]any{"entries": counts, "module_bytes": len(a.Binary), "source_sha256": source.sha, "stack_depths": depths, "allowed_spill_bytes": allowed, "terminating_workload_tstates": tstates}, "", "  ")
					export(prefix+"-results.json", summary)
					t.Log(string(summary))
				})
			}
		})
	}
}
