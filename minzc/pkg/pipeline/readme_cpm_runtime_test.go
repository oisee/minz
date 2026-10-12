package pipeline

import (
	"bytes"
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

func TestReadmeCPMRuntimeAdmission(t *testing.T) {
	for _, name := range []string{"hello_cpm", "hello_cpm_fib"} {
		t.Run(name, func(t *testing.T) {
			s, e := os.ReadFile("../../../examples/nanz/" + name + ".nanz")
			if e != nil {
				t.Fatal(e)
			}
			h, e := nanz.Parse(string(s), "examples/nanz/"+name+".nanz")
			if e != nil {
				t.Fatal(e)
			}
			o := Options{ContractOpt: true, Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024, AssertMode: "none"}
			steps, e := CompileHIRSteps(h, o)
			if dir := os.Getenv("MINZ_README_CPM_ARTIFACTS"); dir != "" {
				dir = filepath.Join(dir, name)
				os.MkdirAll(dir, 0755)
				b, _ := json.MarshalIndent(map[string]any{"diagnostic": implCPMErrorString(e), "options": o, "module": steps.MIR2Module, "allocation": steps.Allocation}, "", "  ")
				os.WriteFile(filepath.Join(dir, "before.json"), b, 0644)
				if steps.MIR2Module != nil {
					os.WriteFile(filepath.Join(dir, "before.mir"), []byte(steps.MIR2Module.Dump()), 0644)
				}
			}
			if e != nil {
				t.Fatal(e)
			}
			t.Log(e)
		})
	}
}

func TestReadmeCPMWholeMain(t *testing.T) {
	for _, program := range []struct{ name, want string }{{"hello_cpm", "Hello!\r\n"}, {"hello_cpm_fib", "Hello Frill!\r\nfib(7)=13\r\ngcd(12,8)=4\r\n"}} {
		t.Run(program.name, func(t *testing.T) {
			source, e := os.ReadFile("../../../examples/nanz/" + program.name + ".nanz")
			if e != nil {
				t.Fatal(e)
			}
			frozen, e := os.ReadFile("../../../reports/readme-cpm-runtime/" + program.name + ".nanz")
			if e != nil || !bytes.Equal(source, frozen) {
				t.Fatal("source freeze", e)
			}
			save := func(path string, v any) {
				if root := os.Getenv("MINZ_README_CPM_ARTIFACTS"); root != "" {
					p := filepath.Join(root, program.name, path)
					os.MkdirAll(filepath.Dir(p), 0755)
					var b []byte
					switch x := v.(type) {
					case string:
						b = []byte(x)
					case []byte:
						b = x
					default:
						b, _ = json.MarshalIndent(v, "", "  ")
					}
					if e := os.WriteFile(p, b, 0644); e != nil {
						t.Fatal(e)
					}
				}
			}
			for _, mode := range []struct {
				name string
				o    Options
			}{{"ordinary", Options{ContractOpt: true}}, {"grace-size", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true}}, {"grace-speed", Options{ContractOpt: true, UseGrace: true, UseHIRGrace: true}}} {
				t.Run(mode.name, func(t *testing.T) {
					h, e := nanz.Parse(string(source), "examples/nanz/"+program.name+".nanz")
					if e != nil {
						t.Fatal(e)
					}
					o := mode.o
					o.Runtime = mir2.Z80RuntimeCPM
					o.RuntimeStackBytes = 1024
					o.AssertMode = "none"
					steps, e := CompileHIRSteps(h, o)
					if e != nil {
						t.Fatal(e)
					}
					image, e := z80asm.NewAssembler().AssembleString(steps.Assembly)
					if e != nil || len(image.Errors) > 0 {
						t.Fatal(e, image.Errors)
					}
					save(mode.name+"/program.a80", steps.Assembly)
					save(mode.name+"/program.com", image.Binary)
					save(mode.name+"/raw.mir", steps.MIR2Raw)
					save(mode.name+"/final.mir", steps.MIR2Module.Dump())
					save(mode.name+"/context.json", map[string]any{"options": o, "allocation": steps.Allocation, "symbols": image.Symbols, "functions": steps.MIR2Module.Funcs, "binary_sha256": fmt.Sprintf("%x", sha256.Sum256(image.Binary)), "source_assertions": h.Asserts})
					for _, clobber := range []bool{false, true} {
						z := emulator.NewRemogattoZ80WithScreen()
						z.SetROMEnd(0)
						mem := z.Memory()
						for p := range mem {
							mem[p] = byte(p*73 + 29)
						}
						z.LoadMemory(0x100, image.Binary)
						mem[0xfdfe] = 0
						mem[0xfdff] = 0
						r := z.GetRegisters()
						r.PC = 0x100
						r.SP = 0xfdfe
						r.A = 0x71
						r.F = 0x85
						r.BC = 0x1234
						r.DE = 0x5678
						r.HL = 0x9abc
						r.IX = 0x2468
						r.IY = 0x1357
						z.SetRegisters(r)
						before := append([]byte(nil), mem...)
						out := []byte{}
						ledger := []map[string]any{}
						minSP := uint16(image.Symbols["__cpm_stack_top"])
						done := false
						bdos := func() {
							rr := z.GetRegisters()
							if byte(rr.BC) != 2 {
								t.Fatal("unexpected BDOS", rr)
							}
							out = append(out, byte(rr.DE))
							ledger = append(ledger, map[string]any{"registers": rr, "char": byte(rr.DE)})
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
							if pc == 0 && z.GetSP() == 0xfe00 {
								done = true
								break
							}
							if pc == 5 {
								bdos()
								continue
							}
							if pc < 0x100 || int(pc) >= 0x100+len(image.Binary) {
								t.Fatal("unexpected PC", pc)
							}
							rr := z.GetRegisters()
							op := mem[pc]
							word := rr.BC
							push := true
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
								t.Fatal("stack word", pc)
							}
							if z.GetSP() < minSP {
								minSP = z.GetSP()
							}
						}
						mutable := map[uint16]bool{}
						for _, range_ := range [][2]int{{image.Symbols["__cpm_stack"], image.Symbols["__cpm_stack_top"]}, {image.Symbols["__cpm_saved_sp"], image.Symbols["__cpm_saved_sp"] + 2}, {image.Symbols["__cpm_status"], image.Symbols["__cpm_status"] + 1}} {
							for p := range_[0]; p < range_[1]; p++ {
								mutable[uint16(p)] = true
							}
						}
						for _, line := range image.Listing {
							if strings.HasPrefix(strings.ToLower(line.Label), "_spill_") {
								for j := range line.Bytes {
									mutable[uint16(line.Address+j)] = true
								}
							}
						}
						for p, b := range before {
							if !mutable[uint16(p)] && mem[p] != b {
								t.Fatalf("unexpected RAM/code write%04x", p)
							}
						}
						save(fmt.Sprintf("%s/clobber-%v.json", mode.name, clobber), map[string]any{"stdout": string(out), "expected": program.want, "BDOS": ledger, "registers": z.GetRegisters(), "cycles": z.GetCycles(), "min_sp": minSP, "stack_depth": image.Symbols["__cpm_stack_top"] - int(minSP), "done": done})
						if !done || string(out) != program.want || minSP < uint16(image.Symbols["__cpm_stack"]) || mem[uint16(image.Symbols["__cpm_stack_guard"])] != 0xa5 || mem[uint16(image.Symbols["__cpm_status"])] != 0 {
							t.Fatalf("whole main got%q want%q done%v minSP%x", out, program.want, done, minSP)
						}
					}
				})
			}
		})
	}
}

func TestReadmeCPMSourceAssertions(t *testing.T) {
	source, e := os.ReadFile("../../../reports/readme-cpm-runtime/hello_cpm_fib.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for _, o := range []Options{{ContractOpt: true}, {ContractOpt: true, UseGrace: true, UseHIRGrace: true, OptSize: true}, {ContractOpt: true, UseGrace: true, UseHIRGrace: true}} {
		h, e := nanz.Parse(string(source), "examples/nanz/hello_cpm_fib.nanz")
		if e != nil || len(h.Asserts) != 2 {
			t.Fatal("preserved assertions", e)
		}
		raw := hir.LowerModule(h)
		if e := RunAssertsMIR2(h, raw); e != nil {
			t.Fatal("raw source assertions", e)
		}
		o.AssertMode = "both"
		if _, e := CompileHIRSteps(h, o); e != nil {
			t.Fatal("final VM/native source assertions", e)
		}
	}
}
