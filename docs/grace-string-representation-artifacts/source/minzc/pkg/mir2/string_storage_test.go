package mir2

import (
	"bytes"
	"strings"
	"testing"
)

func TestStringStorageKinds(t *testing.T) {
	m := &Module{Name: "storage"}
	cases := []struct {
		k    StringKind
		s    string
		want []byte
	}{{StrSString, "AB", []byte{2, 65, 66}}, {StrCString, "AB", []byte{65, 66, 0}}, {StrLString, "AB", []byte{2, 0, 65, 66}}, {StrSString, "", []byte{0}}, {StrCString, "", []byte{0}}, {StrLString, "", []byte{0, 0}}, {StrSString, "A\x00B", []byte{3, 65, 0, 66}}}
	vm := NewVM(m)
	seen := map[int64]bool{}
	for _, c := range cases {
		sym, _ := m.Strings.InternKind(c.s, c.k)
		off, e := vm.ResolveSymbol(sym)
		if e != nil {
			t.Fatal(e)
		}
		if seen[off] {
			t.Fatal("different representation aliases")
		}
		seen[off] = true
		if !bytes.Equal(vm.ReadHeap(off, len(c.want)), c.want) {
			t.Fatalf("kind%d %v", c.k, vm.ReadHeap(off, len(c.want)))
		}
		again, e := vm.ResolveSymbol(sym)
		if e != nil || again != off {
			t.Fatal("cached identity", e)
		}
	}
}
func TestStringStorageBounds(t *testing.T) {
	for _, c := range []struct {
		k  StringKind
		n  int
		ok bool
	}{{StrSString, 255, true}, {StrSString, 256, false}, {StrLString, 65535, true}, {StrLString, 65536, false}} {
		m := &Module{Name: "bounds"}
		sym, _ := m.Strings.InternKind(strings.Repeat("x", c.n), c.k)
		vm := NewVM(m)
		vm.MaxMemory = 70000 // Representation bound, not a 64K executable layout claim.
		_, e := vm.ResolveSymbol(sym)
		if (e == nil) != c.ok {
			t.Fatalf("kind%d n%d err%v", c.k, c.n, e)
		}
		a, e := Z80Codegen(m, &AllocResult{})
		if (e == nil) != c.ok {
			t.Fatalf("emitter kind%d n%d err%v", c.k, c.n, e)
		}
		if !c.ok && a != "" {
			t.Fatal("partial assembly on refusal")
		}
	}
}

func TestStringStorageMemoryLimitAtomic(t *testing.T) {
	m := &Module{Name: "memorylimit"}
	sym, _ := m.Strings.InternKind(strings.Repeat("x", 65535), StrLString)
	vm := NewVM(m)
	before := len(vm.heap)
	if _, e := vm.ResolveSymbol(sym); e == nil {
		t.Fatal("65537-byte encoding fit default64K")
	}
	if len(vm.heap) != before {
		t.Fatal("partial string storage after refusal")
	}
	if _, ok := vm.globalSyms[sym]; ok {
		t.Fatal("failed symbol cached")
	}
	vm.MaxMemory = 70000
	if _, e := vm.ResolveSymbol(sym); e != nil {
		t.Fatal("explicit larger VM storage", e)
	}
}
