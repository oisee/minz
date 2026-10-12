package plm

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
)

func TestCPMConsoleIntrinsic(t *testing.T) {
	for _, tc := range []struct{ body, diagnostic string }{
		{"CALL BDOS(2, 65);", ""},
		{"CALL BDOS(9, 65);", "literal function 2"},
		{"CALL BDOS(2);", "one BYTE argument"},
		{"CALL BDOS(2, 256);", "BYTE argument"},
		{"CALL BDOS(2, 65, 66);", "one BYTE argument"},
	} {
		h, err := Compile("HELLO: DO; " + tc.body + " END HELLO;")
		if tc.diagnostic != "" {
			if err == nil || !strings.Contains(err.Error(), tc.diagnostic) {
				t.Fatal(tc.body, err)
			}
			continue
		}
		if err != nil {
			t.Fatal(err)
		}
		call := h.FuncByName("main").Body.Body[0].(*hir.ExprStmt).Expr.(*hir.CallExpr)
		if call.Fn != "@mir.io.print.char" {
			t.Fatal(call.Fn)
		}
	}
	h, err := Compile("HELLO: DO; BDOS: PROCEDURE (X); DECLARE X BYTE; END BDOS; CALL BDOS(65); END HELLO;")
	if err != nil {
		t.Fatal(err)
	}
	call := h.FuncByName("main").Body.Body[0].(*hir.ExprStmt).Expr.(*hir.CallExpr)
	if call.Fn != "BDOS" {
		t.Fatal("declared procedure was hijacked", call.Fn)
	}
}

func TestAddressedByteListExplicitTerminator(t *testing.T) {
	for _, tc := range []struct{ literal, want string }{{".'hello'", "hello"}, {".('hello', 0)", "hello\x00"}, {".('x', 13, 10, 0)", "x\r\n\x00"}} {
		h, err := Compile("HELLO: DO; DECLARE P WORD; P = " + tc.literal + "; END HELLO;")
		if err != nil {
			t.Fatal(err)
		}
		if len(h.Globals) != 2 || string(h.Globals[1].Init) != tc.want {
			t.Fatal(tc.literal, h.Globals)
		}
	}
	if _, err := Compile("HELLO: DO; DECLARE P WORD; P = .('x', 256); END HELLO;"); err == nil {
		t.Fatal("non-byte accepted")
	}
}
