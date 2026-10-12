package pipeline

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestExternStubSharedLabels(t *testing.T) {
	m := &mir2.Module{Funcs: []*mir2.Func{{Name: "A"}, {Name: "import.func-$"}}}
	source := emitExternStubs("ORG 0x8000\nCALL v_A\nCALL import_func__\nRET\n", m)
	a, err := z80asm.NewAssembler().AssembleString(source)
	if err != nil || len(a.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, a.Errors)
	}
	want := []byte{0xcd, 0x07, 0x80, 0xcd, 0x08, 0x80, 0xc9, 0xc9, 0xc9}
	if string(a.Binary) != string(want) {
		t.Fatalf("stub BIN %x want %x", a.Binary, want)
	}
}
