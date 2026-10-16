package mir2

import (
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestGlobalRelocationRejectsMalformedSlots(t *testing.T) {
	for _, r := range [][]GlobalRelocation{
		{{Offset: -1, Symbol: "p"}}, {{Offset: 3, Symbol: "p"}}, {{Offset: 0, Symbol: "p"}, {Offset: 1, Symbol: "p"}}, {{Offset: 0, Symbol: "missing"}}, {{Offset: 0, Symbol: "@mir2.str.99"}},
	} {
		m := &Module{Globals: []Global{{Name: "p", Ty: NewArray(TyU8, 4), Relocs: r}}, Funcs: []*Func{{Name: "noop", Blocks: []*Block{{Label: "entry", Term: &TermRet{}}}}}}
		if e := Verify(m); e == nil {
			t.Fatal("malformed relocation accepted", r)
		}
		if _, e := NewVM(m).Call("noop", nil); e == nil {
			t.Fatal("VM silently ignored malformed relocation", r)
		}
		if _, e := Z80Codegen(m, &AllocResult{}); e == nil {
			t.Fatal("native backend ignored malformed relocation", r)
		}
	}
}
func TestGlobalRelocationWraps16BitAddress(t *testing.T) {
	m := &Module{Globals: []Global{{Name: "data", Ty: TyU8, Init: []byte{7}}, {Name: "p", Ty: TyPtr, Relocs: []GlobalRelocation{{Symbol: "data", Addend: 65536}}}}}
	vm := NewVM(m)
	p := vm.ReadHeap(vm.GlobalAddr("p"), 2)
	got := uint16(p[0]) | uint16(p[1])<<8
	if got != uint16(vm.GlobalAddr("data")) {
		t.Fatal("relocation is not modulo 65536", got)
	}
	text, e := Z80Codegen(m, &AllocResult{})
	if e != nil || !strings.Contains(text, "DW data+65536") {
		t.Fatal(e, text)
	}
}

func TestGlobalRelocationNativeWrap(t *testing.T) {
	for _, off := range []int64{-65536, -1, 0, 1, 65536} {
		m := &Module{Globals: []Global{{Name: "data", Ty: TyU8, Init: []byte{7}}, {Name: "p", Ty: TyPtr, Relocs: []GlobalRelocation{{Symbol: "data", Addend: off}}}}}
		text, err := Z80Codegen(m, &AllocResult{})
		if err != nil {
			t.Fatal(err)
		}
		a, err := z80asm.NewAssembler().AssembleString("ORG 32768\n" + text)
		if err != nil {
			t.Fatal(err)
		}
		if len(a.Errors) > 0 {
			t.Fatal(a.Errors)
		}
		slot := a.Symbols["p"] - 32768
		got := uint16(a.Binary[slot]) | uint16(a.Binary[slot+1])<<8
		want := uint16(int64(a.Symbols["data"]) + off)
		if got != want {
			t.Fatalf("addend %d: native relocation %d want %d", off, got, want)
		}
	}
}
