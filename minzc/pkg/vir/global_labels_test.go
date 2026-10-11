package vir

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"testing"
)

func TestModuleGlobalsSharedLabels(t *testing.T) {
	m := &mir2.Module{Globals: []mir2.Global{
		{Name: "A", Ty: mir2.TyU8, Init: []byte{73}},
		{Name: "import.data-$", Ty: mir2.TyU16, Init: []byte{0x34, 0x12}},
	}}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + emitModuleGlobals(m))
	if err != nil || len(a.Errors) > 0 {
		t.Fatalf("assemble %v %v", err, a.Errors)
	}
	if string(a.Binary) != string([]byte{73, 0x34, 0x12}) {
		t.Fatalf("global BIN %x", a.Binary)
	}
	if a.Symbols["v_A"] != 0x8000 || a.Symbols["import_data__"] != 0x8001 {
		t.Fatalf("symbols %v", a.Symbols)
	}
}
