package abap

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
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
	if err != nil {
		t.Fatal(err)
	}
	if err := hir.Verify(hm); err == nil {
		t.Fatal("sibling FORM accepted another FORM's local DATA")
	}
}
