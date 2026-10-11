package pipeline

import (
	"encoding/binary"
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

func TestOriginalPLMAbsDiffFibComplete(t *testing.T) {
	// Independent PL/M BYTE arithmetic model: 8-bit unsigned values;
	// FIB uses the mathematical recurrence reduced modulo256.

	source, e := os.ReadFile("../../../examples/plm/abs_diff_fib.plm")
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		h, e := plm.Compile(string(source))
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	export := func(name string, b []byte) {
		if dir := os.Getenv("PLM_ABSDIFF_FIB_EXPORT"); dir != "" {
			os.MkdirAll(dir, 0755)
			if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	export("source.plm", source)
	original := hir.LowerModule(parse())
	export("raw.mir", []byte(original.Dump()))
	if len(original.Funcs) != 2 {
		t.Fatal("unexpected callable count", len(original.Funcs))
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
			raw, final := mir2.NewVM(hir.LowerModule(parse())), mir2.NewVM(s.MIR2Module)
			bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
			if e != nil {
				t.Fatal(e)
			}
			if len(bin.Errors) > 0 {
				t.Fatal(bin.Errors)
			}
			export(mode.name+".a80", []byte(s.Assembly))
			export(mode.name+".bin", bin.Binary)
			export(mode.name+".mir", []byte(s.MIR2Module.Dump()))
			info, _ := json.MarshalIndent(s.MIR2Module.Funcs, "", "  ")
			export(mode.name+"-functions.json", info)
			z := emulator.NewRemogattoZ80()
			for i := range z.Memory() {
				z.Memory()[i] = byte((i*73 + 29) % 256)
			}
			z.LoadMemory(0x8000, bin.Binary)
			drivers := map[string]uint16{}
			for i, f := range s.MIR2Module.Funcs {
				entry := uint16(0x7000 + i*4)
				drivers[f.Name] = entry
				code := []byte{0xcd, byte(bin.Symbols[f.Name]), byte(bin.Symbols[f.Name] >> 8), 0x76}
				z.LoadMemory(entry, code)
				export(mode.name+"-call-"+f.Name+".bin", code)
			}
			beforeRAM := append([]byte(nil), z.Memory()...)
			minSP := uint16(0xf000)
			stackDepth := map[string]int{}
			counts := map[string]int{}
			tstates := map[string]int64{}
			run := func(name string, args []int64, want uint16) {
				counts[name]++
				f := s.MIR2Module.FuncByName(name)
				if f == nil {
					t.Fatal(name)
				}
				for _, vm := range []*mir2.VM{raw, final} {
					vv := []mir2.Value{}
					for _, a := range args {
						vv = append(vv, mir2.Value{I: a})
					}
					out, e := vm.Call(name, vv)
					if e != nil || len(out) != 1 || uint16(out[0].I) != want {
						t.Fatalf("VM %s args%v got%v want%x err%v", name, args, out, want, e)
					}
				}
				z.SetRegisters(emulator.Registers{A: 0x5a, BC: 0x1234, DE: 0x5678, HL: 0x9abc, IX: 0x1357, IY: 0x2468, PC: drivers[name], SP: 0xf000})
				for i, p := range f.Contract.Params {
					loc := s.Allocation.Locs[p.Reg]
					v := uint16(args[i])
					if loc.Kind == mir2.LocMem {
						a := bin.Symbols[mir2.Z80SpillLabel(name, p.Reg)]
						if a == 0 {
							t.Fatal("param spill missing", name, p)
						}
						binary.LittleEndian.PutUint16(z.Memory()[a:a+2], v)
						continue
					}
					switch loc.Name {
					case "HL", "DE", "BC", "IX", "IY":
						hi, lo := map[string][2]string{"HL": {"H", "L"}, "DE": {"D", "E"}, "BC": {"B", "C"}, "IX": {"IXH", "IXL"}, "IY": {"IYH", "IYL"}}[loc.Name][0], map[string][2]string{"HL": {"H", "L"}, "DE": {"D", "E"}, "BC": {"B", "C"}, "IX": {"IXH", "IXL"}, "IY": {"IYH", "IYL"}}[loc.Name][1]
						if e := z.SetRegister8(hi, byte(v>>8)); e != nil {
							t.Fatal(e)
						}
						if e := z.SetRegister8(lo, byte(v)); e != nil {
							t.Fatal(e)
						}
					default:
						if e := z.SetRegister8(loc.Name, byte(v)); e != nil {
							t.Fatal(e, loc)
						}
					}
				}
				callMin := uint16(0xf000)
				for n := 0; n < 100000 && z.GetPC() != drivers[name]+3; n++ {
					tstates[name] += int64(z.Step())
					if sp := z.GetSP(); sp < callMin {
						callMin = sp
					}
				}

				if callMin < 0xefe0 {
					t.Fatalf("unbounded stack descent %x", callMin)
				}
				if callMin < minSP {
					minSP = callMin
				}
				if depth := int(0xf000 - callMin); depth > stackDepth[name] {
					stackDepth[name] = depth
				}
				r := z.GetRegisters()
				ret := mir2.ReturnLocation(f.Contract.Returns[0].Class, f.Contract.Returns[0].Ty)
				got := uint16(0)
				switch ret {
				case "A":
					got = uint16(r.A)
				case "C":
					got = uint16(byte(r.BC))
				case "HL":
					got = r.HL
				case "DE":
					got = r.DE
				case "BC":
					got = r.BC
				default:
					t.Fatal("unsupported ABI", ret, f.Contract)
				}
				if f.Contract.Returns[0].Ty.Width() == 8 {
					got &= 255
				}
				if got != want || r.PC != drivers[name]+3 || r.SP != 0xf000 {
					t.Fatalf("native %s args%v got%x want%x regs%v\n%s", name, args, got, want, r, s.Assembly)
				}
			}
			a, b := 0, 1
			for x := 0; x < 256; x++ {
				run("FIB", []int64{int64(x)}, uint16(a))
				a, b = b, (a+b)&255
				for y := 0; y < 256; y++ {
					difference := x - y
					if difference < 0 {
						difference = -difference
					}
					run("ABS_DIFF", []int64{int64(x), int64(y)}, uint16(difference))
				}
			}

			for _, f := range original.Funcs {
				if counts[f.Name] == 0 {
					t.Fatal("original entry not covered", f.Name)
				}
			}
			mutable := map[int]bool{}
			for _, f := range s.MIR2Module.Funcs {
				for reg, loc := range s.Allocation.Locs {
					if loc.Kind != mir2.LocMem {
						continue
					}
					symbol := mir2.Z80SpillLabel(f.Name, reg)
					if a, ok := bin.Symbols[symbol]; ok {
						mutable[a] = true
						mutable[a+1] = true
					}
				}
			}
			for addr := 0x8000; addr < 0x8000+len(bin.Binary); addr++ {
				if z.Memory()[addr] != beforeRAM[addr] {
					t.Fatalf("code changed %x", addr)
				}
			}
			for addr, original := range beforeRAM {
				if mutable[addr] || addr >= int(minSP) && addr < 0xf000 {
					continue
				}
				if z.Memory()[addr] != original {
					t.Fatalf("full64KiB RAM changed %x old%x new%x", addr, original, z.Memory()[addr])
				}
			}

			summary, _ := json.MarshalIndent(map[string]any{"entries": counts, "t_states_including_call": tstates, "bytes": len(bin.Binary), "stack_depth_per_entry": stackDepth, "stack_low": fmt.Sprintf("%04x", minSP), "allowed_generated_spill_bytes": mutable}, "", "  ")
			export(mode.name+"-summary.json", summary)
			t.Log(string(summary))
		})
	}
}
