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

func TestIndexedLoopFullReportReference(t *testing.T) {
	paths, err := filepath.Glob("testdata/loop-reference/*.abap")
	if err != nil || len(paths) != 5 {
		t.Fatalf("reference sources: %v count=%d", err, len(paths))
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
			var reference struct {
				SHA    string           `json:"source_sha256"`
				Entry  string           `json:"entry"`
				Values map[string]int32 `json:"values"`
				Output string           `json:"output"`
			}
			if err := json.Unmarshal(data, &reference); err != nil {
				t.Fatal(err)
			}
			if reference.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) || reference.Entry != "full report" || len(reference.Values) != 3 || reference.Output != "" {
				t.Fatal("reference identity/observables changed")
			}
			verifyIndexedLoopReport(t, string(src), reference.Values, filepath.Base(path))
		})
	}
}

func verifyIndexedLoopReport(t *testing.T, src string, want map[string]int32, name string) {
	t.Helper()
	m, err := Compile(src, "zref")
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	steps, err := pipeline.CompileHIRSteps(m, pipeline.DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	check := func(read func(string, int) []byte) {
		for name, value := range want {
			if got := int32(binary.LittleEndian.Uint32(read(name, 4))); got != value {
				t.Fatalf("%s=%d want %d", name, got, value)
			}
		}
		if read("_abap_error_code", 1)[0] != 0 {
			t.Fatal("unexpected arithmetic fault")
		}
		if got := int32(binary.LittleEndian.Uint32(read("_abap_index_value", 4))); got != 0 {
			t.Fatalf("report index not restored: %d", got)
		}
	}
	for _, module := range []*mir2.Module{hir.LowerModule(m), steps.MIR2Module} {
		v := mir2.NewVM(module)
		v.MaxSteps = 30000000
		if _, err := v.Call("main", nil); err != nil {
			t.Fatal(err)
		}
		check(func(name string, n int) []byte { return v.ReadHeap(v.GlobalAddr(name), n) })
	}
	text, err := mir2.Z80Codegen(steps.MIR2Module, steps.Allocation, mir2.Z80CodegenOptions{CalledMain: true, UnsignedPairComparePolicy: steps.UnsignedPairComparePolicy})
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + text)
	if err != nil || len(a.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, a.Errors)
	}
	if dir := os.Getenv("MINZ_ABAP_LOOP_ARTIFACTS"); dir != "" {
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		for ext, data := range map[string][]byte{"abap": []byte(src), "a80": []byte("ORG 0x8000\n" + text), "bin": a.Binary} {
			if err := os.WriteFile(filepath.Join(dir, name+"."+ext), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], a.Binary)
	mem[0xeffe], mem[0xefff] = 0, 0xf1
	mem[0xe000] = 0xa5
	z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: 0xeffe})
	// The wide reference deliberately executes 65537 passes of checked i32
	// arithmetic; its native expansion exceeds the former 60M-step budget.
	for i := 0; i < 160000000 && z.GetPC() != 0xf100 && !z.IsHalted(); i++ {
		z.Step()
	}
	r := z.GetRegisters()
	if r.PC != 0xf100 || r.SP != 0xf000 || mem[0xe000] != 0xa5 || binary.LittleEndian.Uint16(mem[0xeffe:0xf000]) != 0xf100 {
		t.Fatalf("native PC/SP/canary: %+v", r)
	}
	check(func(name string, n int) []byte {
		addr, ok := a.Symbols[name]
		if !ok {
			t.Fatalf("missing global %s", name)
		}
		return mem[addr : addr+n]
	})
}

func TestIndexedLoopReturnAndProcessingBlockExit(t *testing.T) {
	// ABAP DO/WHILE documentation: inside the caller loop SY-INDEX must refer
	// to that loop, including after a nested procedure's RETURN.
	verifyIndexedLoopReport(t, `REPORT zref.
DATA x TYPE i.
DATA y TYPE i.
DATA seen TYPE i.
FORM early.
 DO 2 TIMES.
  DO 2 TIMES.
   y = sy-index.
   RETURN.
  ENDDO.
 ENDDO.
ENDFORM.
DO 2 TIMES.
 x = x * 10 + sy-index.
 PERFORM early.
 x = x * 10 + sy-index.
ENDDO.
seen = sy-index.`, map[string]int32{"x": 1122, "y": 1, "seen": 0}, "nested-return")
	verifyIndexedLoopReport(t, `REPORT zref.
DATA x TYPE i.
DATA y TYPE i.
DATA seen TYPE i.
FORM early.
 x = 1.
 EXIT.
 x = 77.
ENDFORM.
PERFORM early.
y = 2.`, map[string]int32{"x": 1, "y": 2, "seen": 0}, "processing-exit")
	if m, err := Compile("REPORT zref. CONTINUE.", "zref"); m != nil || err == nil {
		t.Fatal("CONTINUE outside a loop was admitted")
	}
}

func TestSystemFieldAdmission(t *testing.T) {
	for _, field := range []string{"datum", "uzeit", "ucomm", "tfill", "mandt"} {
		if module, err := Compile("REPORT zref. WRITE sy-"+field+".", "zref"); module != nil || err == nil || !strings.Contains(err.Error(), "precise system-field semantics") {
			t.Fatalf("SY-%s: executable module returned=%t error=%v", field, module != nil, err)
		}
	}
}
