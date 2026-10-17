package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPascalCPMMainAdmission(t *testing.T) {
	for _, name := range []string{"hello", "factorial"} {
		t.Run(name, func(t *testing.T) {
			s, e := os.ReadFile("../../../reports/pascal-cpm-main/" + name + ".pas")
			if e != nil {
				t.Fatal(e)
			}
			h, e := pascal.Compile(string(s), "examples/pascal/"+name+".pas")
			if e != nil {
				t.Fatal(e)
			}
			steps, e := CompileHIRSteps(h, Options{ContractOpt: true, Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024, AssertMode: "none"})
			if root := os.Getenv("MINZ_PASCAL_CPM_ARTIFACTS"); root != "" {
				p := filepath.Join(root, name)
				os.MkdirAll(p, 0755)
				b, _ := json.MarshalIndent(map[string]any{"diagnostic": implCPMErrorString(e), "module": steps.MIR2Module, "allocation": steps.Allocation}, "", "  ")
				os.WriteFile(filepath.Join(p, "before.json"), b, 0644)
				if steps.MIR2Module != nil {
					os.WriteFile(filepath.Join(p, "before.mir"), []byte(steps.MIR2Module.Dump()), 0644)
				}
			}
			if name == "hello" && e != nil {
				t.Fatal(e)
			}
			if e != nil {
				t.Fatal(e)
			}
			t.Log(e)
		})
	}
}

func TestPascalWholeMainCPMLoaderObservation(t *testing.T) { pascalCPMWhole(t, false) }
func TestPascalHelloProductionCPM(t *testing.T)            { pascalCPMWhole(t, true) }
func pascalCPMWhole(t *testing.T, production bool) {
	for _, mode := range []struct {
		name string
		o    Options
	}{{"ordinary", Options{ContractOpt: true}}, {"grace-size", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true}}} {
		if !production && mode.name != "ordinary" {
			continue
		}
		t.Run(mode.name, func(t *testing.T) { pascalCPMWholeMode(t, production, mode.name, mode.o) })
	}
}
func pascalCPMWholeMode(t *testing.T, production bool, mode string, opt Options) {
	pascalCPMProgramMode(t, production, mode, opt, "../../../reports/pascal-cpm-main", []struct{ name, want string }{{"hello", "Hello from Pascal on Z80!\r\n"}, {"factorial", "5040\r\n"}})
}
func pascalCPMProgramMode(t *testing.T, production bool, mode string, opt Options, frozenDir string, programs []struct{ name, want string }) {
	for _, program := range programs {

		t.Run(program.name, func(t *testing.T) {
			source, e := os.ReadFile(frozenDir + "/" + program.name + ".pas")
			if e != nil {
				t.Fatal(e)
			}
			actual, e := os.ReadFile("../../../examples/pascal/" + program.name + ".pas")
			if e != nil || !bytes.Equal(source, actual) {
				t.Fatal("source changed", e)
			}
			h, e := pascal.Compile(string(source), "examples/pascal/"+program.name+".pas")
			if e != nil {
				t.Fatal(e)
			}
			o := opt
			if production {
				o.Runtime = mir2.Z80RuntimeCPM
				o.RuntimeStackBytes = 1024
				o.AssertMode = "none"
			}
			steps, e := CompileHIRSteps(h, o)
			if e != nil {
				t.Fatal(e)
			}
			asm := steps.Assembly
			if !production {
				// The test loader calls main; the production runtime already
				// selected its own platform-wrapper contract during compilation.
				asm = "ORG 0x100\n__test_loader:\n LD SP,0xfdfe\n CALL main\n HALT\n" + calledMainTestAssembly(t, steps)
			}
			image, e := z80asm.NewAssembler().AssembleString(asm)
			if e != nil || len(image.Errors) > 0 {
				t.Fatal(e, image.Errors)
			}
			save := func(name string, v any) {
				if root := os.Getenv("MINZ_PASCAL_CPM_ARTIFACTS"); root != "" {
					p := filepath.Join(root, program.name, mode, name)
					os.MkdirAll(filepath.Dir(p), 0755)
					var b []byte
					switch x := v.(type) {
					case []byte:
						b = x
					case string:
						b = []byte(x)
					default:
						b, _ = json.MarshalIndent(v, "", "  ")
					}
					os.WriteFile(p, b, 0644)
				}
			}
			save("program.a80", asm)
			save("context.json", map[string]any{"options": o, "allocation": steps.Allocation, "symbols": image.Symbols, "source_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "functions": steps.MIR2Module.Funcs})
			save("raw.mir", steps.MIR2Raw)
			save("final.mir", steps.MIR2Module.Dump())
			save("program.com", image.Binary)
			for _, clobber := range []bool{false, true} {
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				mem := z.Memory()
				for p := range mem {
					mem[p] = byte(p*73 + 29)
				}
				z.LoadMemory(0x100, image.Binary)
				r := z.GetRegisters()
				r.PC = 0x100
				r.SP = 0xfdfe
				z.SetRegisters(r)
				if production {
					mem[0xfdfe] = 0
					mem[0xfdff] = 0
				}
				before := append([]byte(nil), mem...)
				out := []byte{}
				reads := []uint16{}
				minSP := r.SP
				done := false
				bdos := func() {
					rr := z.GetRegisters()
					switch byte(rr.BC) {
					case 2:
						out = append(out, byte(rr.DE))
					case 9:
						if program.name == "factorial" {
							t.Fatal("unexpected BDOS9 on numeric program")
						}
						addr := rr.DE
						literal := []byte(strings.TrimSuffix(program.want, "\r\n") + "$")
						if int(addr) < 0x100 || int(addr)+len(literal) > 0x100+len(image.Binary) || !bytes.Equal(mem[int(addr):int(addr)+len(literal)], literal) {
							t.Fatal("BDOS9 not exact source literal")
						}
						start := addr
						for i := 0; i < 256; i++ {
							if int(addr) < 0x100 || int(addr) >= 0x100+len(image.Binary) {
								t.Fatal("BDOS9outside literal", addr)
							}
							reads = append(reads, addr)
							if mem[addr] == '$' {
								if int(addr-start) != len(literal)-1 {
									t.Fatal("early dollar")
								}
								break
							}
							out = append(out, mem[addr])
							addr++
						}
					default:
						t.Fatal("unexpectedBDOS", rr.BC)
					}
					ret := uint16(mem[rr.SP]) | uint16(mem[rr.SP+1])<<8
					if clobber {
						rr.A = 0xa5
						rr.F = 0xff
						rr.BC = 0x1234
						rr.DE = 0x5678
						rr.HL = 0xc3c3
						rr.IX = 0x2468
						rr.IY = 0x1357
					}
					rr.PC = ret
					rr.SP += 2
					z.SetRegisters(rr)
				}
				for step := 0; step < 100000; step++ {
					pc := z.GetPC()
					if production && pc == 0 && z.GetSP() == 0xfe00 {
						done = true
						break
					}
					if pc == 5 {
						bdos()
						continue
					}
					if pc < 0x100 || int(pc) >= 0x100+len(image.Binary) {
						break
					}
					if mem[pc] == 0x76 {
						z.Step()
						done = true
						break
					}
					rr := z.GetRegisters()
					op := mem[pc]
					push := true
					word := rr.BC
					switch op {
					case 0xc5:
					case 0xd5:
						word = rr.DE
					case 0xe5:
						word = rr.HL
					case 0xf5:
						word = uint16(rr.A)<<8 | uint16(rr.F)
					case 0xcd:
						word = pc + 3
					default:
						push = false
					}
					if (op == 0xdd || op == 0xfd) && mem[pc+1] == 0xe5 {
						push = true
						if op == 0xdd {
							word = rr.IX
						} else {
							word = rr.IY
						}
					}
					z.Step()
					if push && (z.GetSP() != rr.SP-2 || mem[rr.SP-2] != byte(word) || mem[rr.SP-1] != byte(word>>8)) {
						t.Fatal("stackword", pc)
					}
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
				}
				mutable := map[uint16]bool{}
				if production {
					for _, extent := range [][2]int{{image.Symbols["__cpm_stack"], image.Symbols["__cpm_stack_top"]}, {image.Symbols["__cpm_saved_sp"], image.Symbols["__cpm_saved_sp"] + 2}, {image.Symbols["__cpm_status"], image.Symbols["__cpm_status"] + 1}} {
						for p := extent[0]; p < extent[1]; p++ {
							mutable[uint16(p)] = true
						}
					}
				}
				for p := minSP; !production && p < 0xfdfe; p++ {
					mutable[p] = true
				}
				for _, g := range steps.MIR2Module.Globals {
					start := image.Symbols[g.Name]
					for p := 0; p < mir2.ByteWidth(g.Ty); p++ {
						mutable[uint16(start+p)] = true
					}
				}
				for _, line := range image.Listing {
					if strings.HasPrefix(strings.ToLower(line.Label), "_spill_") {
						for j := range line.Bytes {
							mutable[uint16(line.Address+j)] = true
						}
					}
				}
				changes := []int{}
				for p, b := range before {
					if !mutable[uint16(p)] && mem[p] != b {
						changes = append(changes, p)
					}
				}
				save(fmt.Sprintf("clobber-%v.json", clobber), map[string]any{"stdout": string(out), "expected": program.want, "done": done, "registers": z.GetRegisters(), "cycles": z.GetCycles(), "min_sp": minSP, "BDOS9_reads": reads, "unexpected_RAM_changes": changes})
				sp := uint16(0xfdfe)
				if production {
					sp = 0xfe00
					if minSP < uint16(image.Symbols["__cpm_stack"]) || mem[uint16(image.Symbols["__cpm_stack_guard"])] != 0xa5 || mem[uint16(image.Symbols["__cpm_status"])] != 0 {
						t.Fatal("stack guard/status")
					}
				}
				if (production || !clobber) && (string(out) != program.want || !done || z.GetSP() != sp || len(changes) > 0) {
					t.Fatal("preserving loader", string(out), program.want, done, z.GetRegisters(), changes)
				}
				t.Logf("clobber%v stdout%q expected%q done%v SP%x", clobber, out, program.want, done, z.GetSP())
			}
		})
	}
}

func TestOriginalPascalFactorialPureModel(t *testing.T) {
	source, e := os.ReadFile("../../../reports/pascal-cpm-main/factorial.pas")
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []Options{{ContractOpt: true}, {ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true}, {ContractOpt: true, UseGrace: true, UseHIRGrace: true}} {
		h, e := pascal.Compile(string(source), "examples/pascal/factorial.pas")
		if e != nil {
			t.Fatal(e)
		}
		raw := hir.LowerModule(h)
		steps, e := CompileHIRSteps(h, o)
		if e != nil {
			t.Fatal(e)
		}
		for n := int64(0); n <= 12; n++ {
			want := int64(1)
			for j := int64(2); j <= n; j++ {
				want = want * j & 65535
			}
			signed := int64(int16(want))
			for _, m := range []*mir2.Module{raw, steps.MIR2Module} {
				got, e := mir2.NewVM(m).Call("FACT", []mir2.Value{{I: n}})
				if e != nil || len(got) != 1 || int64(int16(uint16(got[0].I))) != signed {
					t.Fatal("independent factorial", n, got, signed, e)
				}
			}
			h.Asserts = []hir.Assert{{FuncName: "FACT", Args: []int64{n}, Expected: int64(uint16(want))}}
			if e := RunAssertsZ80(h, steps.MIR2Module, steps.Allocation, steps.Assembly); e != nil {
				t.Fatal("native factorial", n, e)
			}
		}
	}
}
