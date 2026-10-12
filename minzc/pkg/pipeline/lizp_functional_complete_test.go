package pipeline

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"

	"os"
	"path/filepath"
	"testing"
)

func TestOriginalLizpFunctionalComplete(t *testing.T) {
	// Lizp original fn/let*/case semantics plus Book v7 numeric u8 width.
	// Independent host arithmetic and case model; lambdas here capture no locals.
	source, e := os.ReadFile("../../../examples/lizp/functional.lizp")
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		h, e := lizp.Compile(string(source), "functional.lizp")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	export := func(name string, b []byte) {
		if dir := os.Getenv("LIZP_FUNCTIONAL_EXPORT"); dir != "" {
			os.MkdirAll(dir, 0755)
			if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	export("source.lizp", source)
	original := hir.LowerModule(parse())
	export("raw.mir", []byte(original.Dump()))
	if len(original.Funcs) != 8 {
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
			indirectTargets := map[string]int{}
			counts := map[string]int{}
			run := func(name string, args []int64, want uint16) {
				counts[name]++
				f := s.MIR2Module.FuncByName(name)
				if f == nil {
					t.Fatal(name)
				}
				for _, vm := range []*mir2.VM{raw, final} {
					vv := []mir2.Value{}
					for i, a := range args {
						if name == "apply" && i == 0 {
							target := ""
							for _, n := range []string{"lambda_0", "lambda_1", "lambda_2"} {
								if int64(bin.Symbols[n]) == a {
									target = n
								}
							}
							if target == "" {
								t.Fatal("unbound indirect target", a)
							}
							a = vm.GlobalAddr(target)
						}
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
				entered := map[string]int{}
				for n := 0; n < 100000 && z.GetPC() != drivers[name]+3; n++ {
					for _, target := range []string{"apply", "lambda_0", "lambda_1", "lambda_2"} {
						if z.GetPC() == uint16(bin.Symbols[target]) {
							entered[target]++
							indirectTargets[target]++
						}
					}
					z.Step()
					if sp := z.GetSP(); sp < callMin {
						callMin = sp
					}
				}
				expectedTarget := ""
				if name == "apply" {
					for _, target := range []string{"lambda_0", "lambda_1", "lambda_2"} {
						if args[0] == int64(bin.Symbols[target]) {
							expectedTarget = target
						}
					}
				}
				if name == "test_double" {
					expectedTarget = "lambda_0"
				}
				if name == "transform" && args[0] == 0 {
					expectedTarget = "lambda_1"
				}
				if name == "transform" && args[0] == 1 {
					expectedTarget = "lambda_2"
				}
				if expectedTarget != "" && (entered["apply"] != 1 || entered[expectedTarget] != 1) {
					t.Fatalf("indirect path bypassed name%s args%v entered%v", name, args, entered)
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
			for a := 0; a < 256; a++ {
				run("weekday", []int64{int64(a)}, func() uint16 {
					switch a {
					case 0:
						return 77
					case 1:
						return 84
					case 2:
						return 87
					}
					return 63
				}())
				for b := 0; b < 256; b++ {
					run("pythag_sq", []int64{int64(a), int64(b)}, uint16(byte(a*a+b*b)))
					want := b
					if a == 0 {
						want = 2 * b
					} else if a == 1 {
						want = b + 10
					}
					run("transform", []int64{int64(a), int64(b)}, uint16(byte(want)))
				}
			}
			run("test_double", nil, 10)
			for _, name := range []string{"lambda_0", "lambda_1", "lambda_2"} {
				for x := 0; x < 256; x++ {
					want := byte(2 * x)
					if name == "lambda_2" {
						want = byte(x + 10)
					}
					run(name, []int64{int64(x)}, uint16(want))
					run("apply", []int64{int64(bin.Symbols[name]), int64(x)}, uint16(want))
				}
			}
			indirect := false
			for _, b := range s.MIR2Module.FuncByName("apply").Blocks {
				for _, i := range b.Insts {
					if i.Op == mir2.OpCallIndirect {
						indirect = true
					}
				}
			}
			if !indirect || !strings.Contains(s.Assembly, "JP __call_ix") {
				t.Fatal("original indirect call lost", s.Assembly)
			}
			if !strings.Contains(s.Assembly, "CALL __mul8") {
				t.Fatal("actual arithmetic helper call lost")
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
			for addr, original := range beforeRAM {
				if mutable[addr] || addr >= int(minSP) && addr < 0xf000 {
					continue
				}
				if z.Memory()[addr] != original {
					t.Fatalf("full64KiB RAM changed %x old%x new%x", addr, original, z.Memory()[addr])
				}
			}

			summary, _ := json.MarshalIndent(map[string]any{"entries": counts, "bytes": len(bin.Binary), "stack_depth_per_entry": stackDepth, "stack_low": fmt.Sprintf("%04x", minSP), "allowed_generated_spill_bytes": mutable, "executed_indirect_entries": indirectTargets}, "", "  ")
			export(mode.name+"-summary.json", summary)
			t.Log(string(summary))
		})
	}
}
