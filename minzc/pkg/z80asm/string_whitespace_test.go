package z80asm

import (
	"bytes"
	"testing"
)

func TestDBPreservesQuotedWhitespace(t *testing.T) {
	source := "ORG 0x8000\nDB \"CPU:      \", 0\nDB ' a  b\tc ', 0\nlabel: DB \"  x  \", 9, \"y   z\"\n"
	a, e := NewAssembler().AssembleString(source)
	if e != nil || a == nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	want := append([]byte("CPU:      \x00 a  b\tc \x00  x  \t"), []byte("y   z")...)
	if !bytes.Equal(a.Binary, want) {
		t.Fatalf("DB bytes got %q want %q", a.Binary, want)
	}
}
