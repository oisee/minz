package abap

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDataScopeFullReportReference(t *testing.T) {
	paths, err := filepath.Glob("testdata/data-scope-reference/*.abap")
	if err != nil || len(paths) != 4 {
		t.Fatalf("sources %d error %v", len(paths), err)
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
				Entry  string           `json:"entry"`
				Values map[string]int32 `json:"values"`
				Output string           `json:"output"`
			}
			if err := json.Unmarshal(data, &ref); err != nil {
				t.Fatal(err)
			}
			if ref.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) || ref.Entry != "full report" || len(ref.Values) != 3 || ref.Output != "" {
				t.Fatal("reference identity/observables changed")
			}
			verifyIndexedLoopReport(t, string(src), ref.Values, "data-scope-"+filepath.Base(path))
		})
	}
}
func TestDataScopeAdmission(t *testing.T) {
	for _, tc := range []struct{ src, diagnostic string }{
		{"FORM f. DATA x TYPE i. DATA x TYPE i. ENDFORM.", "duplicate procedure object"},
		{"FORM f USING x TYPE i. DATA x TYPE i. ENDFORM.", "duplicate procedure object"},
		{"DATA x TYPE i. START-OF-SELECTION. IF 1 = 1. DATA x TYPE i. ENDIF.", "duplicate report object"},
		{"FORM f. DATA x TYPE i VALUE sy-index. ENDFORM.", "literal initialization"},
	} {
		t.Run(tc.src, func(t *testing.T) {
			m, err := Compile("REPORT zref. "+tc.src, "zref")
			if err == nil || m != nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatalf("module=%v error=%v want %s", m, err, tc.diagnostic)
			}
		})
	}
}

func TestDataScopeShadowVisibility(t *testing.T) {
	// DATA documentation: the local name becomes visible at its declaration.
	// Provider 2.14.8 cannot execute the numeric probe: JS's lexical TDZ masks
	// the earlier global read. This gate is therefore specification-backed.
	verifyIndexedLoopReport(t, `REPORT zref.
DATA x TYPE i. DATA y TYPE i. DATA seen TYPE i VALUE 7.
FORM measure.
 x = seen.
 DATA seen TYPE i VALUE 1.
 y = seen.
ENDFORM.
PERFORM measure.`, map[string]int32{"x": 7, "y": 1, "seen": 7}, "data-scope-numeric-shadow")
	verifyIndexedLoopReport(t, `REPORT zref.
DATA x TYPE i. DATA y TYPE i. DATA seen TYPE i.
DATA c TYPE c LENGTH 3 VALUE 'old'.
FORM measure.
 IF c = 'old'. x = 1. ENDIF.
 DATA c TYPE c LENGTH 3 VALUE 'new'.
 IF c = 'new'. y = 1. ENDIF.
ENDFORM.
PERFORM measure.
IF c = 'old'. seen = 1. ENDIF.`, map[string]int32{"x": 1, "y": 1, "seen": 1}, "data-scope-character-shadow")
}

func TestDataScopeImplicitResultAdmission(t *testing.T) {
	result := FormParam{Name: "x", AbapTy: "i"}
	for _, decl := range []Decl{
		&FunctionDecl{Name: "f", Importing: []FormParam{result}, Exporting: []FormParam{result}},
		&ClassDecl{Name: "c", Methods: []*MethodDecl{{Name: "f", Importing: []FormParam{result}, Returning: &result}}},
	} {
		m, err := LowerProgramWithTarget(&Program{Name: "zref", Decls: []Decl{decl}}, 0)
		if err == nil || m != nil || !strings.Contains(err.Error(), "duplicate procedure object") {
			t.Fatalf("module=%v error=%v", m, err)
		}
	}
}
