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

// Each unchanged source is paired with an independently executed ABAP
// transpiler/runtime result. No MinZ-derived value is used as the oracle.
func TestFORMReferenceEffectsVMAndZ80(t *testing.T) {
	paths, err := filepath.Glob("testdata/form-reference/*.abap")
	if err != nil || len(paths) != 6 {
		t.Fatalf("reference sources: %v %v", paths, err)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			reference, err := os.ReadFile(strings.TrimSuffix(path, ".abap") + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var expected struct {
				SourceSHA256 string            `json:"source_sha256"`
				Values       map[string]uint16 `json:"values"`
			}
			if err = json.Unmarshal(reference, &expected); err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(src)) != expected.SourceSHA256 {
				t.Fatal("source differs from independent reference")
			}
			if len(expected.Values) != 3 {
				t.Fatal("reference must observe x, y and seen")
			}
			verifyABAPGlobalMemory(t, string(src), expected.Values)
		})
	}
}

func verifyABAPGlobalMemory(t *testing.T, src string, want map[string]uint16) {
	t.Helper()
	expected := make(map[string]int32, len(want))
	for name, val := range want {
		expected[name] = int32(val)
	}
	verifyABAPIntegerMemory(t, src, expected)
}

func verifyABAPIntegerMemory(t *testing.T, src string, want map[string]int32) {
	t.Helper()
	parse := func() *hir.Module {
		t.Helper()
		m, e := Compile(src, "zref")
		if e != nil {
			t.Fatal(e)
		}
		if e = hir.Verify(m); e != nil {
			t.Fatal(e)
		}
		return m
	}
	check := func(m *mir2.Module) {
		t.Helper()
		v := mir2.NewVM(m)
		if _, e := v.Call("probe", nil); e != nil {
			t.Fatal(e)
		}
		for n, w := range want {
			if got := binary.LittleEndian.Uint32(v.ReadHeap(v.GlobalAddr(n), 4)); got != uint32(w) {
				t.Fatalf("VM %s=%d want %d", n, got, w)
			}
		}
	}
	check(hir.LowerModule(parse()))
	steps, e := pipeline.CompileHIRSteps(parse(), pipeline.DefaultOptions())
	if e != nil {
		t.Fatal(e)
	}
	check(steps.MIR2Module)
	asm, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if e != nil || len(asm.Errors) > 0 {
		t.Fatalf("assembly: %v %v", e, asm.Errors)
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], asm.Binary)
	mem[0xeffe], mem[0xefff] = 0, 0xf1
	pc, ok := asm.Symbols["probe"]
	if !ok {
		t.Fatal("missing entry")
	}
	z.SetRegisters(emulator.Registers{PC: uint16(pc), SP: 0xeffe})
	for i := 0; i < 1000000 && z.GetPC() != 0xf100; i++ {
		z.Step()
	}
	r := z.GetRegisters()
	if r.PC != 0xf100 || r.SP != 0xf000 {
		t.Fatalf("return PC=%04x SP=%04x", r.PC, r.SP)
	}
	for n, w := range want {
		addr, ok := asm.Symbols[n]
		if !ok {
			t.Fatalf("missing global %s", n)
		}
		if got := binary.LittleEndian.Uint32(mem[addr : addr+4]); got != uint32(w) {
			t.Fatalf("Z80 %s=%d want %d\n%s", n, got, w, steps.Assembly)
		}
	}
}

func TestWRITEWithAddressableLocalsNative(t *testing.T) {
	src := `REPORT zwrite.
 DATA x TYPE i VALUE 5.
 FORM show USING p TYPE i.
  WRITE p.
  p = p + 1.
  WRITE p.
 ENDFORM.
 FORM probe.
  DATA local TYPE i VALUE 7.
  PERFORM show USING x.
  WRITE local.
  local = local + x.
  WRITE local.
 ENDFORM.`
	m, e := Compile(src, "zwrite")
	if e != nil {
		t.Fatal(e)
	}
	steps, e := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if e != nil {
		t.Fatal(e)
	}
	asm, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if e != nil || len(asm.Errors) > 0 {
		t.Fatalf("assembly: %v %v", e, asm.Errors)
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], asm.Binary)
	mem[0xeffe], mem[0xefff] = 0, 0xf1
	z.SetRegisters(emulator.Registers{PC: uint16(asm.Symbols["probe"]), SP: 0xeffe})
	out := []byte{}
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
	if string(out) != "5 6 7 13 " {
		t.Fatalf("output=%q\n%s", out, steps.Assembly)
	}
	addr := asm.Symbols["x"]
	if binary.LittleEndian.Uint16(mem[addr:addr+2]) != 6 {
		t.Fatal("USING mutation did not reach actual object")
	}
}

func TestFORMAdmissionRejectsUnsafeParameterModes(t *testing.T) {
	for _, src := range []string{
		`REPORT zbad. FORM bump USING p TYPE i. p = p + 1. ENDFORM. PERFORM bump USING 7.`,
		`REPORT zbad. FORM bump CHANGING p TYPE i. CLEAR p. ENDFORM. FORM relay USING p TYPE i. PERFORM bump CHANGING p. ENDFORM. PERFORM relay USING 7.`,
		`REPORT zbad. FORM value_mode USING VALUE(p) TYPE i. ENDFORM.`,
		`REPORT zbad. FORM untyped USING p. ENDFORM.`,
	} {
		if _, err := Compile(src, "bad"); err == nil {
			t.Fatalf("unsafe parameter mode admitted: %s", src)
		}
	}
}
