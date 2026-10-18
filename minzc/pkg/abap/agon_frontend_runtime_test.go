package abap

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"strings"
	"testing"
)

func TestAgonFrontendUnchangedPickerAdmission(t *testing.T) {
	src, e := os.ReadFile("../../../examples/abap/zpickfile_cpm.abap")
	if e != nil {
		t.Fatal(e)
	}
	m, e := Compile(string(src), "zpickfile_cpm", hir.TargetAgon)
	if e != nil {
		t.Fatal(e)
	}
	if e = BindAgonZ80Runtime(m); e != nil {
		t.Fatal(e)
	}
	s, e := pipeline.CompileHIRSteps(m, pipeline.Options{Backend: "z80", Runtime: mir2.Z80RuntimeAgonZ80, AssertMode: "none"})
	if e != nil {
		t.Fatal(e)
	}
	if strings.Contains(s.Assembly, "CALL 5") {
		t.Fatal("native frontend retained CP/M service calls")
	}
}

// These provider functions are pure compiled code; no MOS/BDOS host callbacks
// participate. Native filesystem execution is checked separately with raw SPI.
func TestAgonFrontendCompiledNameProvider(t *testing.T) {
	src, e := os.ReadFile("../../../examples/abap/zpickfile_cpm.abap")
	if e != nil {
		t.Fatal(e)
	}
	m, e := Compile(string(src), "zpickfile_cpm", hir.TargetAgon)
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
	for _, module := range []*mir2.Module{raw, steps.MIR2Module} {
		v := mir2.NewVM(module)
		v.MaxSteps = 3000000
		call := func(name string) int64 {
			t.Helper()
			r, e := v.Call(name, nil)
			if e != nil || len(r) != 1 {
				t.Fatalf("%s: %v %v", name, r, e)
			}
			return r[0].I
		}
		info := v.GlobalAddr("_abap_fs_agon_info_buffer")
		scratch := v.GlobalAddr("_abap_fs_agon_scratch_buffer")
		for _, c := range []struct {
			name, want string
			attr       byte
		}{
			{"fleet.csv", "FLEET   CSV", 0}, {"README", "README     ", 0},
			{"TOOLONG99.CSV", "", 0}, {"A.LONG", "", 0}, {"A..CSV", "", 0}, {"../X.CSV", "", 0}, {"A B.CSV", "", 0}, {".CSV", "", 0}, {"A.", "", 0}, {"A.CSV", "", 16}, {"LABEL", "", 8},
		} {
			v.WriteHeapSlice(info, make([]byte, 278))
			v.WriteHeap(info+8, c.attr)
			v.WriteHeapSlice(info+22, append([]byte(c.name), 0))
			accepted := call("_abap_fs_agon_encode") != 0
			if accepted != (c.want != "") {
				t.Fatalf("%q attr%d accepted%v", c.name, c.attr, accepted)
			}
			if accepted && string(v.ReadHeap(scratch, 11)) != c.want {
				t.Fatalf("%q encoded%q want%q", c.name, v.ReadHeap(scratch, 11), c.want)
			}
		}
		records := v.GlobalAddr("_abap_fs_agon_records_buffer")
		for _, name := range []string{"ZETA    CSV", "ALPHA   CSV", "MIDDLE  CSV", "ALPHA   CSV"} {
			v.WriteHeapSlice(scratch, []byte(name))
			if call("_abap_fs_agon_insert") == 0 {
				t.Fatal("valid bounded insertion refused")
			}
		}
		if got := string(v.ReadHeap(records, 33)); got != "ALPHA   CSVMIDDLE  CSVZETA    CSV" {
			t.Fatalf("catalog order/dedup %q", got)
		}
		v.WriteHeapSlice(v.GlobalAddr("_abap_fs_agon_count"), []byte{128, 0})
		v.WriteHeapSlice(scratch, []byte("OVERFLOWCSV"))
		before := append([]byte(nil), v.ReadHeap(records, 128*11)...)
		if call("_abap_fs_agon_insert") != 0 || string(before) != string(v.ReadHeap(records, len(before))) {
			t.Fatal("catalog overflow changed existing records")
		}
	}
}

func TestAgonFrontendRejectsModifiedStringConsoleLeaf(t *testing.T) {
	m, e := Compile("REPORT zstr. DATA n TYPE i. WRITE n.", "zstr", hir.TargetAgon)
	if e != nil {
		t.Fatal(e)
	}
	m.FuncByName("abap_write_str").Body.Body[0].(*hir.AsmStmt).Code += " / LD (0),A"
	if e = BindAgonZ80Runtime(m); e == nil || !strings.Contains(e.Error(), "modified ABAP runtime body abap_write_str") {
		t.Fatalf("tampered canonical string console admitted: %v", e)
	}
}
