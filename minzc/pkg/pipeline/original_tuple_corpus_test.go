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

func TestOriginalTupleCorpusComplete(t *testing.T) {
	// Frozen domain/oracles in reports/original-tuple-corpus/frozen-domain.json.
	// Book v7 §7.5: check every returned element, including unused projections.
	sources := []struct {
		key, path, sha string
		entries        []string
	}{
		{"tuple", "examples/nanz/tuple_assert_gate.nanz", "4d4456e75beb9dcb43df6bc1483a3b8c9fa2b3c8dfe9d4f68c3b6679abcde6d8", []string{"divmod"}},
		{"ex11", "reports/showcase-src/2026-03-11/ex11_minmax_multiret.nanz", "f7f3be929afe1d2ce0b4212b46b004bfeb9a562780ff9b54b9eef2a7efcf7326", []string{"minmax", "swap", "smaller", "larger"}},
		{"ex13", "reports/showcase-src/2026-03-11/ex13_multiret_assert.nanz", "6677d09ec89a8489a9691bd4caa184ef88149c370b87fe543dc4b3e1432270b7", []string{"minmax", "swap", "min_of", "max_of"}},
	}
	bounds := []uint16{0, 1, 2, 7, 127, 128, 254, 255, 256, 257, 32767, 32768, 65534, 65535}
	export := func(name string, b []byte) {
		if dir := os.Getenv("TUPLE_CORPUS_EXPORT"); dir != "" {
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
					z := emulator.NewRemogattoZ80()
					for i := range z.Memory() {
						z.Memory()[i] = byte(i*73 + 29)
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
							z.Step()
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
					if source.key == "tuple" {
						for x := 0; x < 256; x++ {
							for y := 1; y < 256; y++ {
								run("divmod", uint16(x), uint16(y), []uint16{uint16(x / y), uint16(x % y)})
							}
						}
					} else {
						pair := func(x, y uint16) {
							lo, hi := x, y
							if x > y {
								lo, hi = y, x
							}
							for _, name := range source.entries {
								var w []uint16
								switch name {
								case "minmax":
									w = []uint16{lo, hi}
								case "swap":
									w = []uint16{y, x}
								case "smaller", "min_of":
									w = []uint16{lo}
								case "larger", "max_of":
									w = []uint16{hi}
								}
								run(name, x, y, w)
							}
						}
						for x := 0; x < 256; x++ {
							for y := 0; y < 256; y++ {
								pair(uint16(x), uint16(y))
							}
						}
						for _, x := range bounds {
							for _, y := range bounds {
								pair(x, y)
							}
						}
					}
					for _, name := range source.entries {
						if counts[name] == 0 {
							t.Fatal("entry not covered", name)
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
					summary, _ := json.MarshalIndent(map[string]any{"entries": counts, "module_bytes": len(a.Binary), "source_sha256": source.sha, "stack_depths": depths, "allowed_spill_bytes": allowed}, "", "  ")
					export(prefix+"-results.json", summary)
					t.Log(string(summary))
				})
			}
		})
	}
}
