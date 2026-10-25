package nanz_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

// Import the real library. Every backing byte, both canaries, failed outputs,
// and descriptor lengths are checked inside the workload (not just a checksum).
func TestByteBufferImportedNative(t *testing.T) {
	const source = `import mem.bytes
fun probe(seed: u8, capacity: u16) -> u16 {
    var storage: [u8; 258]
    var buffer: ByteBuffer
    var saved: ByteView
    var piece: ByteView
    var byte: u8 = 197
    var data: ^u8 = &storage
    var i: u16 = 0
    while i < 258 { data[i] = 197
        i = i + 1 }
    buffer.init(data + 1, capacity)
    i = 0
    while i < capacity {
        if buffer.try_push(seed + u8(i)) != 1 { return 1 }
        i = i + 1
    }
    if buffer.try_push(99) != 0 { return 2 }
    if buffer.len != capacity { return 3 }
    buffer.view(&saved)
    if saved.len != capacity { return 4 }
    // Check every byte, including unused capacity and the two external guards.
    i = 0
    while i < 258 {
        var expected: u8 = 197
        if i > 0 { if i <= capacity { expected = seed + u8(i - 1) } }
        if data[i] != expected { return 5 }
        i = i + 1
    }
    if saved.try_get(capacity, &byte) != 0 { return 6 }
    if byte != 197 { return 7 }
    if saved.try_set(capacity, 99) != 0 { return 8 }
    if data[capacity + 1] != 197 { return 30 }
    piece.init(data, 7)
    if saved.subview(65535, 2, &piece) != 0 { return 9 }
    if piece.len != 7 { return 10 }
    if piece.data != data { return 11 }
    if saved.subview(capacity, 1, &piece) != 0 { return 12 }
    if piece.len != 7 { return 13 }
    if saved.subview(capacity, 0, &piece) != 1 { return 14 }
    if piece.len != 0 { return 15 }
    if piece.data != data + 1 + capacity { return 16 }
    if capacity > 0 {
        if saved.subview(0, capacity, &piece) != 1 { return 17 }
        // Aliasing output with input is supported; this keeps the final byte.
        if piece.subview(capacity - 1, 1, &piece) != 1 { return 18 }
        if piece.try_get(0, &byte) != 1 { return 19 }
        if byte != seed + u8(capacity - 1) { return 20 }
        if piece.try_set(0, 42) != 1 { return 21 }
        if data[capacity] != 42 { return 22 }
    }
    buffer.clear()
    if buffer.len != 0 { return 23 }
    if buffer.cap != capacity { return 31 }
    if buffer.data != data + 1 { return 32 }
    if saved.len != capacity { return 24 }
    if capacity > 0 {
        if buffer.try_push(91) != 1 { return 25 }
        if saved.try_get(0, &byte) != 1 { return 26 }
        if byte != 91 { return 27 }
    }
    if data[0] != 197 { return 28 }
    if data[257] != 197 { return 29 }
    return 0
}
// A wrapped sum must not admit an out-of-bounds range. No backing read occurs.
fun overflowing_range() -> u16 {
    var storage: [u8; 1]
    var view: ByteView
    var out: ByteView
    view.init(&storage, 65535)
    out.init(&storage, 7)
    if view.subview(65534, 3, &out) != 0 { return 1 }
    if out.len != 7 { return 2 }
    return 0
}
fun empty_null() -> u16 {
    var null: ^u8 = 0
    var view: ByteView
    var out: ByteView
    var buffer: ByteBuffer
    var byte: u8 = 197
    view.init(null, 0)
    buffer.init(null, 0)
    if view.try_get(0, &byte) != 0 { return 1 }
    if byte != 197 { return 2 }
    if view.try_set(0, 42) != 0 { return 3 }
    if buffer.try_push(42) != 0 { return 4 }
    if view.subview(0, 0, &out) != 1 { return 5 }
    if out.data != null { return 6 }
    if out.len != 0 { return 7 }
    return 0
}
`
	m, err := nanz.ParseWithOpts(source, "byte-buffer-workload.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	m.NonNullObjects = true
	canonical := false
	for _, st := range m.Structs {
		if strings.HasSuffix(st.Name, "ByteView") {
			want, err := hir.BorrowedViewType(st.Name, mir2.TyU8)
			if err != nil || !reflect.DeepEqual(st, want) {
				t.Fatalf("library view differs from shared HIR descriptor: %v, %v", st, err)
			}
			canonical = true
		}
	}
	if !canonical {
		t.Fatal("import omitted ByteView")
	}
	for _, capacity := range []int64{0, 1, 3, 255, 256} {
		for _, seed := range []int64{0, 1, 127, 255} {
			m.Asserts = append(m.Asserts, hir.Assert{FuncName: "probe", Args: []int64{seed, capacity}, Expected: 0, Source: "bounded byte buffer full state"})
		}
	}
	m.Asserts = append(m.Asserts, hir.Assert{FuncName: "overflowing_range", Expected: 0, Source: "subview overflow rejection"})
	m.Asserts = append(m.Asserts, hir.Assert{FuncName: "empty_null", Expected: 0, Source: "empty null descriptors do not dereference"})
	if err := hir.Verify(m); err != nil {
		t.Fatal(err)
	}
	if err := pipeline.RunAssertsMIR2(m, hir.LowerModule(hir.CloneModule(m))); err != nil {
		t.Fatal(err)
	}
	if _, err := pipeline.CompileHIRSteps(m, pipeline.Options{DisableVariantSearch: true}); err != nil {
		t.Fatal(err)
	}
}
