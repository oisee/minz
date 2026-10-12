package lanz

import (
	"github.com/minz/minzc/pkg/hir"
	"testing"
)

func TestLambdaRecordsLexicalCaptures(t *testing.T) {
	m, err := Compile(`(fun test () u8
 (var sum u8 1)
 (var cb ptr (lambda ((x u8)) u8 (return (+ sum x))))
 (return sum))`, "captures")
	if err != nil {
		t.Fatal(err)
	}
	f := m.FuncByName("lambda_0")
	if f == nil || len(f.Captures) != 1 || f.Captures[0].Name != "sum" {
		t.Fatalf("captures %+v", f)
	}
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
}
