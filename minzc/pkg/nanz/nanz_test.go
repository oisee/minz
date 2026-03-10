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

// ── Struct methods (fun TypeName.method) ──────────────────────────────────────

func TestStructMethods(t *testing.T) {
	src := `struct Vec2 {
    x: u8
    y: u8
}

fun Vec2.add(self: Vec2, other: Vec2) -> Vec2 {
    return self
}

fun Vec2.scale(self: Vec2, factor: u8) -> Vec2 {
    return self
}

fun use_vec(a: Vec2, b: Vec2) -> Vec2 {
    var c: Vec2
    c = a.add(b)
    return c
}
`
	m, err := nanz.Parse(src, "struct_method_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Expect: Vec2_add, Vec2_scale, use_vec (3 funcs total)
	funcNames := make(map[string]bool)
	for _, f := range m.Funcs {
		funcNames[f.Name] = true
	}
	if !funcNames["Vec2_add"] {
		t.Errorf("expected function named Vec2_add, got funcs: %v", funcNames)
	}
	if !funcNames["Vec2_scale"] {
		t.Errorf("expected function named Vec2_scale, got funcs: %v", funcNames)
	}
	if len(m.Funcs) != 3 {
		t.Errorf("funcs: want 3, got %d: %v", len(m.Funcs), funcNames)
	}

	// Vec2_add must have 2 params of struct type
	var addFunc *hir.Func
	for _, f := range m.Funcs {
		if f.Name == "Vec2_add" {
			addFunc = f
		}
	}
	if addFunc == nil {
		t.Fatal("Vec2_add not found")
	}
	if len(addFunc.Params) != 2 {
		t.Errorf("Vec2_add params: want 2, got %d", len(addFunc.Params))
	}
	if addFunc.Params[0].Name != "self" {
		t.Errorf("Vec2_add param[0]: want 'self', got %q", addFunc.Params[0].Name)
	}
}

func TestStructMethodUFCS(t *testing.T) {
	// When v: Vec2 and Vec2.add is declared, v.add(other) → CallExpr{Fn:"Vec2_add"}
	src := `struct Vec2 {
    x: u8
    y: u8
}

fun Vec2.len(self: Vec2) -> u8 {
    return self.x
}

fun compute(v: Vec2) -> u8 {
    return v.len()
}
`
	m, err := nanz.Parse(src, "ufcs_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Find "compute" and inspect its body
	var compute *hir.Func
	for _, f := range m.Funcs {
		if f.Name == "compute" {
			compute = f
		}
	}
	if compute == nil {
		t.Fatal("compute not found")
	}

	// Body[0] should be ReturnStmt with CallExpr{Fn: "Vec2_len"}
	ret, ok := compute.Body.Body[0].(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("body[0]: want ReturnStmt, got %T", compute.Body.Body[0])
	}
	call, ok := ret.Val.(*hir.CallExpr)
	if !ok {
		t.Fatalf("return val: want CallExpr, got %T", ret.Val)
	}
	if call.Fn != "Vec2_len" {
		t.Errorf("UFCS dispatch: want Fn=Vec2_len, got %q", call.Fn)
	}
	if len(call.Args) != 1 {
		t.Errorf("UFCS args: want 1 (self), got %d", len(call.Args))
	}
}

// ── Operator overloading ───────────────────────────────────────────────────────

func TestOperatorOverloading(t *testing.T) {
	src := `struct Vec2 {
    x: u8
    y: u8
}

fun +(a: Vec2, b: Vec2) -> Vec2 {
    return a
}

fun -(a: Vec2, b: Vec2) -> Vec2 {
    return a
}

fun compute(a: Vec2, b: Vec2) -> Vec2 {
    return a + b
}
`
	m, err := nanz.Parse(src, "op_overload_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// op_add and op_sub functions should exist
	funcNames := make(map[string]bool)
	for _, f := range m.Funcs {
		funcNames[f.Name] = true
	}
	if !funcNames["op_add"] {
		t.Errorf("expected function op_add, got: %v", funcNames)
	}
	if !funcNames["op_sub"] {
		t.Errorf("expected function op_sub, got: %v", funcNames)
	}

	// compute body: return a + b → CallExpr{Fn:"op_add"}
	var compute *hir.Func
	for _, f := range m.Funcs {
		if f.Name == "compute" {
			compute = f
		}
	}
	if compute == nil {
		t.Fatal("compute not found")
	}
	ret, ok := compute.Body.Body[0].(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("body[0]: want ReturnStmt, got %T", compute.Body.Body[0])
	}
	call, ok := ret.Val.(*hir.CallExpr)
	if !ok {
		t.Fatalf("return val: want CallExpr{op_add}, got %T", ret.Val)
	}
	if call.Fn != "op_add" {
		t.Errorf("op dispatch: want op_add, got %q", call.Fn)
	}
	if len(call.Args) != 2 {
		t.Errorf("op_add args: want 2, got %d", len(call.Args))
	}
}

func TestOperatorNoOverloadForPrimitives(t *testing.T) {
	// Even when op_add is declared for Vec2, primitive u8 + u8 should stay BinExpr
	src := `struct Vec2 { x: u8, y: u8 }

fun +(a: Vec2, b: Vec2) -> Vec2 { return a }

fun prim(x: u8, y: u8) -> u8 {
    return x + y
}
`
	m, err := nanz.Parse(src, "op_prim_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var prim *hir.Func
	for _, f := range m.Funcs {
		if f.Name == "prim" {
			prim = f
		}
	}
	if prim == nil {
		t.Fatal("prim not found")
	}
	ret, ok := prim.Body.Body[0].(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("body[0]: want ReturnStmt, got %T", prim.Body.Body[0])
	}
	// x + y with u8 operands should remain BinExpr (not CallExpr)
	if _, isBin := ret.Val.(*hir.BinExpr); !isBin {
		t.Errorf("primitive + should stay BinExpr, got %T", ret.Val)
	}
}

// ── Struct field offset resolution ────────────────────────────────────────────

func TestStructFieldOffsets(t *testing.T) {
	// struct Color { r: u8, g: u8, b: u8 }
	// Field offsets: r=0, g=1, b=2
	src := `struct Color {
    r: u8
    g: u8
    b: u8
}

fun get_green(c: Color) -> u8 {
    return c.g
}

fun get_blue(c: Color) -> u8 {
    return c.b
}
`
	m, err := nanz.Parse(src, "field_offset_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Find get_green and get_blue
	funcs := make(map[string]*hir.Func)
	for _, f := range m.Funcs {
		funcs[f.Name] = f
	}

	// get_green: return c.g → FieldExpr{Field:"g", Offset:1}
	fg := funcs["get_green"]
	if fg == nil {
		t.Fatal("get_green not found")
	}
	ret1 := fg.Body.Body[0].(*hir.ReturnStmt)
	field1 := ret1.Val.(*hir.FieldExpr)
	if field1.Field != "g" {
		t.Errorf("field name: want g, got %q", field1.Field)
	}
	if field1.Offset != 1 {
		t.Errorf("g offset: want 1, got %d", field1.Offset)
	}

	// get_blue: return c.b → FieldExpr{Field:"b", Offset:2}
	fb := funcs["get_blue"]
	if fb == nil {
		t.Fatal("get_blue not found")
	}
	ret2 := fb.Body.Body[0].(*hir.ReturnStmt)
	field2 := ret2.Val.(*hir.FieldExpr)
	if field2.Field != "b" {
		t.Errorf("field name: want b, got %q", field2.Field)
	}
	if field2.Offset != 2 {
		t.Errorf("b offset: want 2, got %d", field2.Offset)
	}
}

func TestStructFieldOffsets_U16(t *testing.T) {
	// Mixed widths: struct Vec3d { x: u16, y: u16, z: u8 }
	// x=0 (2 bytes), y=2 (2 bytes), z=4 (1 byte)
	src := `struct Vec3d {
    x: u16
    y: u16
    z: u8
}

fun get_z(v: Vec3d) -> u8 {
    return v.z
}
`
	m, err := nanz.Parse(src, "field_offset_u16_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var getZ *hir.Func
	for _, f := range m.Funcs {
		if f.Name == "get_z" {
			getZ = f
		}
	}
	if getZ == nil {
		t.Fatal("get_z not found")
	}
	ret := getZ.Body.Body[0].(*hir.ReturnStmt)
	field := ret.Val.(*hir.FieldExpr)
	if field.Offset != 4 {
		t.Errorf("z offset: want 4 (2+2+0), got %d", field.Offset)
	}
}

// ── Interface declaration ─────────────────────────────────────────────────────

func TestInterfaceDecl(t *testing.T) {
	src := `
struct Dog {
    name: u16
}

interface Animal {
    speak
    move
}

fun Dog.speak(self: Dog) -> void {
    self[0] = 1
}
`
	m, err := nanz.Parse(src, "interface_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Interfaces) != 1 {
		t.Fatalf("interfaces: want 1, got %d", len(m.Interfaces))
	}
	iface := m.Interfaces[0]
	if iface.Name != "Animal" {
		t.Errorf("interface name: want Animal, got %s", iface.Name)
	}
	if len(iface.Methods) != 2 {
		t.Fatalf("interface methods: want 2, got %d: %v", len(iface.Methods), iface.Methods)
	}
	if iface.Methods[0] != "speak" {
		t.Errorf("method[0]: want speak, got %s", iface.Methods[0])
	}
	if iface.Methods[1] != "move" {
		t.Errorf("method[1]: want move, got %s", iface.Methods[1])
	}
	// Verify struct still parsed
	if len(m.Structs) != 1 || m.Structs[0].Name != "Dog" {
		t.Errorf("struct Dog not found")
	}
	// Roundtrip
	printed := nanz.Print(m)
	if !strings.Contains(printed, "interface Animal") {
		t.Errorf("printed output missing 'interface Animal':\n%s", printed)
	}
	if !strings.Contains(printed, "speak") {
		t.Errorf("printed output missing method 'speak':\n%s", printed)
	}
	m2, err := nanz.Parse(printed, "interface_test2")
	if err != nil {
		t.Errorf("roundtrip parse failed: %v\n\nPrinted:\n%s", err, printed)
	}
	if len(m2.Interfaces) != 1 || m2.Interfaces[0].Name != "Animal" {
		t.Errorf("roundtrip: interface Animal not found")
	}
}

func TestStructParamUFCS(t *testing.T) {
	// Verify that a struct-typed parameter resolves UFCS dispatch correctly:
	// a.speak() where a: Dog → Dog_speak(a, ...)
	// and that LowerModule doesn't panic (classForParam handles *mir2.StructTy).
	src := `
struct Dog {
    name: u8
}

fun Dog.speak(self: Dog) -> void {
    self[0] = 1
}

fun make_sound(a: Dog) -> void {
    a.speak()
}
`
	m, err := nanz.Parse(src, "struct_param_ufcs_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Find make_sound and verify its body contains a call to Dog_speak.
	var makeSoundFunc *hir.Func
	for _, f := range m.Funcs {
		if f.Name == "make_sound" {
			makeSoundFunc = f
			break
		}
	}
	if makeSoundFunc == nil {
		t.Fatal("make_sound function not found")
	}
	if len(makeSoundFunc.Body.Body) == 0 {
		t.Fatal("make_sound body is empty")
	}
	exprStmt, ok := makeSoundFunc.Body.Body[0].(*hir.ExprStmt)
	if !ok {
		t.Fatalf("make_sound body[0]: want ExprStmt, got %T", makeSoundFunc.Body.Body[0])
	}
	callExpr, ok := exprStmt.Expr.(*hir.CallExpr)
	if !ok {
		t.Fatalf("make_sound body[0] expr: want CallExpr, got %T", exprStmt.Expr)
	}
	if callExpr.Fn != "Dog_speak" {
		t.Errorf("UFCS dispatch: want Dog_speak, got %s", callExpr.Fn)
	}

	// LowerModule must not panic (classForParam must handle *mir2.StructTy).
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("LowerModule panicked: %v", r)
			}
		}()
		hir.LowerModule(m)
	}()
}

// TestVarRefExprTy_GlobalStruct verifies that a global struct variable referenced
// in a call site gets its correct type (*mir2.StructTy), not the hardcoded TyU8.
// This is the fix for Bug A (LD A, HL invalid Z80 instruction).
func TestVarRefExprTy_GlobalStruct(t *testing.T) {
	src := `
struct Dog { sound: u8 }
global g_dog: Dog

fun Dog.bark(self: Dog) -> u8 {
    return self[0]
}

fun test() -> u8 {
    return g_dog.bark()
}
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}

	// Find the function named "test".
	testFn := m.FuncByName("test")
	if testFn == nil {
		t.Fatal("function 'test' not found")
	}

	// Walk the body looking for VarRefExpr{Name: "g_dog"}.
	// After the fix, it must NOT have type TyU8.
	found := false
	for _, s := range testFn.Body.Body {
		if rs, ok := s.(*hir.ReturnStmt); ok && rs.Val != nil {
			walkExpr(t, rs.Val, "g_dog", &found)
		}
	}

	// The key assertion: LowerModule must not panic (previously it panicked or
	// emitted invalid LD A,HL because g_dog had TyU8 instead of *StructTy).
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("LowerModule panicked (VarRefExpr.Ty bug): %v", r)
			}
		}()
		hir.LowerModule(m)
	}()
}

func walkExpr(t *testing.T, e hir.Expr, name string, found *bool) {
	t.Helper()
	switch ex := e.(type) {
	case *hir.VarRefExpr:
		if ex.Name == name {
			*found = true
			if ex.Ty == mir2.TyU8 {
				t.Errorf("VarRefExpr{%q}.Ty = TyU8 (hardcoded); expected struct type after fix", name)
			}
		}
	case *hir.CallExpr:
		for _, a := range ex.Args {
			walkExpr(t, a, name, found)
		}
	}
}

// TestLambdaCapture verifies that a fused forEach lambda that writes to an
// outer local variable (closure capture) does not panic during LowerModule.
//
// The lambda |x: u8| { s = s + x } captures `s` from the enclosing function.
// Before the fix, LowerModule tried to lower lambda_0 as a standalone function
// and panicked with "undefined variable s". After the fix, hasFreeVars detects
// the free variable and skips standalone lowering; the lambda is only ever
// inlined by lowerFusedForEach, where `s` is correctly threaded as a block param.
func TestLambdaCapture(t *testing.T) {
	src := `
fun sum_chain(buf: ^u8, n: u8) -> u8 {
    var s: u8 = 0
    buf.forEach(|x: u8| { s = (s + x) }, n)
    return s
}
`
	m, err := nanz.Parse(src, "lambda_capture_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// LowerModule must not panic.
	var mir2mod interface{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("LowerModule panicked (closure capture bug): %v", r)
			}
		}()
		mir2mod = hir.LowerModule(m)
	}()

	// The resulting MIR2 module must contain sum_chain (not skipped).
	type namer interface{ FuncByName(string) interface{} }
	if mir2mod == nil {
		t.Fatal("LowerModule returned nil")
	}
	// Verify codegen produces non-empty assembly.
	mirmod := mir2mod.(*mir2.Module)
	if mirmod.FuncByName("sum_chain") == nil {
		t.Error("sum_chain not found in lowered module")
	}
	// lambda_0 has a free variable 's' → it must NOT appear as a standalone MIR2 func.
	if mirmod.FuncByName("lambda_0") != nil {
		t.Error("lambda_0 should not be a standalone MIR2 function (it's inlined)")
	}
}

// TestMapInPlace verifies that buf.mapInPlace(|x: u8| x+2, n) is recognised as
// an iterator chain, lowers without panic, and emits a Store-back in the loop.
func TestMapInPlace(t *testing.T) {
	src := `
fun run(buf: ^u8, n: u8) {
    buf.mapInPlace(|x: u8| (x + 2), n)
}
`
	m, err := nanz.Parse(src, "mapinplace_test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	var mirmod *mir2.Module
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("LowerModule panicked: %v", r)
			}
		}()
		mirmod = hir.LowerModule(m)
	}()
	fn := mirmod.FuncByName("run")
	if fn == nil {
		t.Fatal("run not found")
	}
	// Verify a Store instruction exists (the write-back).
	foundStore := false
	for _, blk := range fn.Blocks {
		for _, inst := range blk.Insts {
			if inst.Op == mir2.OpStore {
				foundStore = true
			}
		}
	}
	if !foundStore {
		t.Error("mapInPlace: expected OpStore write-back in MIR2, found none")
	}
}

// TestMapInPlaceWithFilter verifies that filter + mapInPlace fusion works.
func TestMapInPlaceWithFilter(t *testing.T) {
	src := `
fun run(buf: ^u8, n: u8) {
    buf.filter(|x: u8| (x > 0)).mapInPlace(|x: u8| (x * 2), n)
}
`
	m, err := nanz.Parse(src, "mapinplace_filter_test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("LowerModule panicked: %v", r)
			}
		}()
		hir.LowerModule(m)
	}()
}

// ── Interface as parameter type ───────────────────────────────────────────────

// TestInterfaceParamType_UniqueImpl: fun feed(a: Animal) { a.speak() }
// Only Dog implements Animal → dispatch monomorphizes to Dog_speak.
func TestInterfaceParamType_UniqueImpl(t *testing.T) {
	src := `
interface Animal { speak }
struct Dog {}
fun Dog.speak(self: Dog) -> u8 { return 1 }

fun feed(a: Animal) -> u8 {
    return a.speak()
}
`
	m, err := nanz.Parse(src, "iface_param_unique")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	feed := m.FuncByName("feed")
	if feed == nil {
		t.Fatal("feed not found")
	}
	ret, ok := feed.Body.Body[0].(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("body[0]: want ReturnStmt, got %T", feed.Body.Body[0])
	}
	call, ok := ret.Val.(*hir.CallExpr)
	if !ok {
		t.Fatalf("return value: want CallExpr, got %T", ret.Val)
	}
	if call.Fn != "Dog_speak" {
		t.Errorf("expected call to Dog_speak, got %q", call.Fn)
	}
}

// TestInterfaceParamType_AmbiguousImpl: two structs implement the interface →
// compiler must return an error (ambiguous, use concrete type).
func TestInterfaceParamType_AmbiguousImpl(t *testing.T) {
	src := `
interface Animal { speak }
struct Dog {}
struct Cat {}
fun Dog.speak(self: Dog) -> u8 { return 1 }
fun Cat.speak(self: Cat) -> u8 { return 2 }

fun feed(a: Animal) -> u8 {
    return a.speak()
}
`
	_, err := nanz.Parse(src, "iface_param_ambiguous")
	if err == nil {
		t.Fatal("expected error for ambiguous interface dispatch, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected 'ambiguous' in error, got: %v", err)
	}
}

// TestInterfaceGlobalType: global declared with interface type.
func TestInterfaceGlobalType(t *testing.T) {
	src := `
interface Drawable { draw }
struct Sprite {}
fun Sprite.draw(self: Sprite) -> void {}

global g_thing: Drawable

fun render() -> void {
    g_thing.draw()
}
`
	m, err := nanz.Parse(src, "iface_global")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	render := m.FuncByName("render")
	if render == nil {
		t.Fatal("render not found")
	}
	expr, ok := render.Body.Body[0].(*hir.ExprStmt)
	if !ok {
		t.Fatalf("body[0]: want ExprStmt, got %T", render.Body.Body[0])
	}
	call, ok := expr.Expr.(*hir.CallExpr)
	if !ok {
		t.Fatalf("expr: want CallExpr, got %T", expr.Expr)
	}
	if call.Fn != "Sprite_draw" {
		t.Errorf("expected call to Sprite_draw, got %q", call.Fn)
	}
}

// Unused import guard: hir is used elsewhere in the file.
var _ = (*hir.Module)(nil)

// ── use-before-init warnings ──────────────────────────────────────────────────

// TestUseBeforeInit_WarnOnUninitVar verifies that a var declared without an
// initializer and then passed directly to a function triggers a warning.
func TestUseBeforeInit_WarnOnUninitVar(t *testing.T) {
	src := `
fun sink(p: ^u8) -> void {}

fun test() -> void {
    var ptr: ^u8
    sink(ptr)
}
`
	m, err := nanz.Parse(src, "ubi_test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(m.Warnings) == 0 {
		t.Fatal("expected at least one warning for use-before-init, got none")
	}
	found := false
	for _, w := range m.Warnings {
		if strings.Contains(w, "ptr") && strings.Contains(w, "before initialization") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning mentioning 'ptr' and 'before initialization', got: %v", m.Warnings)
	}
}

// TestUseBeforeInit_NoWarnAfterAssign verifies that assigning a var before use
// suppresses the warning.
func TestUseBeforeInit_NoWarnAfterAssign(t *testing.T) {
	src := `
@extern fun get_ptr() -> ^u8
fun sink(p: ^u8) -> void {}

fun test() -> void {
    var ptr: ^u8
    ptr = get_ptr()
    sink(ptr)
}
`
	m, err := nanz.Parse(src, "ubi_no_warn")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	for _, w := range m.Warnings {
		if strings.Contains(w, "ptr") {
			t.Errorf("unexpected warning for 'ptr' (it was assigned before use): %s", w)
		}
	}
}

// TestUseBeforeInit_NoWarnOnInitDecl verifies that var with initializer produces no warning.
func TestUseBeforeInit_NoWarnOnInitDecl(t *testing.T) {
	src := `
fun sink(v: u8) -> void {}

fun test() -> void {
    var x: u8 = 42
    sink(x)
}
`
	m, err := nanz.Parse(src, "ubi_init_decl")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	for _, w := range m.Warnings {
		if strings.Contains(w, "x") && strings.Contains(w, "before initialization") {
			t.Errorf("unexpected use-before-init warning for initialized var: %s", w)
		}
	}
}

// TestUseBeforeInit_ReturnUninit verifies that returning an uninitialized var warns.
func TestUseBeforeInit_ReturnUninit(t *testing.T) {
	src := `
fun get_val() -> u8 {
    var v: u8
    return v
}
`
	m, err := nanz.Parse(src, "ubi_return")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	found := false
	for _, w := range m.Warnings {
		if strings.Contains(w, "v") && strings.Contains(w, "before initialization") {
			found = true
		}
	}
	if !found {
		t.Errorf("expected warning for returning uninitialized 'v', got: %v", m.Warnings)
	}
}
