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

func TestInteger32ReferenceVMAndZ80(t *testing.T) {
	paths, err := filepath.Glob("testdata/integer-reference/*.abap")
	if err != nil || len(paths) != 2 {
		t.Fatalf("reference sources: %v %v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(strings.TrimSuffix(path, ".abap") + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var ref struct {
				SHA    string           `json:"source_sha256"`
				Values map[string]int32 `json:"values"`
			}
			if err = json.Unmarshal(data, &ref); err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(src)) != ref.SHA {
				t.Fatal("reference source changed")
			}
			if len(ref.Values) != 3 {
				t.Fatal("missing reference globals")
			}
			m, err := Compile(string(src), "zref")
			if err != nil {
				t.Fatal(err)
			}
			for _, g := range m.Globals {
				if _, ok := ref.Values[g.Name]; ok && g.Ty != mir2.TyI32 {
					t.Fatalf("%s has type %s", g.Name, g.Ty)
				}
			}
			verifyABAPIntegerMemory(t, string(src), ref.Values)
		})
	}
}

func TestInteger32DecimalOutputVMAndZ80(t *testing.T) {
	for _, val := range []int32{0, 1, -1, 65536, -65537, -2147483648, 2147483647} {
		t.Run(fmt.Sprint(val), func(t *testing.T) {
			src := fmt.Sprintf("REPORT znumber. DATA x TYPE i VALUE %d. FORM probe. WRITE x. ENDFORM.", val)
			m, err := Compile(src, "znumber")
			if err != nil {
				t.Fatal(err)
			}
			steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			want := fmt.Sprintf("%d ", val) // Existing compact CP/M separator convention.
			for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
				v := mir2.NewVM(module)
				var out []byte
				v.Hosts["_abap_num_putchar"] = func(args []mir2.Value) ([]mir2.Value, error) {
					if len(args) != 1 {
						return nil, fmt.Errorf("putchar arity")
					}
					out = append(out, byte(args[0].I))
					return nil, nil
				}
				if _, err := v.Call("probe", nil); err != nil {
					t.Fatal(err)
				}
				if string(out) != want {
					t.Fatalf("VM output %q want %q", out, want)
				}
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if err != nil || len(a.Errors) > 0 {
				t.Fatalf("assemble: %v %v", err, a.Errors)
			}
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			copy(mem[0x8000:], a.Binary)
			mem[0xeffe], mem[0xefff] = 0, 0xf1
			z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["probe"]), SP: 0xeffe})
			var out []byte
			for i := 0; i < 1000000 && z.GetPC() != 0xf100; i++ {
				if z.GetPC() == 5 {
					r := z.GetRegisters()
					if byte(r.BC) != 2 {
						t.Fatalf("unexpected BDOS %d", byte(r.BC))
					}
					out = append(out, byte(r.DE))
					r.PC = binary.LittleEndian.Uint16(mem[r.SP : r.SP+2])
					r.SP += 2
					z.SetRegisters(r)
				} else {
					z.Step()
				}
			}
			r := z.GetRegisters()
			if r.PC != 0xf100 || r.SP != 0xf000 {
				t.Fatalf("return PC=%04x SP=%04x", r.PC, r.SP)
			}
			if string(out) != want {
				t.Fatalf("native output %q want %q", out, want)
			}
		})
	}
}

func TestInteger32LiteralAdmission(t *testing.T) {
	for _, value := range []string{"2147483648", "-2147483649", "999999999999999999999999999999"} {
		src := "REPORT zrange. DATA x TYPE i VALUE " + value + "."
		if _, err := Compile(src, "zrange"); err == nil {
			t.Errorf("accepted out-of-range DATA %s", value)
		}
		src = "REPORT zrange. DATA x TYPE i. FORM probe. x = " + value + ". ENDFORM."
		if _, err := Compile(src, "zrange"); err == nil {
			t.Errorf("accepted out-of-range assignment %s", value)
		}
	}
	for _, value := range []string{"-2147483648", "2147483647", "65536", "-65537"} {
		if _, err := Compile("REPORT zrange. DATA x TYPE i VALUE "+value+". FORM probe. x = "+value+". ENDFORM.", "zrange"); err != nil {
			t.Errorf("valid TYPE i literal %s: %v", value, err)
		}
	}
}
