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

func TestExpressionReferenceVMAndZ80(t *testing.T) {
	paths, err := filepath.Glob("testdata/expression-reference/*.abap")
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
				SHA    string            `json:"source_sha256"`
				Values map[string]uint16 `json:"values"`
			}
			if err = json.Unmarshal(data, &ref); err != nil {
				t.Fatal(err)
			}
			if fmt.Sprintf("%x", sha256.Sum256(src)) != ref.SHA {
				t.Fatal("reference source changed")
			}
			if len(ref.Values) != 3 {
				t.Fatal("missing observable globals")
			}
			verifyABAPGlobalMemory(t, string(src), ref.Values)
		})
	}
}

func TestExpressionAdmission(t *testing.T) {
	for _, tokens := range [][]string{
		{}, {"x", "+"}, {"x", "y"}, {"(", "x"}, {"x", ")"},
		{"x", "**", "2"}, {"foo", "(", "x", ")"}, {"x", "IS", "INITIAL"},
		{"1.5"}, {"|template|"}, {"999999999999999999999999999999"},
	} {
		if _, err := parseExpression(tokens); err == nil {
			t.Errorf("accepted unsupported expression %v", tokens)
		}
	}
	// Syntactically valid upstream ABAP must still be rejected by our adapter
	// when the executable construct has no admitted semantics.
	for _, stmt := range []string{
		"x = abs( x ).", "x = x ** 2.", "IF x IS INITIAL. x = 2. ENDIF.",
		"DO abs( x ) TIMES. x = 2. ENDDO.",
	} {
		src := "REPORT zexpr. DATA x TYPE i VALUE 1. FORM probe. " + stmt + " ENDFORM."
		if _, err := Compile(src, "zexpr"); err == nil {
			t.Errorf("accepted unsupported executable syntax: %s", stmt)
		}
	}
}

func TestUnknownExpressionLoweringRefuses(t *testing.T) {
	l := &lowerer{}
	for _, e := range []Expr_{
		&InvalidExpr{Message: "missing operand"}, &FieldAccess{Obj: &IntLit{Val: 1}, Field: "x"},
		&BinOp{Op: "mystery", LHS: &IntLit{Val: 1}, RHS: &IntLit{Val: 2}},
		&UnaryOp{Op: "mystery", Val: &IntLit{Val: 1}},
	} {
		if _, err := l.lowerExpr(e); err == nil {
			t.Errorf("silently lowered %T", e)
		}
	}
}
