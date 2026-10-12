package pipeline

import (
	"crypto/sha256"
	"encoding/binary"
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

// Exact existing nativeCompositionFixtures sources. Frozen boundary lattice;
// all a/b bytes, never a claim of complete three/four-dimensional Cartesian.
func TestNativeCompositionBoundaryLattice(t *testing.T) {
	fixtures := []struct {
		name, src string
		arity     int
		model     func([]int64) byte
	}{
		{"a1", `fun a1(a: u8, b: u8, c: u8) -> u8 { let p: u8 = a + b
 return p - (c + a) }`, 3, func(v []int64) byte { return byte(v[1] - v[2]) }},
		{"a2", `fun a2(a: u8, b: u8, c: u8, d: u8) -> u8 { let p: u8 = a - b
 return p + (c - d) }`, 4, func(v []int64) byte { return byte(v[0] - v[1] + v[2] - v[3]) }},
		{"a4", `fun a4(a: u8, b: u8, c: u8) -> u8 { return c - ((a >> 1) - (b << 1)) }`, 3, func(v []int64) byte { return byte(v[2] - (v[0] >> 1) + (v[1] << 1)) }},
		{"a8", `fun a8(a: u8, b: u8, c: u8) -> u8 { return b - a + (c << 1) }`, 3, func(v []int64) byte { return byte(v[1] - v[0] + (v[2] << 1)) }}}
	bounds := []int64{0, 1, 127, 128, 254, 255}
	export := func(name string, b []byte) {
		if d := os.Getenv("COMPOSITION_BOUNDARY_EXPORT"); d != "" {
			p := filepath.Join(d, name)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, tc := range fixtures {
		t.Run(tc.name, func(t *testing.T) {
			export(tc.name+"/source.nanz", []byte(tc.src))
			parse := func() *hir.Module {
				h, e := nanz.Parse(tc.src, "critic.nanz")
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			for _, mode := range []struct {
				name string
				o    Options
			}{{"ordinary", Options{ContractOpt: true, AssertMode: "none"}}, {"grace-size", Options{ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}}} {
				t.Run(mode.name, func(t *testing.T) {
					s, e := CompileHIRSteps(parse(), mode.o)
					if e != nil {
						t.Fatal(e)
					}
					a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
					if e != nil || a == nil || len(a.Errors) > 0 {
						t.Fatalf("assembly %v %+v", e, a)
					}
					prefix := tc.name + "/" + mode.name
					export(prefix+".a80", []byte(s.Assembly))
					export(prefix+".bin", a.Binary)
					export(prefix+".mir", []byte(s.MIR2Module.Dump()))
					rm := hir.LowerModule(parse())
					export(prefix+"-raw.mir", []byte(rm.Dump()))
					raw, final := mir2.NewVM(rm), mir2.NewVM(s.MIR2Module)
					raw.MaxSteps = 0
					final.MaxSteps = 0
					f := s.MIR2Module.FuncByName(tc.name)
					if len(f.Contract.Params) != tc.arity || len(f.Contract.Returns) != 1 {
						t.Fatal("ABI")
					}
					z := emulator.NewRemogattoZ80WithScreen()
					z.SetROMEnd(0)
					prof := emulator.NewProfiler()
					z.SetProfiler(prof)
					for i := range z.Memory() {
						z.Memory()[i] = byte(i*41 + 29)
					}
					z.LoadMemory(0x8000, a.Binary)
					target := a.Symbols[tc.name]
					if target < 0x8000 {
						t.Fatal("target")
					}
					z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
					before := append([]byte(nil), z.Memory()...)
					homes := []string{}
					for _, p := range f.Contract.Params {
						l := s.Allocation.Locs[p.Reg]
						if l.Kind == mir2.LocMem || p.Ty.Width() != 8 {
							t.Fatal("param home", l)
						}
						homes = append(homes, l.Name)
					}
					home := mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
					codeText := ""
					for _, line := range strings.Split(s.Assembly, "\n") {
						codeText += strings.SplitN(line, ";", 2)[0] + "\n"
					}
					if strings.Contains(strings.ToUpper(codeText), "EXX") || strings.Contains(strings.ToUpper(codeText), "AF'") {
						t.Fatal("unexpected shadow bank use")
					}
					spills := map[int]bool{}
					for name, addr := range a.Symbols {
						if strings.HasPrefix(name, "_spill_") {
							spills[int(addr)] = true
						}
					}
					args := make([]int64, tc.arity)
					values := make([]mir2.Value, tc.arity)
					depth := uint16(0xf000)
					calls, cycles := int64(0), int64(0)
					digest := sha256.New()
					var record [8]byte
					domains := []map[string]int64{}
					for _, c := range bounds {
						ds := []int64{4}
						if tc.arity == 4 {
							ds = bounds
						}
						for _, d := range ds {
							start := cycles
							for x := int64(0); x < 256; x++ {
								for y := int64(0); y < 256; y++ {
									args[0], args[1], args[2] = x, y, c
									if tc.arity == 4 {
										args[3] = d
									}
									want := tc.model(args)
									for i, v := range args {
										values[i] = mir2.Value{I: v}
									}
									for _, vm := range []*mir2.VM{raw, final} {
										out, e := vm.Call(tc.name, values)
										if e != nil || len(out) != 1 || out[0].I != int64(want) {
											t.Fatal("VM", args, out, want, e)
										}
									}
									z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x97, F: 0x45, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
									for i, h := range homes {
										if e := z.SetRegister8(h, byte(args[i])); e != nil {
											t.Fatal(e)
										}
									}
									done := false
									for n := 0; n < 1000; n++ {
										if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
											done = true
											break
										}
										cycles += int64(z.Step())
										if z.GetSP() < depth {
											depth = z.GetSP()
										}
									}
									if !done || depth < 0xe000 {
										t.Fatal("CALL/SP", args)
									}
									r := z.GetRegisters()
									got := r.A
									switch home {
									case "A":
									case "B":
										got = byte(r.BC >> 8)
									case "C":
										got = byte(r.BC)
									case "D":
										got = byte(r.DE >> 8)
									case "E":
										got = byte(r.DE)
									case "H":
										got = byte(r.HL >> 8)
									case "L":
										got = byte(r.HL)
									default:
										t.Fatal("return", home)
									}
									if got != want {
										t.Fatal("native", args, got, want)
									}
									if !strings.Contains(codeText, "IX") && r.IX != 0x1357 {
										t.Fatal("IX")
									}
									if !strings.Contains(codeText, "IY") && r.IY != 0x2468 {
										t.Fatal("IY")
									}
									binary.LittleEndian.PutUint16(record[0:2], r.BC)
									binary.LittleEndian.PutUint16(record[2:4], r.DE)
									binary.LittleEndian.PutUint16(record[4:6], r.HL)
									record[6], record[7] = r.A, r.F
									digest.Write(record[:])
									calls++
								}
							}
							for addr, w := range prof.WriteCount {
								if w > 0 && !spills[addr] && !(addr >= int(depth) && addr < 0xf000) {
									t.Fatal("write authority", addr)
								}
							}
							for addr, v := range before {
								if spills[addr] || addr >= int(depth) && addr < 0xf000 {
									continue
								}
								if z.Memory()[addr] != v {
									t.Fatal("RAM/code", addr)
								}
							}
							domains = append(domains, map[string]int64{"c": c, "d": d, "calls": 65536, "T": cycles - start})
						}
					}
					b, _ := json.MarshalIndent(map[string]any{"source_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(tc.src))), "options": mode.o, "module_bytes": len(a.Binary), "calls": calls, "T_including_CALL": cycles, "stack_bytes": 0xf000 - depth, "parameter_homes": homes, "return_home": home, "spill_bytes": spills, "domains": domains, "register_output_sha256": fmt.Sprintf("%x", digest.Sum(nil))}, "", "  ")
					export(prefix+"-results.json", b)
					t.Logf("calls=%d T=%d bytes=%d stack=%d", calls, cycles, len(a.Binary), 0xf000-depth)
				})
			}
		})
	}
}
