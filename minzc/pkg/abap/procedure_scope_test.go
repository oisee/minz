package abap

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"strings"
	"testing"
)

func TestProcedureLocalDataScopesAndReturns(t *testing.T) {
	src := `REPORT zscope.
FORM first USING pv_x TYPE i CHANGING pv_result TYPE i.
 DATA lv_value TYPE i VALUE 300.
 pv_result = lv_value + pv_x.
ENDFORM.
FORM second USING pv_x TYPE i CHANGING pv_result TYPE i.
 DATA lv_value TYPE i VALUE 7.
 pv_result = lv_value + pv_x.
ENDFORM.
DATA pv_x TYPE i VALUE 9.
DATA lv_value TYPE i VALUE 99.
DATA lv_result TYPE i.
PERFORM first USING pv_x CHANGING lv_result.
* assert first(2) == 302 via mir2
* assert second(2) == 9 via mir2
`
	verifyProcedureModule(t, src)
}

func TestFormsExampleStrictHIRAndNativeReturns(t *testing.T) {
	src, err := os.ReadFile("../../../examples/abap/forms.abap")
	if err != nil {
		t.Fatal(err)
	}
	verifyProcedureModule(t, string(src)+"\n* assert factorial(6) == 720 via mir2\n")
}

func verifyProcedureModule(t *testing.T, src string) {
	t.Helper()
	hm, err := Compile(src, "forms")
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.Verify(hm); err != nil {
		t.Fatal(err)
	}
	opts := pipeline.DefaultOptions()
	opts.AssertMode = "none"
	steps, err := pipeline.CompileHIRSteps(hm, opts)
	if err != nil {
		t.Fatal(err)
	}
	if err := pipeline.RunAssertsMIR2(hm, steps.MIR2Module); err != nil {
		t.Fatal(err)
	}
	if err := pipeline.RunAssertsZ80(hm, steps.MIR2Module, steps.Allocation, steps.Assembly); err != nil {
		t.Fatal(err)
	}
}

func TestProcedureLocalsDoNotLeakIntoSibling(t *testing.T) {
	hm, err := Compile(`REPORT zscope.
FORM first USING pv_x TYPE i CHANGING pv_result TYPE i.
 DATA lv_private TYPE i VALUE 3.
 pv_result = lv_private + pv_x.
ENDFORM.
FORM second USING pv_x TYPE i CHANGING pv_result TYPE i.
 pv_result = lv_private + pv_x.
ENDFORM.`, "scope")
	if hm != nil || err == nil || !strings.Contains(err.Error(), "undeclared ABAP object \"lv_private\"") {
		t.Fatalf("sibling FORM accepted another FORM's local DATA: executable module returned=%t error=%v", hm != nil, err)
	}
}

func TestEventDataHasReportScope(t *testing.T) {
	hm, err := Compile(`REPORT zevents.
INITIALIZATION.
 DATA lv_i TYPE i VALUE 1.
START-OF-SELECTION.
 DATA: lv_has_row TYPE i,
       lv_count TYPE i VALUE 7.
 lv_i = lv_count.
END-OF-SELECTION.
 lv_has_row = lv_i.`, "events")
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.Verify(hm); err != nil {
		t.Fatal(err)
	}
	for name, want := range map[string]byte{"lv_i": 1, "lv_has_row": 0, "lv_count": 7} {
		found := false
		for _, global := range hm.Globals {
			if global.Name == name {
				found = true
				got := byte(0)
				if len(global.Init) > 0 {
					got = global.Init[0]
				}
				if got != want {
					t.Fatalf("%s initial byte=%d, want %d", name, got, want)
				}
			}
		}
		if !found {
			t.Fatalf("missing event DATA report variable %s", name)
		}
	}
	opts := pipeline.DefaultOptions()
	opts.AssertMode = "none"
	if _, err := pipeline.CompileHIRSteps(hm, opts); err != nil {
		t.Fatal(err)
	}
}

func TestReportEventExamplesCompile(t *testing.T) {
	for _, name := range []string{"mara_report", "selscreen"} {
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile("../../../examples/abap/" + name + ".abap")
			if err != nil {
				t.Fatal(err)
			}
			hm, err := Compile(string(src), name)
			if err != nil {
				t.Fatal(err)
			}
			if err := hir.Verify(hm); err != nil {
				t.Fatal(err)
			}
			opts := pipeline.DefaultOptions()
			opts.AssertMode = "none"
			if _, err := pipeline.CompileHIRSteps(hm, opts); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestWriteSplitSystemField(t *testing.T) {
	decl, err := parseWriteStmt([]string{"WRITE", "SY", "-", "SUBRC", "."})
	if err != nil {
		t.Fatal(err)
	}
	write := decl.(*formBodyDecl).stmt.(*WriteStmt)
	if len(write.Exprs) != 1 {
		t.Fatalf("WRITE SY-SUBRC produced %d expressions", len(write.Exprs))
	}
	field, ok := write.Exprs[0].(*SYField)
	if !ok || field.Field != "SUBRC" {
		t.Fatalf("WRITE SY-SUBRC = %#v", write.Exprs[0])
	}
}
