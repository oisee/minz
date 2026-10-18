package abap

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
)

func TestAgonZ80CanonicalRuntimeBinding(t *testing.T) {
	for _, mutate := range []bool{false, true} {
		m, err := Compile("REPORT zconsole. DATA n TYPE i VALUE 70000. WRITE n.", "zconsole", hir.TargetAgon)
		if err != nil {
			t.Fatal(err)
		}
		if mutate {
			m.FuncByName("_abap_num_putchar").Body.Body[0].(*hir.AsmStmt).Code += " / LD (0), A"
		}
		err = BindAgonZ80Runtime(m)
		if mutate {
			if err == nil || !strings.Contains(err.Error(), "modified") {
				t.Fatalf("mutated canonical runtime admitted: %v", err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		if m.Target != hir.TargetAgon || strings.Contains(m.Dump(), "CALL 5") || strings.Contains(m.Dump(), "RST") || !strings.Contains(m.Dump(), "@mir.io.print.char") {
			t.Fatal("binding changed platform or retained opaque operating-system ASM")
		}
	}
}

func TestAgonZ80BindingRejectsUnsupportedInput(t *testing.T) {
	m, err := Compile("REPORT zinput. PARAMETERS n TYPE i. WRITE n.", "zinput", hir.TargetAgon)
	if err != nil {
		t.Fatal(err)
	}
	if err := BindAgonZ80Runtime(m); err == nil || !strings.Contains(err.Error(), "not supported") {
		t.Fatalf("unsupported input leaf silently admitted: %v", err)
	}
}

func TestAgonZ80I32ReportIndependentOracle(t *testing.T) {
	src, e := os.ReadFile("testdata/agon-z80-reference/i32-report.abap")
	if e != nil {
		t.Fatal(e)
	}
	data, e := os.ReadFile("testdata/agon-z80-reference/i32-report.json")
	if e != nil {
		t.Fatal(e)
	}
	var ref struct {
		SHA    string           `json:"source_sha256"`
		Entry  string           `json:"entry"`
		Values map[string]int32 `json:"values"`
		Output string           `json:"output"`
	}
	if e = json.Unmarshal(data, &ref); e != nil {
		t.Fatal(e)
	}
	if ref.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) || ref.Entry != "full report" {
		t.Fatal("independent oracle identity changed")
	}
	m, e := Compile(string(src), "i32-report", hir.TargetAgon)
	if e != nil {
		t.Fatal(e)
	}
	if e = BindAgonZ80Runtime(m); e != nil {
		t.Fatal(e)
	}
	raw := hir.LowerModule(m)
	steps, e := pipeline.CompileHIRSteps(m, pipeline.Options{Backend: "z80", Runtime: mir2.Z80RuntimeAgonZ80, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	for _, mod := range []*mir2.Module{raw, steps.MIR2Module} {
		v := mir2.NewVM(mod)
		v.MaxSteps = 30000000
		var out []byte
		v.Hosts["@mir.io.print.char"] = func(args []mir2.Value) ([]mir2.Value, error) { out = append(out, byte(args[0].I)); return nil, nil }
		if _, e = v.Call("main", nil); e != nil {
			t.Fatal(e)
		}
		for name, want := range ref.Values {
			if got := int32(binary.LittleEndian.Uint32(v.ReadHeap(v.GlobalAddr(name), 4))); got != want {
				t.Fatalf("%s=%d want%d", name, got, want)
			}
		}
		if string(out) != ref.Output {
			t.Fatalf("output %q want %q", out, ref.Output)
		}
	}
}
