package z80asm

import (
	"bytes"
	"testing"
)

func TestLocalReferencesPreserveStringBytes(t *testing.T) {
	a, e := NewAssembler().AssembleString("ORG 0x8000\nmain:\n JR .done\n DB \".help .quit\", 0, '.exit', 0\n.done: RET\n")
	if e != nil || a == nil || len(a.Errors) > 0 {
		t.Fatal(e, a.Errors)
	}
	want := append([]byte{0x18, 18}, []byte(".help .quit\x00.exit\x00")...)
	want = append(want, 0xc9)
	if !bytes.Equal(a.Binary, want) {
		t.Fatalf("got%q want%q", a.Binary, want)
	}
}
