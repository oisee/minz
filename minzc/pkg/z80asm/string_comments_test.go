package z80asm

import (
	"bytes"
	"testing"
)

func TestQuotedSemicolonIsData(t *testing.T) {
	for _, src := range []string{`DB ";",0 ; comment`, `text: DB ';',0 ; comment`, `DB "a;b",0 ; comment`} {
		a, e := NewAssembler().AssembleString("ORG 0x8000\n" + src)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		want := []byte{59, 0}
		if src == `DB "a;b",0 ; comment` {
			want = []byte{'a', ';', 'b', 0}
		}
		if !bytes.Equal(a.Binary, want) {
			t.Fatalf("%s: %v want%v", src, a.Binary, want)
		}
	}
	p, e := ParseLine("EX AF,AF' ; comment", 1)
	if e != nil || p.Comment != "comment" {
		t.Fatal(p, e)
	}
}
