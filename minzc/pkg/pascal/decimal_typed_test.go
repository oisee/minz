package pascal

import (
	"strings"
	"testing"
)

func TestDecimalSignedUnsignedTypedSelection(t *testing.T) {
	h, e := Compile("program P; var I:Integer; W:Word; begin I := -1; W := 65535; WriteLn(I); WriteLn(W); end.", "typed.pas")
	if e != nil {
		t.Fatal(e)
	}
	dump := h.Dump()
	if !strings.Contains(dump, "WriteI16") || !strings.Contains(dump, "WriteU16") {
		t.Fatal(dump)
	}
	if h.FuncByName("WriteI16") == nil || h.FuncByName("WriteU16") == nil {
		t.Fatal("missing typed decimal helpers")
	}
}
