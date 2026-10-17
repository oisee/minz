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

func TestWriteFullReportsReferenceVMAndZ80(t *testing.T) {
	for _, name := range []string{"templates", "literals"} {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile("testdata/write-reference/" + name + ".abap")
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile("testdata/write-reference/" + name + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var ref struct {
				SHA    string `json:"source_sha256"`
				Entry  string `json:"entry"`
				Output string `json:"output"`
				Values struct {
					X, Seen int32
					Y       string
				} `json:"values"`
			}
			if err := json.Unmarshal(data, &ref); err != nil {
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
				for field, want := range map[string]int32{"x": ref.Values.X, "seen": ref.Values.Seen} {
					if got := int32(binary.LittleEndian.Uint32(read(field, 4))); got != want {
						t.Fatalf("%s=%d want %d", field, got, want)
					}
				}
				if got := string(read("y", len(ref.Values.Y))); got != ref.Values.Y {
					t.Fatalf("y=%q want %q", got, ref.Values.Y)
				}
				if read("_abap_error_code", 1)[0] != 0 {
					t.Fatal("runtime fault")
				}
			}
			for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
				v := mir2.NewVM(module)
				var out []byte
				v.Hosts["_abap_num_putchar"] = func(args []mir2.Value) ([]mir2.Value, error) { out = append(out, byte(args[0].I)); return nil, nil }
				if _, err := v.Call("main", nil); err != nil {
					t.Fatal(err)
				}
				check(func(field string, n int) []byte { return v.ReadHeap(v.GlobalAddr(field), n) })
				if string(out) != ref.Output {
					t.Fatalf("VM output=%q want %q", out, ref.Output)
				}
			}
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
						t.Fatalf("assembly: %v %v", err, a.Errors)
					}
					if 0x8000+len(a.Binary) >= 0xe000 {
						t.Fatal("image overlaps canary")
					}
					if dir := os.Getenv("MINZ_ABAP_WRITE_ARTIFACTS"); dir != "" {
						if err := os.MkdirAll(dir, 0755); err != nil {
							t.Fatal(err)
						}
						for ext, b := range map[string][]byte{"abap": src, "json": data, "a80": []byte("ORG 0x8000\n" + image.text), "bin": a.Binary} {
							if err := os.WriteFile(filepath.Join(dir, name+"-"+image.name+"."+ext), b, 0644); err != nil {
								t.Fatal(err)
							}
						}
					}
					z := emulator.NewRemogattoZ80()
					mem := z.Memory()
					copy(mem[0x8000:], a.Binary)
					mem[0xe000] = 0xa5
					sp := uint16(0xf000)
					if image.called {
						sp = 0xeffe
						mem[sp], mem[sp+1] = 0, 0xf1
					}
					z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: sp})
					var out []byte
					for n := 0; n < 3000000 && z.GetPC() != 0xf100 && !z.IsHalted(); n++ {
						if z.GetPC() == 5 {
							r := z.GetRegisters()
							if byte(r.BC) != 2 {
								t.Fatalf("unexpected BDOS %d", byte(r.BC))
							}
							out = append(out, byte(r.DE))
							if len(out) > len(ref.Output) || string(out) != ref.Output[:len(out)] {
								t.Fatalf("native prefix=%q want %q", out, ref.Output)
							}
							r.PC = binary.LittleEndian.Uint16(mem[r.SP : r.SP+2])
							r.SP += 2
							z.SetRegisters(r)
						} else {
							z.Step()
						}
					}
					r := z.GetRegisters()
					done := r.PC == 0xf100
					if !image.called {
						done = z.IsHalted() && (mem[r.PC] == 0x76 || mem[r.PC-1] == 0x76)
					}
					if !done || r.SP != 0xf000 || mem[0xe000] != 0xa5 {
						t.Fatalf("termination PC=%04x SP=%04x canary=%02x", r.PC, r.SP, mem[0xe000])
					}
					if image.called && binary.LittleEndian.Uint16(mem[0xeffe:0xf000]) != 0xf100 {
						t.Fatal("outer return word changed")
					}
					check(func(field string, n int) []byte { addr := a.Symbols[field]; return mem[addr : addr+n] })
					if string(out) != ref.Output {
						t.Fatalf("native output=%q want %q", out, ref.Output)
					}
				})
			}
		})
	}
}

func TestWriteFormattingAdmission(t *testing.T) {
	for _, statement := range []string{
		"WRITE x CURRENCY y.", "WRITE x TO y.", "WRITE x NO-SIGN.",
		"WRITE |{ x WIDTH = 5 }|.", "ULINE.", "ULINE /1(5).", "SKIP 2.", "NEW-LINE SCROLLING.",
		"WRITE 'é'.", "WRITE |é|.",
	} {
		t.Run(statement, func(t *testing.T) {
			m, err := Compile("REPORT zadmit. DATA x TYPE i. DATA y TYPE c LENGTH 5. "+statement, "zadmit")
			if m != nil || err == nil {
				t.Fatalf("formatting/text admitted: module=%t err=%v", m != nil, err)
			}
			t.Log(err)
		})
	}
}

func TestWriteTemplateQuotedExpression(t *testing.T) {
	// Whitespace and braces inside an expression literal must remain quoted.
	parts, err := parseWriteTemplate([]string{"|<{", "'a } b'", "}>|"})
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 3 {
		t.Fatalf("parts=%v", parts)
	}
	value, ok := parts[1].(*TemplateValue)
	if !ok {
		t.Fatal("missing template value")
	}
	lit, ok := value.Val.(*StringLit)
	if !ok || lit.Val != "a } b" {
		t.Fatalf("quoted value=%v", value.Val)
	}
	if _, err := parseWriteTemplate([]string{`|\q|`}); err == nil || !strings.Contains(err.Error(), "escape") {
		t.Fatalf("unsupported escape admitted: %v", err)
	}
}
