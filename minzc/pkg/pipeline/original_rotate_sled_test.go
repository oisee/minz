package pipeline

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalRotateSledNative(t *testing.T) {
	path := "examples/nanz/16_rotate_sled.nanz"
	b, e := os.ReadFile("../../../" + path)
	if e != nil {
		t.Fatal(e)
	}
	export := func(name string, b []byte) {
		if d := os.Getenv("ROTATE_SLED_EXPORT"); d != "" {
			p := filepath.Join(d, name)
			os.MkdirAll(filepath.Dir(p), 0755)
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	for _, mode := range []struct {
		name string
		o    Options
	}{{"ordinary", Options{ContractOpt: true, AssertMode: "none"}}, {"grace-size", Options{ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}}} {
		t.Run(mode.name, func(t *testing.T) {
			h, e := nanz.Parse(string(b), path)
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, mode.o)
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil || a == nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			export(mode.name+".a80", []byte(s.Assembly))
			export(mode.name+".bin", a.Binary)
			export(mode.name+".mir", []byte(s.MIR2Module.Dump()))
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			prof := emulator.NewProfiler()
			z.SetProfiler(prof)
			for n := range z.Memory() {
				z.Memory()[n] = byte(n*41 + 29)
			}
			z.LoadMemory(0x8000, a.Binary)
			before := append([]byte(nil), z.Memory()...)
			out := []byte{}
			ports := []uint16{}
			z.SetIOHandlers(func(p uint16) byte { t.Fatal("unexpectedIN", p); return 0 }, func(p uint16, v byte) {
				ports = append(ports, p)
				if byte(p) != 0x23 || byte(p>>8) != v {
					t.Fatal("OUT", p, v)
				}
				out = append(out, v)
			})
			costs := map[string]int64{}
			mismatches := map[string][]string{}
			depth := uint16(0xf000)
			call := func(f *mir2.Func, x byte) byte {
				target := a.Symbols[f.Name]
				z.LoadMemory(0x7000, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
				z.SetRegisters(emulator.Registers{PC: 0x7000, SP: 0xf000, A: 0x97, F: 0x45, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
				if len(f.Contract.Params) > 0 {
					if e := z.SetRegister8(s.Allocation.Locs[f.Contract.Params[0].Reg].Name, x); e != nil {
						t.Fatal(e)
					}
				}
				for n := 0; n < 10000; n++ {
					if f.Name == "main" && z.Memory()[z.GetPC()] == 0x76 && z.GetSP() == 0xeffe {
						return 0
					}
					if z.GetPC() == 0x7003 && z.GetSP() == 0xf000 {
						if z.GetRegisters().IX != 0x1357 || z.GetRegisters().IY != 0x2468 {
							t.Fatal("indices")
						}
						home := "A"
						if len(f.Contract.Returns) > 0 {
							home = mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
						}
						rr := z.GetRegisters()
						v := rr.A
						switch home {
						case "A":
						case "B":
							v = byte(rr.BC >> 8)
						case "C":
							v = byte(rr.BC)
						case "D":
							v = byte(rr.DE >> 8)
						case "E":
							v = byte(rr.DE)
						case "H":
							v = byte(rr.HL >> 8)
						case "L":
							v = byte(rr.HL)
						default:
							t.Fatal("returnhome", home)
						}
						return v
					}
					costs[f.Name] += int64(z.Step())
					if z.GetSP() < depth {
						depth = z.GetSP()
					}
				}
				t.Fatal("CALL/SP", f.Name)
				return 0
			}
			for _, f := range s.MIR2Module.Funcs {
				if f.Name == "main" {
					continue
				}
				for n := 0; n < 256; n++ {
					x := byte(n)
					want := x
					switch f.Name {
					case "swap_nibbles":
						want = x<<4 | x>>4
					case "rol1":
						want = x<<1 | x>>7
					case "rol3":
						want = x<<3 | x>>5
					case "high_nibble":
						want = x >> 4
					case "low_nibble":
						want = x & 15
					default:
						t.Fatal("model", f.Name)
					}
					got := call(f, x)
					if got != want {
						mismatches[f.Name] = append(mismatches[f.Name], fmt.Sprintf("x=%02x got=%02x want=%02x", x, got, want))
					}
				}
			}
			out = nil
			ports = nil
			for _, f := range s.MIR2Module.Funcs {
				if f.Name == "main" {
					call(f, 0)
				}
			}
			if string(out) != "1861011" {
				mismatches["main"] = []string{fmt.Sprintf("got=%q want=1861011", out)}
			}
			for n := range z.Memory() {
				if n >= 0x7000 && n <= 0x7003 {
					continue
				}
				if n >= int(depth) && n < 0xf000 {
					continue
				}
				if z.Memory()[n] != before[n] || prof.WriteCount[n] != 0 {
					t.Fatal("memorywrite", n)
				}
			}
			result := map[string]any{"bytes": len(a.Binary), "calls": 1281, "costs_t_states": costs, "min_sp": depth, "stdout": string(out), "ports": ports, "mismatches": mismatches}
			j, _ := json.MarshalIndent(result, "", "  ")
			export(mode.name+".json", j)
			t.Logf("original mismatched helpers=%d stdout=%q", len(mismatches), out)
			if len(mismatches) > 0 {
				t.Fatal("native model", mismatches)
			}
		})
	}
}
