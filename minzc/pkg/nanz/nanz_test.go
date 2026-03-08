package nanz_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/nanz"
)

const sampleNanz = `struct Point {
    x: u8
    y: u8
}

global counter: u8
global vram: u8 at(0x4000)
global table: [u8; 4] = [1, 2, 4, 8]

fun add(a: u8, b: u8) -> u8 {
    return (a + b)
}

fun fill(buf: ^u8, len: u8) {
    var i: u8 = 0
    while (i < len) {
        buf[i] = 0
        i = (i + 1)
    }
}

fun fib(n: u16) -> u16 {
    if (n < 2) {
        return n
    }
    var a: u16 = 0
    var b: u16 = 1
    var i: u16 = 2
    while (i <= n) {
        var t: u16 = (a + b)
        a = b
        b = t
        i = (i + 1)
    }
    return b
}

fun io_example() {
    let port = @ptr(u8, 0xFE)
    let val = port^
    port^ = 255
}
`

func TestParse(t *testing.T) {
	m, err := nanz.Parse(sampleNanz, "test")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(m.Structs) != 1 {
		t.Errorf("structs: want 1, got %d", len(m.Structs))
	}
	if m.Structs[0].Name != "Point" {
		t.Errorf("struct name: want Point, got %s", m.Structs[0].Name)
	}
	if len(m.Globals) != 3 {
		t.Errorf("globals: want 3, got %d", len(m.Globals))
	}
	if m.Globals[1].At == nil || *m.Globals[1].At != 0x4000 {
		t.Errorf("vram.At: want 0x4000, got %v", m.Globals[1].At)
	}
	if len(m.Globals[2].Init) != 4 {
		t.Errorf("table init len: want 4, got %d", len(m.Globals[2].Init))
	}
	if len(m.Funcs) != 4 {
		t.Errorf("funcs: want 4, got %d", len(m.Funcs))
	}
}

func TestPrint(t *testing.T) {
	m, err := nanz.Parse(sampleNanz, "test")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	out := nanz.Print(m)
	// Must contain key constructs
	checks := []string{
		"struct Point",
		"global counter: u8",
		"global vram: u8 at(0x4000)",
		"global table: [u8; 4]",
		"fun add(a: u8, b: u8) -> u8",
		"fun fill(buf: ptr, len: u8)",
		"fun fib(n: u16) -> u16",
		"@ptr(u8, 0x00FE)",
		"port^",
	}
	for _, want := range checks {
		if !strings.Contains(out, want) {
			t.Errorf("Print output missing: %q\n\nGot:\n%s", want, out)
		}
	}
}

func TestParseRoundtrip(t *testing.T) {
	// Parse → Print → Parse again: second parse must not error
	m1, err := nanz.Parse(sampleNanz, "test")
	if err != nil {
		t.Fatalf("first parse: %v", err)
	}
	printed := nanz.Print(m1)
	_, err = nanz.Parse(printed, "test2")
	if err != nil {
		t.Errorf("second parse failed: %v\n\nPrinted:\n%s", err, printed)
	}
}

func TestForEach(t *testing.T) {
	src := `fun sum_bytes(buf: ^u8, len: u8) -> u16 {
    var total: u16 = 0
    for b: u8 in buf[0..len] {
        total = (total + u16(b))
    }
    return total
}
`
	m, err := nanz.Parse(src, "foreach_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("funcs: want 1, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if f.Name != "sum_bytes" {
		t.Errorf("func name: want sum_bytes, got %s", f.Name)
	}
	// Body should contain: VarDeclStmt, ForEachStmt, ReturnStmt
	if len(f.Body.Body) != 3 {
		t.Errorf("body stmts: want 3, got %d", len(f.Body.Body))
	}

	// Check roundtrip
	printed := nanz.Print(m)
	_, err = nanz.Parse(printed, "foreach_test2")
	if err != nil {
		t.Errorf("roundtrip parse failed: %v\n\nPrinted:\n%s", err, printed)
	}
	if !strings.Contains(printed, "for b: u8 in") {
		t.Errorf("printed output missing for-each loop:\n%s", printed)
	}
}

func TestAtDecl(t *testing.T) {
	src := `global port: u8 at(0xFE)
global screen: [u8; 6912] at(0x4000)
`
	m, err := nanz.Parse(src, "at_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Globals) != 2 {
		t.Fatalf("globals: want 2, got %d", len(m.Globals))
	}
	if m.Globals[0].At == nil || *m.Globals[0].At != 0xFE {
		t.Errorf("port.At: want 0xFE")
	}
	if m.Globals[1].At == nil || *m.Globals[1].At != 0x4000 {
		t.Errorf("screen.At: want 0x4000")
	}
}
