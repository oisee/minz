package nanz_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestFactcheck_AddressOfGlobalParses(t *testing.T) {
	src := `
global counter: u8

fun ptr_to_counter() -> ptr {
    return &counter
}
`
	m, err := nanz.Parse(src, "addr_global_test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("funcs: want 1, got %d", len(m.Funcs))
	}
}

func TestFactcheck_AddressOfFieldParses(t *testing.T) {
	src := `
struct Point { x: u8 }
global p: Point

fun ptr_to_field() -> ptr {
    return &p.x
}
`
	m, err := nanz.Parse(src, "addr_field_test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	ret := m.Funcs[0].Body.Body[0].(*hir.ReturnStmt)
	ao, ok := ret.Val.(*hir.AddrOfExpr)
	if !ok {
		t.Fatalf("return expr: want AddrOfExpr, got %T", ret.Val)
	}
	fe, ok := ao.X.(*hir.FieldExpr)
	if !ok {
		t.Fatalf("addr-of target: want FieldExpr, got %T", ao.X)
	}
	if fe.Field != "x" || fe.Offset != 0 {
		t.Fatalf("field target mismatch: field=%q offset=%d", fe.Field, fe.Offset)
	}
}

func TestFactcheck_AddressOfFieldCompilesAndDerefs(t *testing.T) {
	src := `
struct Pair { a: u8, b: u8 }
global pair_g: Pair

fun write_then_read() -> u8 {
    let pb = &pair_g.b
    pb^ = 7
    return pb^
}

assert write_then_read() == 7 via mir2
`
	hm, err := nanz.Parse(src, "addr_field_compile_test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := pipeline.CompileHIR(hm); err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestFactcheck_AddressOfArrayIndexCompilesAndDerefs(t *testing.T) {
	src := `
global data: [u8; 4] = [10, 20, 30, 40]

fun read_index_ptr() -> u8 {
    var p: ptr
    p = &data[2]
    return p^
}

fun write_index_ptr() -> u8 {
    var p: ptr
    p = &data[1]
    p^ = 99
    return data[1]
}

assert read_index_ptr() == 30 via mir2
assert write_index_ptr() == 99 via mir2
`
	hm, err := nanz.Parse(src, "addr_index_compile_test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if _, err := pipeline.CompileHIR(hm); err != nil {
		t.Fatalf("compile: %v", err)
	}
}

func TestFactcheck_OffsetOfIsNotIntrinsic(t *testing.T) {
	src := `
struct Point { x: u8, y: u8 }

fun field_offset() -> u8 {
    return offsetOf(Point.x)
}
`
	m, err := nanz.Parse(src, "offsetof_test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	var f *hir.Func
	for _, fn := range m.Funcs {
		if fn.Name == "field_offset" {
			f = fn
			break
		}
	}
	if f == nil {
		t.Fatal("field_offset not found")
	}

	ret, ok := f.Body.Body[0].(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("body[0]: want ReturnStmt, got %T", f.Body.Body[0])
	}
	call, ok := ret.Val.(*hir.CallExpr)
	if !ok {
		t.Fatalf("return expr: want CallExpr, got %T", ret.Val)
	}
	if call.Fn != "offsetOf" {
		t.Fatalf("expected ordinary call to offsetOf, got %q", call.Fn)
	}
	if len(call.Args) != 1 {
		t.Fatalf("offsetOf args: want 1, got %d", len(call.Args))
	}
}

func TestFactcheck_BitAccessorSyntaxRejected(t *testing.T) {
	src := `
fun set_bit(v: u8) -> u8 {
    v.7 = 1
    return v
}
`
	_, err := nanz.Parse(src, "bit_accessor_test")
	if err == nil {
		t.Fatal("expected parse error for v.7 = 1")
	}
	t.Logf("got expected error: %v", err)
}

func TestFactcheck_AsmImportRejected(t *testing.T) {
	dir, err := os.MkdirTemp("", "nanz_asm_import")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	if err := os.WriteFile(filepath.Join(dir, "asmmod.a80"), []byte("ret\n"), 0o644); err != nil {
		t.Fatalf("write asm fixture: %v", err)
	}

	src := `import asmmod
fun main() -> u8 { return 0 }
`
	_, err = nanz.ParseWithOpts(src, "main.nanz", nanz.ParseOpts{
		BaseDir: dir,
	})
	if err == nil {
		t.Fatal("expected module-not-found error for asm import")
	}
	if !strings.Contains(err.Error(), "module not found") {
		t.Fatalf("unexpected asm import error: %v", err)
	}
}
