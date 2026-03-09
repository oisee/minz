package nanz_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
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

func TestLambda(t *testing.T) {
	// Non-capturing lambda: |x: u8| expr  →  anonymous fun lambda_0(x: u8) -> T
	src := `fun apply(arr: ^u8, n: u8) -> void {
    let cb = |x: u8| (x + 1)
    var i: u8 = 0
    while (i < n) {
        i = (i + 1)
    }
}

fun multiarg(arr: ^u8, n: u8) -> void {
    let f = |a: u8, b: u8| (a + b)
}

fun block_lambda(arr: ^u8, n: u8) -> void {
    let g = |x: u8| {
        return (x * 2)
    }
}

fun no_type(arr: ^u8, n: u8) -> void {
    let h = |x| (x + 1)
}
`
	m, err := nanz.Parse(src, "lambda_test")
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	// 4 explicit funs + 4 generated lambdas = 8
	if len(m.Funcs) != 8 {
		var names []string
		for _, f := range m.Funcs {
			names = append(names, f.Name)
		}
		t.Errorf("funcs: want 8 (4 explicit + 4 lambdas), got %d: %v", len(m.Funcs), names)
	}

	// Lambda functions should be named lambda_0..lambda_3
	lambdaNames := map[string]bool{}
	for _, f := range m.Funcs {
		if strings.HasPrefix(f.Name, "lambda_") {
			lambdaNames[f.Name] = true
		}
	}
	for _, want := range []string{"lambda_0", "lambda_1", "lambda_2", "lambda_3"} {
		if !lambdaNames[want] {
			t.Errorf("expected lambda %q in module funcs", want)
		}
	}

	// lambda_0 should have 1 param (x: u8), return u8
	var lambda0Found bool
	for _, f := range m.Funcs {
		if f.Name == "lambda_0" {
			lambda0Found = true
			if len(f.Params) != 1 {
				t.Errorf("lambda_0 params: want 1, got %d", len(f.Params))
			} else if f.Params[0].Name != "x" {
				t.Errorf("lambda_0 param[0].Name: want x, got %s", f.Params[0].Name)
			}
		}
	}
	if !lambda0Found {
		t.Error("lambda_0 not found in module")
	}

	// Roundtrip: print → parse must not error
	out := nanz.Print(m)
	_, err = nanz.Parse(out, "lambda_test2")
	if err != nil {
		t.Errorf("roundtrip parse failed: %v\n\nPrinted:\n%s", err, out)
	}
}

func TestIterChain(t *testing.T) {
	// Test 1: simple forEach — arr.forEach(|x: u8| { cb(x) }, n)
	src1 := `@extern fun cb(v: u8)
fun run(arr: ^u8, n: u8) {
    arr.forEach(|x: u8| { cb(x) }, n)
}
`
	m1, err := nanz.Parse(src1, "iter_chain_test")
	if err != nil {
		t.Fatalf("simple forEach parse: %v", err)
	}
	// 1 extern + 1 explicit + 1 lambda = 3 funcs
	if len(m1.Funcs) != 3 {
		var names []string
		for _, f := range m1.Funcs {
			names = append(names, f.Name)
		}
		t.Fatalf("simple forEach: want 3 funcs, got %d: %v", len(m1.Funcs), names)
	}
	// LowerModule must not panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("simple forEach: LowerModule panicked: %v", r)
			}
		}()
		hir.LowerModule(m1)
	}()

	// Test 2: map + forEach chain — arr.map(|x: u8| (x * 2)).forEach(|x: u8| { cb(x) }, n)
	src2 := `@extern fun cb(v: u8)
fun run(arr: ^u8, n: u8) {
    arr.map(|x: u8| (x * 2)).forEach(|x: u8| { cb(x) }, n)
}
`
	m2, err := nanz.Parse(src2, "iter_chain_map_test")
	if err != nil {
		t.Fatalf("map+forEach parse: %v", err)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("map+forEach: LowerModule panicked: %v", r)
			}
		}()
		hir.LowerModule(m2)
	}()

	// Test 3: filter + forEach chain — arr.filter(|x: u8| (x > 5)).forEach(|x: u8| { cb(x) }, n)
	src3 := `@extern fun cb(v: u8)
fun run(arr: ^u8, n: u8) {
    arr.filter(|x: u8| (x > 5)).forEach(|x: u8| { cb(x) }, n)
}
`
	m3, err := nanz.Parse(src3, "iter_chain_filter_test")
	if err != nil {
		t.Fatalf("filter+forEach parse: %v", err)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("filter+forEach: LowerModule panicked: %v", r)
			}
		}()
		hir.LowerModule(m3)
	}()

	// Test 4: full chain — arr.map(|x| x*2).filter(|x| x>5).forEach(|x| cb(x), n)
	src4 := `@extern fun cb(v: u8)
fun run(arr: ^u8, n: u8) {
    arr.map(|x: u8| (x * 2)).filter(|x: u8| (x > 5)).forEach(|x: u8| { cb(x) }, n)
}
`
	m4, err := nanz.Parse(src4, "iter_chain_full_test")
	if err != nil {
		t.Fatalf("full chain parse: %v", err)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("full chain: LowerModule panicked: %v", r)
			}
		}()
		hir.LowerModule(m4)
	}()

	// Roundtrip: Print → Parse must not error
	printed := nanz.Print(m4)
	if _, err := nanz.Parse(printed, "iter_chain_roundtrip"); err != nil {
		t.Errorf("iter chain roundtrip parse failed: %v\n\nPrinted:\n%s", err, printed)
	}

	// Verify UFCS output: the printed form should contain the lambda functions
	if !strings.Contains(printed, "lambda_") {
		t.Errorf("printed output missing lambda_ functions:\n%s", printed)
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

// ── RangedTy parsing ──────────────────────────────────────────────────────────

func TestParseRangedType_Param(t *testing.T) {
	// fun lut_sin(angle: u8<0..255>) -> u8 { return 0 }
	src := `fun lut_sin(angle: u8<0..255>) -> u8 { return 0 }`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("want 1 func, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if len(f.Params) != 1 {
		t.Fatalf("want 1 param, got %d", len(f.Params))
	}
	rt, ok := f.Params[0].Ty.(*mir2.RangedTy)
	if !ok {
		t.Fatalf("param type: want *RangedTy, got %T", f.Params[0].Ty)
	}
	if rt.Base != mir2.TyU8 {
		t.Errorf("base: want u8, got %s", rt.Base)
	}
	if rt.Lo != 0 || rt.Hi != 256 {
		// source: 0..255 (inclusive) → stored as [0, 256)
		t.Errorf("range: want [0,256), got [%d,%d)", rt.Lo, rt.Hi)
	}
}

func TestParseRangedType_ReturnType(t *testing.T) {
	// fun clamp(x: u8) -> u8<0..63> { return 0 }
	src := `fun clamp(x: u8) -> u8<0..63> { return 0 }`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	rt, ok := m.Funcs[0].RetTy.(*mir2.RangedTy)
	if !ok {
		t.Fatalf("return type: want *RangedTy, got %T", m.Funcs[0].RetTy)
	}
	if rt.Lo != 0 || rt.Hi != 64 {
		t.Errorf("range: want [0,64), got [%d,%d)", rt.Lo, rt.Hi)
	}
}

func TestParseRangedType_U16(t *testing.T) {
	// Ranged u16 param
	src := `fun table(i: u16<0..1023>) -> u16 { return 0 }`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	rt, ok := m.Funcs[0].Params[0].Ty.(*mir2.RangedTy)
	if !ok {
		t.Fatalf("param type: want *RangedTy, got %T", m.Funcs[0].Params[0].Ty)
	}
	if rt.Base != mir2.TyU16 {
		t.Errorf("base: want u16, got %s", rt.Base)
	}
	if rt.Lo != 0 || rt.Hi != 1024 {
		t.Errorf("range: want [0,1024), got [%d,%d)", rt.Lo, rt.Hi)
	}
}

func TestParseRangedType_String(t *testing.T) {
	// RangedTy.String() should be "u8<0..63>" (inclusive hi)
	rt := mir2.NewRanged(mir2.TyU8, 0, 64) // exclusive hi = 64
	want := "u8<0..63>"
	if rt.String() != want {
		t.Errorf("String(): want %q, got %q", want, rt.String())
	}
}

func TestParseRangedType_Helpers(t *testing.T) {
	rt := mir2.NewRanged(mir2.TyU8, 10, 20)

	if !mir2.IsRanged(rt) {
		t.Error("IsRanged should return true")
	}
	if mir2.IsRanged(mir2.TyU8) {
		t.Error("IsRanged should return false for plain u8")
	}

	lo, hi, ok := mir2.RangeOf(rt)
	if !ok || lo != 10 || hi != 20 {
		t.Errorf("RangeOf: want (10,20,true), got (%d,%d,%v)", lo, hi, ok)
	}
	if _, _, ok2 := mir2.RangeOf(mir2.TyU8); ok2 {
		t.Error("RangeOf(TyU8) should return ok=false")
	}

	if mir2.BaseOf(rt) != mir2.TyU8 {
		t.Errorf("BaseOf: want TyU8, got %s", mir2.BaseOf(rt))
	}
	if mir2.BaseOf(mir2.TyU16) != mir2.TyU16 {
		t.Error("BaseOf(TyU16) should be identity")
	}
}

// Unused import guard: hir is used elsewhere in the file.
var _ = (*hir.Module)(nil)
