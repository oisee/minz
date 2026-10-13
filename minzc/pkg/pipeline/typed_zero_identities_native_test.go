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

func TestTypedZeroIdentitiesNative(t *testing.T) {
	source := ""
	entries := []struct{ name, op, ty string }{}
	for _, ty := range []string{"u8", "i8", "u16", "i16"} {
		for i, op := range []string{"|", "^", "-", "&", "*", "<<"} {
			n := fmt.Sprintf("identity_%s_%d", ty, i)
			source += fmt.Sprintf("fun %s(x:%s)->%s{return x %s %s(0)}\n", n, ty, ty, op, ty)
			entries = append(entries, struct{ name, op, ty string }{n, op, ty})
		}
	}
	source += "fun flag_control(x:u8)->u8{let p:bool=x<128\n let y:u8=x | 0\n if p {return y}\n return y ^ 255}\n"
	source += "fun zero_control(x:u8)->u8{if x==0{return x & 0}\n return x | 0}\n"
	entries = append(entries, struct{ name, op, ty string }{"flag_control", "control", "u8"}, struct{ name, op, ty string }{"zero_control", "zero-control", "u8"})
	export := func(n string, b []byte) {
		if d := os.Getenv("ZERO_IDENTITY_EXPORT"); d != "" {
			p := filepath.Join(d, n)
			if e := os.MkdirAll(filepath.Dir(p), 0755); e != nil {
				t.Fatal(e)
			}
			if e := os.WriteFile(p, b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	export("source.nanz", []byte(source))
	parse := func() *hir.Module {
		h, e := nanz.Parse(source, "typed-zero-original.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
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
				t.Fatalf("asm%v %+v", e, a)
			}
			export(mode.name+".a80", []byte(s.Assembly))
			export(mode.name+".bin", a.Binary)
			export(mode.name+".mir", []byte(s.MIR2Module.Dump()))
			raw, final := mir2.NewVM(hir.LowerModule(parse())), mir2.NewVM(s.MIR2Module)
			raw.MaxSteps = 0
			final.MaxSteps = 0
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			p := emulator.NewProfiler()
			z.SetProfiler(p)
			for i := range z.Memory() {
				z.Memory()[i] = byte(i*41 + 29)
			}
			z.LoadMemory(0x8000, a.Binary)
			before := append([]byte(nil), z.Memory()...)
			cost := map[string]int64{}
			counts := map[string]int{}
			minSP := uint16(0xf000)
			for _, entry := range entries {
				f := s.MIR2Module.FuncByName(entry.name)
				values := []uint16{0, 1, 2, 7, 127, 128, 254, 255, 256, 257, 32767, 32768, 65534, 65535}
				if f.Contract.Params[0].Ty.Width() == 8 {
					values = nil
					for i := 0; i < 256; i++ {
						values = append(values, uint16(i))
					}
				}
				for _, v := range values {
					want := v
					if entry.op == "&" || entry.op == "*" {
						want = 0
					}
					if entry.op == "control" && v >= 128 {
						want = v ^ 255
					}
					for _, vm := range []*mir2.VM{raw, final} {
						out, e := vm.Call(entry.name, []mir2.Value{{I: int64(v)}})
						if e != nil || len(out) != 1 || out[0].I != int64(want) {
							t.Fatal("VM", entry, v, out, want, e)
						}
					}
					driver := uint16(0x7000)
					target := a.Symbols[entry.name]
					z.LoadMemory(driver, []byte{0xcd, byte(target), byte(target >> 8), 0x76})
					z.SetRegisters(emulator.Registers{PC: driver, SP: 0xf000, A: 0x5a, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468})
					par := f.Contract.Params[0]
					loc := s.Allocation.Locs[par.Reg]
					if loc.Kind == mir2.LocMem {
						t.Fatal("unexpected paramspill")
					}
					if par.Ty.Width() == 16 {
						h := map[string][2]string{"HL": {"H", "L"}, "DE": {"D", "E"}, "BC": {"B", "C"}, "IX": {"IXH", "IXL"}, "IY": {"IYH", "IYL"}}[loc.Name]
						z.SetRegister8(h[0], byte(v>>8))
						z.SetRegister8(h[1], byte(v))
					} else {
						z.SetRegister8(loc.Name, byte(v))
					}
					done := false
					for i := 0; i < 1000; i++ {
						if z.GetPC() == driver+3 && z.GetSP() == 0xf000 {
							done = true
							break
						}
						cost[entry.name] += int64(z.Step())
						if z.GetSP() < minSP {
							minSP = z.GetSP()
						}
					}
					if !done {
						t.Fatal("CALLSP", entry)
					}
					r := z.GetRegisters()
					home := mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
					got := uint16(r.A)
					switch home {
					case "HL":
						got = r.HL
					case "DE":
						got = r.DE
					case "BC":
						got = r.BC
					case "A":
					case "C":
						got = uint16(byte(r.BC))
					default:
						t.Fatal(home)
					}
					if got != want {
						t.Fatal("native", entry, v, got, want)
					}
					counts[entry.name]++
				}
			}
			for addr, w := range p.WriteCount {
				if w > 0 && !(addr >= int(minSP) && addr < 0xf000) {
					t.Fatal("native writes", addr)
				}
			}
			for addr, v := range before {
				if addr >= 0x7000 && addr < 0x7004 || addr >= int(minSP) && addr < 0xf000 {
					continue
				}
				if z.Memory()[addr] != v {
					t.Fatal("RAMcode", addr)
				}
			}
			d, _ := json.MarshalIndent(map[string]any{"source_sha256": fmt.Sprintf("%x", sha256.Sum256([]byte(source))), "module_bytes": len(a.Binary), "calls": counts, "tstates": cost, "options": mode.o}, "", "  ")
			export(mode.name+"-results.json", d)
			t.Log(string(d))
		})
	}
}
