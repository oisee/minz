package abap

import (
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"os"
	"strings"
	"testing"
)

func TestAgonSALVUnchangedReportAdmission(t *testing.T) {
	src, err := os.ReadFile("../../../examples/abap/zfleet_csv_alv.abap")
	if err != nil {
		t.Fatal(err)
	}
	m, err := Compile(string(src), "zfleet_csv_alv", hir.TargetAgon)
	if err != nil {
		t.Fatal(err)
	}
	if err = BindAgonZ80Runtime(m); err != nil {
		t.Fatal(err)
	}
	s, err := pipeline.CompileHIRSteps(m, pipeline.Options{Backend: "z80", Runtime: mir2.Z80RuntimeAgonZ80, AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(s.Assembly, "CALL 5") {
		t.Fatal("Agon grid retained CP/M service")
	}
	for _, symbol := range []string{"__agon_screen_begin:", "__agon_screen_end:", "CALL __agon_screen_cleanup"} {
		if !strings.Contains(s.Assembly, symbol) {
			t.Fatalf("missing native terminal lifecycle %s", symbol)
		}
	}
}
