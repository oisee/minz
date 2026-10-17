package abap

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestCharacterFullReportReferenceVMAndZ80(t *testing.T) {
	paths, err := filepath.Glob("testdata/character-reference/*.abap")
	if err != nil || len(paths) != 3 {
		t.Fatalf("source count: %v %v", paths, err)
	}
	for _, p := range paths {
		t.Run(filepath.Base(p), func(t *testing.T) {
			src, err := os.ReadFile(p)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(strings.TrimSuffix(p, ".abap") + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var ref struct {
				SHA    string `json:"source_sha256"`
				Entry  string `json:"entry"`
				Output string `json:"output"`
				Values struct {
					X, Y string
					Seen int32
				} `json:"values"`
			}
			if err = json.Unmarshal(data, &ref); err != nil {
				t.Fatal(err)
			}
			if ref.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) || ref.Entry != "full report" {
				t.Fatal("reference identity changed")
			}
			m, err := Compile(string(src), "zref")
			if err != nil {
				t.Fatal(err)
			}
			steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			check := func(read func(string, int) []byte) {
				t.Helper()
				for name, want := range map[string]string{"x": ref.Values.X, "y": ref.Values.Y} {
					if got := string(read(name, len(want))); got != want {
						t.Fatalf("%s=%q want %q", name, got, want)
					}
				}
				if got := int32(binary.LittleEndian.Uint32(read("seen", 4))); got != ref.Values.Seen {
					t.Fatalf("seen=%d want %d", got, ref.Values.Seen)
				}
				if got := read("_abap_error_code", 1)[0]; got != 0 {
					t.Fatalf("unexpected arithmetic fault %d", got)
				}
			}
			for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
				v := mir2.NewVM(module)
				var out []byte
				v.Hosts["_abap_num_putchar"] = func(args []mir2.Value) ([]mir2.Value, error) { out = append(out, byte(args[0].I)); return nil, nil }
				if _, err := v.Call("main", nil); err != nil {
					t.Fatal(err)
				}
				check(func(name string, n int) []byte { return v.ReadHeap(v.GlobalAddr(name), n) })
				if string(out) != ref.Output {
					t.Fatalf("VM output %q want %q", out, ref.Output)
				}
			}
			// Both ordinary bare-metal termination and the existing called-main
			// codegen contract execute the complete report. No ASM/source patch.
			called, err := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{CalledMain: true, UnsignedPairComparePolicy: steps.UnsignedPairComparePolicy})
			if err != nil {
				t.Fatal(err)
			}
			for _, image := range []struct {
				name, text string
				called     bool
			}{{"bare", steps.Assembly, false}, {"called", called, true}} {
				t.Run(image.name, func(t *testing.T) {
					a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + image.text)
					if err != nil || len(a.Errors) > 0 {
						t.Fatalf("assemble: %v %v", err, a.Errors)
					}
					z := emulator.NewRemogattoZ80()
					mem := z.Memory()
					copy(mem[0x8000:], a.Binary)
					sp := uint16(0xf000)
					if image.called {
						sp = 0xeffe
						mem[sp], mem[sp+1] = 0, 0xf1
					}
					mem[0xe000] = 0xa5
					z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: sp})
					var out []byte
					for i := 0; i < 3000000 && z.GetPC() != 0xf100 && !z.IsHalted(); i++ {
						if z.GetPC() == 5 {
							r := z.GetRegisters()
							if byte(r.BC) != 2 {
								t.Fatalf("unexpected BDOS %d", byte(r.BC))
							}
							out = append(out, byte(r.DE))
							if len(out) > len(ref.Output) || string(out) != ref.Output[:len(out)] || r.SP >= 0xf000 {
								t.Log(image.text)
								t.Fatalf("BDOS prefix=%q want=%q SP=%04x regs=%+v", out, ref.Output, r.SP, r)
							}
							r.PC = binary.LittleEndian.Uint16(mem[r.SP : r.SP+2])
							r.SP += 2
							z.SetRegisters(r)
						} else {
							z.Step()
						}
					}
					r := z.GetRegisters()
					termination := r.PC == 0xf100
					if !image.called {
						termination = z.IsHalted() && (mem[r.PC] == 0x76 || mem[r.PC-1] == 0x76)
					}
					if !termination || r.SP != 0xf000 || mem[0xe000] != 0xa5 {
						t.Log(image.text)
						t.Fatalf("termination %s PC=%04x SP=%04x canary=%02x", image.name, r.PC, r.SP, mem[0xe000])
					}
					check(func(name string, n int) []byte { addr := a.Symbols[name]; return mem[addr : addr+n] })
					if string(out) != ref.Output {
						t.Fatalf("native output %q want %q", out, ref.Output)
					}
				})
			}

		})
	}
}

func TestCharacterAdmission(t *testing.T) {
	for _, src := range []string{
		"DATA x TYPE c LENGTH 2 VALUE 'é'.",
		"DATA x TYPE c LENGTH 2. x = 7.",
		"DATA x TYPE c LENGTH 2. DATA y TYPE i. y = x.",
		"DATA x TYPE c LENGTH 2. DATA y TYPE i. y = - x.",
		"DATA y TYPE i. y = `7`.",
		"FORM probe. DATA y TYPE i VALUE '7'. ENDFORM.",
		"DATA y TYPE i. y = `7` + 1.",
		"DATA x TYPE c LENGTH 2. IF x = `X`. ENDIF.",
		"FORM probe USING x TYPE c. x = 'X'. ENDFORM. PERFORM probe USING 'Y'.",
		"FORM probe CHANGING x TYPE c. ENDFORM. PERFORM probe CHANGING 'Y'.",
	} {
		if _, err := Compile("REPORT zbad. "+src, "zbad"); err == nil {
			t.Fatalf("accepted unimplemented character semantics: %s", src)
		}
	}
	a, err := expressionAtom("'a  '")
	if err != nil {
		t.Fatal(err)
	}
	b, err := expressionAtom("`a  `")
	if err != nil {
		t.Fatal(err)
	}
	if a.(*StringLit).IsString || !b.(*StringLit).IsString {
		t.Fatal("lost fixed/dynamic literal distinction")
	}
}
