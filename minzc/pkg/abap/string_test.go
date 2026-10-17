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

func TestStringFullReportReferenceVMAndZ80(t *testing.T) {
	paths, err := filepath.Glob("testdata/string-reference/*.abap")
	if err != nil || len(paths) != 2 {
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
					X, Y, C string
					Seen    int32
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
			check := func(t *testing.T, read func(string, int) []byte) {
				t.Helper()
				for name, want := range map[string]string{"x": ref.Values.X, "y": ref.Values.Y} {
					length := int(binary.LittleEndian.Uint16(read(name, 2)))
					if length != len(want) {
						t.Fatalf("%s length=%d want %d", name, length, len(want))
					}
					capacity := int(binary.LittleEndian.Uint16(read(name, 4)[2:]))
					if capacity < length {
						t.Fatalf("%s length %d exceeds capacity %d", name, length, capacity)
					}
					got := string(read(name, 4+length)[4:])
					if got != want {
						t.Fatalf("%s=%q want %q", name, got, want)
					}
				}
				if got := string(read("c", len(ref.Values.C))); got != ref.Values.C {
					t.Fatalf("c=%q want %q", got, ref.Values.C)
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
				check(t, func(name string, n int) []byte { return v.ReadHeap(v.GlobalAddr(name), n) })
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
					check(t, func(name string, n int) []byte {
						symbol := name
						if name == "c" {
							symbol = "v_c"
						} // documented register-name escaping in native backend
						addr, ok := a.Symbols[symbol]
						if !ok {
							t.Fatalf("missing native object %s", symbol)
						}
						return mem[addr : addr+n]
					})
					if string(out) != ref.Output {
						t.Fatalf("native output %q want %q", out, ref.Output)
					}
				})
			}

		})
	}
}

func TestStringCapacityFaultBeforeCommit(t *testing.T) {
	src := `REPORT zcapacity.
DATA x TYPE string VALUE ` + "`AB`" + `.
DATA y TYPE string VALUE ` + "`KEEP`" + `.
DATA seen TYPE i VALUE 0.
FORM copy USING source TYPE string CHANGING result TYPE string.
 result = source.
ENDFORM.
FORM probe.
 PERFORM copy USING x CHANGING y.
 seen = 99.
ENDFORM.`
	m, err := Compile(src, "zcapacity")
	if err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(a.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, a.Errors)
	}
	// Simulate a valid foreign descriptor whose owned destination has a smaller
	// capacity than this compilation unit's inferred bound. No source changes.
	for _, capacity := range []uint16{0, 1} {
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
				v := mir2.NewVM(module)
				addr := v.GlobalAddr("y")
				before := v.ReadHeap(addr, 8)
				binary.LittleEndian.PutUint16(before[:2], capacity)
				binary.LittleEndian.PutUint16(before[2:4], capacity)
				v.WriteHeapSlice(addr, before)
				if _, err := v.Call("probe", nil); err == nil {
					t.Fatal("capacity failure returned normally")
				}
				if string(v.ReadHeap(addr, 8)) != string(before) {
					t.Fatal("VM committed part of overflowing assignment")
				}
				if v.ReadHeap(v.GlobalAddr("_abap_error_code"), 1)[0] != 3 || binary.LittleEndian.Uint32(v.ReadHeap(v.GlobalAddr("seen"), 4)) != 0 {
					t.Fatal("VM fault/status boundary")
				}
			}
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			copy(mem[0x8000:], a.Binary)
			addr := a.Symbols["y"]
			before := append([]byte(nil), mem[addr:addr+8]...)
			binary.LittleEndian.PutUint16(before[:2], capacity)
			binary.LittleEndian.PutUint16(before[2:4], capacity)
			copy(mem[addr:], before)
			mem[0xeffe], mem[0xefff] = 0, 0xf1
			mem[0xe000] = 0xa5
			z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["probe"]), SP: 0xeffe})
			for i := 0; i < 1000000 && z.GetPC() != 0xf100 && !z.IsHalted(); i++ {
				z.Step()
			}
			r := z.GetRegisters()
			if !z.IsHalted() || r.PC == 0xf100 || r.SP > 0xeffe || r.SP < 0xe000 || mem[0xe000] != 0xa5 {
				t.Fatalf("native fault boundary %+v", r)
			}
			if string(mem[addr:addr+8]) != string(before) || mem[a.Symbols["_abap_error_code"]] != 3 || binary.LittleEndian.Uint32(mem[a.Symbols["seen"]:a.Symbols["seen"]+4]) != 0 {
				t.Fatal("native committed overflowing assignment")
			}
			if mem[0xeffe] != 0 || mem[0xefff] != 0xf1 {
				t.Fatal("outer return address damaged")
			}
		})
	}
}

func TestStringAdmission(t *testing.T) {
	for _, src := range []string{
		"DATA x TYPE string. x = 7.",
		"DATA x TYPE string. x = x && x.",
		"DATA x TYPE string. DATA y TYPE i. y = x.",
		"PARAMETERS x TYPE string.",
		"DATA x TYPE string VALUE 'é'.",
		"FORM probe USING x TYPE string. ENDFORM. PERFORM probe USING `X`.",
	} {
		if _, err := Compile("REPORT zbad. "+src, "zbad"); err == nil {
			t.Fatalf("accepted unsupported string semantics: %s", src)
		}
	}
	p, err := Parse("REPORT zbound. DATA x TYPE string. DATA c TYPE c LENGTH 9. FORM probe. x = `abcdefghijkl`. ENDFORM.", "zbound")
	if err != nil {
		t.Fatal(err)
	}
	if n, err := inferStringCapacity(p); err != nil || n != 12 {
		t.Fatalf("capacity=%d error=%v", n, err)
	}
}
