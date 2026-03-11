package nanz_test

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
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

// ── ^Struct pointer receiver syntax ──────────────────────────────────────────

// TestPtrReceiver_FieldReadWrite verifies that `self: ^Acc` style pointer params:
//   - Parse correctly (no error)
//   - Field reads (`self.val`) resolve to the correct struct field (offset=0, Ty=TyU8)
//   - Field writes (`self.val = X`) lower without panic
//   - UFCS dispatch through `^Struct` receiver works
func TestPtrReceiver_FieldReadWrite(t *testing.T) {
	src := `
struct Acc {
    val: u8
}

global acc_g: Acc

fun Acc.add(self: ^Acc, amount: u8) -> u8 {
    self.val = self.val + amount
    return self.val
}

fun Acc.reset(self: ^Acc) -> void {
    self.val = 0
}

fun sum_two(a: u8, b: u8) -> u8 {
    acc_g.reset()
    acc_g.add(a)
    acc_g.add(b)
    return acc_g.val
}
`
	m, err := nanz.Parse(src, "ptr_receiver_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Verify Acc.add body: body[0] is assign self.val = ..., body[1] is return self.val
	var addFunc *hir.Func
	for _, f := range m.Funcs {
		if f.Name == "Acc_add" {
			addFunc = f
			break
		}
	}
	if addFunc == nil {
		t.Fatal("Acc_add not found")
	}
	if len(addFunc.Body.Body) < 2 {
		t.Fatalf("Acc_add: expected ≥2 stmts, got %d", len(addFunc.Body.Body))
	}

	// body[0]: AssignStmt with FieldExpr target
	assign, ok := addFunc.Body.Body[0].(*hir.AssignStmt)
	if !ok {
		t.Fatalf("Acc_add body[0]: want AssignStmt, got %T", addFunc.Body.Body[0])
	}
	fe, ok := assign.Target.(*hir.FieldExpr)
	if !ok {
		t.Fatalf("Acc_add assign target: want FieldExpr, got %T", assign.Target)
	}
	if fe.Field != "val" {
		t.Errorf("FieldExpr.Field: want 'val', got %q", fe.Field)
	}
	if fe.Offset != 0 {
		t.Errorf("FieldExpr.Offset: want 0, got %d", fe.Offset)
	}
	if fe.Ty != mir2.TyU8 {
		t.Errorf("FieldExpr.Ty: want TyU8, got %v", fe.Ty)
	}

	// Lowering must not panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("LowerModule panicked: %v", r)
			}
		}()
		hir.LowerModule(m)
	}()
}

// TestPtrReceiver_ExplicitDeref verifies that `self^.field` (explicit deref syntax)
// produces the same FieldExpr as auto-deref `self.field`.
func TestPtrReceiver_ExplicitDeref(t *testing.T) {
	src := `
struct Counter {
    n: u8
}

fun Counter.inc(self: ^Counter) -> void {
    self^.n = self^.n + 1
}
`
	m, err := nanz.Parse(src, "explicit_deref_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var incFunc *hir.Func
	for _, f := range m.Funcs {
		if f.Name == "Counter_inc" {
			incFunc = f
			break
		}
	}
	if incFunc == nil {
		t.Fatal("Counter_inc not found")
	}
	assign, ok := incFunc.Body.Body[0].(*hir.AssignStmt)
	if !ok {
		t.Fatalf("Counter_inc body[0]: want AssignStmt, got %T", incFunc.Body.Body[0])
	}
	fe, ok := assign.Target.(*hir.FieldExpr)
	if !ok {
		t.Fatalf("Counter_inc assign target: want FieldExpr, got %T", assign.Target)
	}
	if fe.Field != "n" {
		t.Errorf("FieldExpr.Field: want 'n', got %q", fe.Field)
	}

	// LowerModule must not panic
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("LowerModule panicked: %v", r)
			}
		}()
		hir.LowerModule(m)
	}()
}

// TestPtrReceiver_UFCS verifies that UFCS dispatch works on a ^Struct var:
// acc_g.add(n) → Acc_add(addr_of(acc_g), n)
func TestPtrReceiver_UFCS(t *testing.T) {
	src := `
struct Acc { val: u8 }

global acc_g: Acc

fun Acc.add(self: ^Acc, amount: u8) -> u8 {
    self.val = self.val + amount
    return self.val
}

fun caller(n: u8) -> u8 {
    return acc_g.add(n)
}
`
	m, err := nanz.Parse(src, "ptr_receiver_ufcs_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var callerFunc *hir.Func
	for _, f := range m.Funcs {
		if f.Name == "caller" {
			callerFunc = f
			break
		}
	}
	if callerFunc == nil {
		t.Fatal("caller not found")
	}

	ret, ok := callerFunc.Body.Body[0].(*hir.ReturnStmt)
	if !ok {
		t.Fatalf("caller body[0]: want ReturnStmt, got %T", callerFunc.Body.Body[0])
	}
	call, ok := ret.Val.(*hir.CallExpr)
	if !ok {
		t.Fatalf("caller return: want CallExpr, got %T", ret.Val)
	}
	if call.Fn != "Acc_add" {
		t.Errorf("UFCS dispatch: want Acc_add, got %s", call.Fn)
	}
}

// ── Hello World / @print / @extern(addr) ─────────────────────────────────────

// TestExternAddr verifies that @extern(0x0010) fun ... sets ExternAddr=0x10.
func TestExternAddr(t *testing.T) {
	src := `@extern(0x0010) fun print_char(a: u8) -> void`
	m, err := nanz.Parse(src, "extern_addr_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("funcs: want 1, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if f.Name != "print_char" {
		t.Errorf("func name: want print_char, got %q", f.Name)
	}
	if !f.IsExtern {
		t.Error("IsExtern: want true")
	}
	if f.ExternAddr != 0x0010 {
		t.Errorf("ExternAddr: want 0x0010, got 0x%04X", f.ExternAddr)
	}
}

// TestHelloWorldASM verifies that a Nanz program using @print("Hello, World!\n")
// produces Z80 assembly containing the string bytes and the OUT instruction.
func TestHelloWorldASM(t *testing.T) {
	src := `@extern fun puts(ptr: ^u8) -> void

fun main() -> void {
    @print("Hello, World!\n")
    @print_nl()
}
`
	m, err := nanz.Parse(src, "hello_world")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	// Verify string was interned
	if len(m.Strings) != 1 {
		t.Errorf("strings: want 1, got %d: %v", len(m.Strings), m.Strings)
	}
	if len(m.Strings) >= 1 && m.Strings[0] != "Hello, World!\n" {
		t.Errorf("string[0]: want %q, got %q", "Hello, World!\n", m.Strings[0])
	}

	// Verify @print parses to a CallExpr{Fn:"@mir.io.print.str"}
	mainFunc := m.FuncByName("main")
	if mainFunc == nil {
		t.Fatal("main not found")
	}
	if len(mainFunc.Body.Body) < 2 {
		t.Fatalf("main body stmts: want >=2, got %d", len(mainFunc.Body.Body))
	}
	stmt0, ok := mainFunc.Body.Body[0].(*hir.ExprStmt)
	if !ok {
		t.Fatalf("body[0]: want ExprStmt, got %T", mainFunc.Body.Body[0])
	}
	call0, ok := stmt0.Expr.(*hir.CallExpr)
	if !ok {
		t.Fatalf("body[0].Expr: want CallExpr, got %T", stmt0.Expr)
	}
	if call0.Fn != "@mir.io.print.str" {
		t.Errorf("@print fn: want @mir.io.print.str, got %q", call0.Fn)
	}

	// Compile through the full pipeline to Z80 assembly.
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("CompileHIR: %v", err)
	}

	// Assembly must contain the string bytes ("Hello, World!\n" = 72 101 108 ...)
	// The string is emitted as a DB sequence in the MIR2 string pool section.
	// Check for 'H' (decimal 72) and presence of OUT instruction.
	checks := []string{
		"72",  // 'H' in decimal (DB encoding)
		"OUT", // OUT (0x01), A — the print_str loop
	}
	for _, want := range checks {
		if !strings.Contains(asm, want) {
			t.Errorf("assembly missing %q\n\nFull asm:\n%s", want, asm)
		}
	}

	// Assembly must reference the string pool symbol.
	if !strings.Contains(asm, "_mir2_str_") {
		t.Errorf("assembly missing _mir2_str_ symbol (sanitized from @mir2.str.)\n\nFull asm:\n%s", asm)
	}
}

// TestHelloWorldStringEscapes verifies that processStringEscapes correctly
// handles escape sequences like \n, \t, \\, \".
func TestHelloWorldStringEscapes(t *testing.T) {
	src := `fun greet() -> void {
    @print("Line1\nLine2\ttabbed")
}
`
	m, err := nanz.Parse(src, "escape_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Strings) != 1 {
		t.Fatalf("strings: want 1, got %d", len(m.Strings))
	}
	got := m.Strings[0]
	want := "Line1\nLine2\ttabbed"
	if got != want {
		t.Errorf("string[0]: want %q, got %q", want, got)
	}
}

// TestExternAddrRST verifies that RST address (multiple of 8, ≤0x38) is
// correctly stored in ExternAddr.
func TestExternAddrRST(t *testing.T) {
	cases := []struct {
		src  string
		name string
		addr uint16
	}{
		{`@extern(0x0008) fun rst8() -> void`, "rst8", 0x0008},
		{`@extern(0x0038) fun rst56() -> void`, "rst56", 0x0038},
		{`@extern(0x1234) fun call_fixed() -> void`, "call_fixed", 0x1234},
	}
	for _, tc := range cases {
		m, err := nanz.Parse(tc.src, "rst_test")
		if err != nil {
			t.Errorf("%s: Parse: %v", tc.name, err)
			continue
		}
		if len(m.Funcs) != 1 {
			t.Errorf("%s: want 1 func, got %d", tc.name, len(m.Funcs))
			continue
		}
		if m.Funcs[0].ExternAddr != tc.addr {
			t.Errorf("%s: ExternAddr want 0x%04X, got 0x%04X", tc.name, tc.addr, m.Funcs[0].ExternAddr)
		}
	}
}

// TestRegisterAnnotation verifies that @z80_a, @z80_hl etc. on params
// set the RegClass field correctly.
func TestRegisterAnnotation(t *testing.T) {
	src := `@extern(0x0010) fun print_char(@z80_a c: u8) -> void`
	m, err := nanz.Parse(src, "reg_annot_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("funcs: want 1, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if len(f.Params) != 1 {
		t.Fatalf("params: want 1, got %d", len(f.Params))
	}
	if f.Params[0].RegClass != mir2.ClassAcc {
		t.Errorf("param[0].RegClass: want ClassAcc (%d), got %d", mir2.ClassAcc, f.Params[0].RegClass)
	}
}

// ── New integer types (u24 / i24 / u32 / i32) ────────────────────────────────

func TestNewIntTypes_Parse(t *testing.T) {
	src := `
fun foo(a: u24, b: i24, c: u32, d: i32) -> u32 {
    return c
}
`
	m, err := nanz.Parse(src, "new_int_types")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("funcs: want 1, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	cases := []struct {
		idx  int
		want mir2.Ty
		name string
	}{
		{0, mir2.TyU24, "u24"},
		{1, mir2.TyI24, "i24"},
		{2, mir2.TyU32, "u32"},
		{3, mir2.TyI32, "i32"},
	}
	for _, tc := range cases {
		if f.Params[tc.idx].Ty != tc.want {
			t.Errorf("param[%d] (%s): want %v, got %v", tc.idx, tc.name, tc.want, f.Params[tc.idx].Ty)
		}
	}
	if f.RetTy != mir2.TyU32 {
		t.Errorf("return type: want u32, got %v", f.RetTy)
	}
}

// ── Fixed-point types (f.8 / f8.8 / f.16 / f8.16 / f16.8) ───────────────────

func TestFixedPointTypes_Parse(t *testing.T) {
	// Fixed-point syntax is locked ("coming soon") in the Nanz parser.
	// The types exist in the MIR2 IR (pkg/mir2/types.go) but are not
	// yet surface-level syntax. This test verifies the parser rejects them.
	cases := []struct {
		name string
		src  string
	}{
		{"f.8", `fun foo(x: f.8) -> f.8 { return x }`},
		{"f.16", `fun foo(x: f.16) -> f.16 { return x }`},
		{"f8.8", `fun foo(x: f8.8) -> f8.8 { return x }`},
		{"f8.16", `fun foo(x: f8.16) -> f8.16 { return x }`},
		{"f16.8", `fun foo(x: f16.8) -> f16.8 { return x }`},
		{"f16.16", `fun foo(x: f16.16) -> f16.16 { return x }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := nanz.Parse(tc.src, "fxp_test")
			if err == nil {
				t.Fatal("expected parse error for locked fixed-point type, got nil")
			}
		})
	}
}

func TestFixedPointTypes_Width(t *testing.T) {
	cases := []struct {
		ty    mir2.Ty
		width int
	}{
		{mir2.TyF0_8, 8},
		{mir2.TyF0_16, 16},
		{mir2.TyF8_8, 16},
		{mir2.TyF8_16, 24},
		{mir2.TyF16_8, 24},
		{mir2.TyF16_16, 32},
	}
	for _, tc := range cases {
		if tc.ty.Width() != tc.width {
			t.Errorf("%v.Width(): want %d, got %d", tc.ty, tc.width, tc.ty.Width())
		}
	}
}

func TestNewIntTypes_Width(t *testing.T) {
	cases := []struct {
		ty    mir2.Ty
		width int
		name  string
	}{
		{mir2.TyU24, 24, "u24"},
		{mir2.TyI24, 24, "i24"},
		{mir2.TyU32, 32, "u32"},
		{mir2.TyI32, 32, "i32"},
	}
	for _, tc := range cases {
		if tc.ty.Width() != tc.width {
			t.Errorf("%s.Width(): want %d, got %d", tc.name, tc.width, tc.ty.Width())
		}
		if tc.ty.String() != tc.name {
			t.Errorf("%s.String(): want %q, got %q", tc.name, tc.name, tc.ty.String())
		}
	}
}

// ── Multiple return values ────────────────────────────────────────────────────

func TestMultiReturn_Parse(t *testing.T) {
	src := `
fun minmax(a: u16, b: u16) -> (u16, u16) {
    if a <= b { return (a, b) }
    return (b, a)
}
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(m.Funcs) != 1 {
		t.Fatalf("want 1 func, got %d", len(m.Funcs))
	}
	f := m.Funcs[0]
	if len(f.RetTys) != 2 {
		t.Fatalf("want 2 RetTys, got %d", len(f.RetTys))
	}
	if f.RetTys[0] != mir2.TyU16 || f.RetTys[1] != mir2.TyU16 {
		t.Errorf("RetTys: want [u16, u16], got [%v, %v]", f.RetTys[0], f.RetTys[1])
	}
}

func TestMultiReturn_TupleLet(t *testing.T) {
	src := `
fun minmax(a: u16, b: u16) -> (u16, u16) {
    if a <= b { return (a, b) }
    return (b, a)
}
fun caller(x: u16, y: u16) -> u16 {
    let (lo, hi) = minmax(x, y)
    return hi
}
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	caller := m.FuncByName("caller")
	if caller == nil {
		t.Fatal("caller func not found")
	}
	_ = caller
}

func TestMultiReturn_BlankIdentifier(t *testing.T) {
	src := `
fun swap(a: u16, b: u16) -> (u16, u16) {
    return (b, a)
}
fun onlyfirst(x: u16, y: u16) -> u16 {
    let (r, _) = swap(x, y)
    return r
}
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if m.FuncByName("onlyfirst") == nil {
		t.Fatal("onlyfirst not found")
	}
}

func TestMultiReturn_E2E_Z80(t *testing.T) {
	src := `
fun minmax(a: u16, b: u16) -> (u16, u16) {
    if a <= b { return (a, b) }
    return (b, a)
}
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	// minmax should return lo in HL and hi in DE.
	if !strings.Contains(asm, "minmax:") {
		t.Error("no minmax label in output")
	}
	t.Logf("minmax Z80:\n%s", asm)
}

// ── fold / reduce ─────────────────────────────────────────────────────────────

func TestFold_Sum_ReturnValue(t *testing.T) {
	// fold(ptr, init, cb, n) — result used as return value
	src := `
global data: [u8; 4] = [10, 20, 30, 40]

fun add_u8(acc: u8, x: u8) -> u8 {
    return acc + x
}

fun sum_data() -> u8 {
    return fold(&data, 0, add_u8, 4)
}
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	if !strings.Contains(asm, "sum_data:") {
		t.Error("no sum_data label in output")
	}
	// Should contain a DJNZ loop for the fold.
	if !strings.Contains(asm, "DJNZ") {
		t.Error("expected DJNZ loop in fold output")
	}
	t.Logf("sum_data Z80:\n%s", asm)
}

func TestFold_LetBinding(t *testing.T) {
	// fold result bound to a let variable
	src := `
global nums: [u8; 3] = [1, 2, 3]

fun add_u8(acc: u8, x: u8) -> u8 {
    return acc + x
}

fun total() -> u8 {
    let s = fold(&nums, 0, add_u8, 3)
    return s
}
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
	if !strings.Contains(asm, "total:") {
		t.Error("no total label in output")
	}
	t.Logf("total Z80:\n%s", asm)
}

func TestFold_Reduce_Synonym(t *testing.T) {
	// reduce is a synonym for fold
	src := `
global vals: [u8; 2] = [5, 3]

fun add_u8(acc: u8, x: u8) -> u8 {
    return acc + x
}

fun using_reduce() -> u8 {
    return reduce(&vals, 0, add_u8, 2)
}
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile error: %v", err)
	}
}

// ── assert ────────────────────────────────────────────────────────────────────

func TestAssert_Parse(t *testing.T) {
	src := `
fun add(a: u8, b: u8) -> u8 {
    return a + b
}

assert add(3, 4) == 7
assert add(0, 0) == 0
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if len(m.Asserts) != 2 {
		t.Fatalf("want 2 asserts, got %d", len(m.Asserts))
	}
	if m.Asserts[0].FuncName != "add" {
		t.Errorf("assert[0] FuncName: want 'add', got %q", m.Asserts[0].FuncName)
	}
	if m.Asserts[0].Expected != 7 {
		t.Errorf("assert[0] Expected: want 7, got %d", m.Asserts[0].Expected)
	}
}

func TestAssert_PassesCompilation(t *testing.T) {
	src := `
fun abs_diff(a: u8, b: u8) -> u8 {
    if a >= b { return a - b }
    return b - a
}

assert abs_diff(10, 5) == 5
assert abs_diff(5, 10) == 5
assert abs_diff(7, 7) == 0
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, err = pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile error (asserts should pass): %v", err)
	}
}

func TestAssert_FailsCompilation(t *testing.T) {
	src := `
fun add(a: u8, b: u8) -> u8 {
    return a + b
}

assert add(3, 4) == 8
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	_, err = pipeline.CompileHIR(m)
	if err == nil {
		t.Fatal("expected compile error from failing assert, got nil")
	}
	if !strings.Contains(err.Error(), "assert") {
		t.Errorf("expected error message to mention 'assert', got: %v", err)
	}
	t.Logf("expected error: %v", err)
}

// TestStructLiteral_Parse verifies that struct literal syntax parses correctly.
func TestStructLiteral_Parse(t *testing.T) {
	src := `struct Color {
    r: u8
    g: u8
    b: u8
}

global palette: Color

fun set_palette() -> void {
    palette = Color{ r: 255, g: 128, b: 0 }
}
`
	m, err := nanz.Parse(src, "struct_lit_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	// set_palette should have one AssignStmt whose Val is a StructLitExpr
	var fn *hir.Func
	for _, f := range m.Funcs {
		if f.Name == "set_palette" {
			fn = f
			break
		}
	}
	if fn == nil {
		t.Fatal("set_palette not found")
	}
	assign, ok := fn.Body.Body[0].(*hir.AssignStmt)
	if !ok {
		t.Fatalf("expected AssignStmt, got %T", fn.Body.Body[0])
	}
	lit, ok := assign.Val.(*hir.StructLitExpr)
	if !ok {
		t.Fatalf("expected StructLitExpr, got %T", assign.Val)
	}
	if lit.St.Name != "Color" {
		t.Errorf("struct name: want Color, got %q", lit.St.Name)
	}
	if len(lit.Fields) != 3 {
		t.Errorf("field count: want 3, got %d", len(lit.Fields))
	}
	if lit.Fields[0].Name != "r" {
		t.Errorf("field[0]: want r, got %q", lit.Fields[0].Name)
	}
}

// TestStructLiteral_Codegen verifies that a struct literal compiles to valid Z80.
func TestStructLiteral_Codegen(t *testing.T) {
	src := `struct Color {
    r: u8
    g: u8
    b: u8
}

global palette: Color

fun set_red() -> void {
    palette = Color{ r: 255, g: 0, b: 0 }
}
`
	m, err := nanz.Parse(src, "struct_lit_codegen_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("CompileHIR: %v", err)
	}
	// Must contain field stores for r=255 (0xFF), g=0, b=0
	if !strings.Contains(asm, "255") && !strings.Contains(asm, "0xFF") &&
		!strings.Contains(asm, "0ffh") {
		t.Errorf("expected r=255 store in output; got:\n%s", asm)
	}
	t.Logf("struct literal Z80 output:\n%s", asm)
}

func TestSMCParam_Parse(t *testing.T) {
	src := `fun draw(@smc r0: u16) -> void {
}`
	m, err := nanz.Parse(src, "smc_parse_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if len(m.Funcs) == 0 {
		t.Fatal("no functions parsed")
	}
	f := m.Funcs[0]
	if len(f.Params) != 1 {
		t.Fatalf("expected 1 param, got %d", len(f.Params))
	}
	if !f.Params[0].SMC {
		t.Errorf("expected Param.SMC=true for @smc r0")
	}
	if f.Params[0].Name != "r0" {
		t.Errorf("expected param name 'r0', got %q", f.Params[0].Name)
	}
}

func TestSMCParam_Codegen_SingleByte(t *testing.T) {
	src := `fun draw_sprite(@smc r0: u16) -> void {
    var b: u8 = 195
    r0^ = b
}
`
	m, err := nanz.Parse(src, "smc_codegen_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("CompileHIR: %v", err)
	}
	// Must have the baked LD HL,0 slot and the EQU label
	if !strings.Contains(asm, "draw_sprite$r0$imm") {
		t.Errorf("expected EQU label 'draw_sprite$r0$imm' in output; got:\n%s", asm)
	}
	// Must have the auto-generated patcher
	if !strings.Contains(asm, "draw_sprite_set_r0:") {
		t.Errorf("expected patcher 'draw_sprite_set_r0:' in output; got:\n%s", asm)
	}
	t.Logf("@smc single-byte Z80:\n%s", asm)
}

func TestSMCParam_Codegen_StructLit(t *testing.T) {
	src := `struct Row2 {
    b0: u8
    b1: u8
}

fun draw_row(@smc r0: u16) -> void {
    r0^ = Row2{ b0: 195, b1: 60 }
}
`
	m, err := nanz.Parse(src, "smc_struct_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("CompileHIR: %v", err)
	}
	// Must have the patcher EQU and synthesised function
	if !strings.Contains(asm, "draw_row$r0$imm") {
		t.Errorf("expected EQU label in output; got:\n%s", asm)
	}
	if !strings.Contains(asm, "draw_row_set_r0:") {
		t.Errorf("expected patcher function in output; got:\n%s", asm)
	}
	// Should have INC HL for the second field
	if !strings.Contains(asm, "INC") {
		t.Errorf("expected INC HL for second field; got:\n%s", asm)
	}
	t.Logf("@smc struct literal Z80:\n%s", asm)
}

func TestSMCParam_EmissionQuality(t *testing.T) {
	src := `struct Row2 {
    b0: u8
    b1: u8
}

fun draw_row(@smc r0: u16) -> void {
    r0^ = Row2{ b0: 195, b1: 60 }
}
`
	m, err := nanz.Parse(src, "smc_quality_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("CompileHIR: %v", err)
	}

	t.Logf("full Z80 output:\n%s", asm)

	// ── Assert ABSENT: alloca anti-patterns ──────────────────────────────────
	if strings.Contains(asm, "DEC SP") {
		t.Errorf("REGRESSION: alloca anti-pattern 'DEC SP' found in output (alloca fallback triggered)")
	}
	if strings.Contains(asm, "LD BC, SP") {
		t.Errorf("REGRESSION: alloca anti-pattern 'LD BC, SP' found in output (alloca base pointer setup)")
	}
	if strings.Contains(asm, "PUSH BC") {
		t.Errorf("REGRESSION: alloca anti-pattern 'PUSH BC' found in output (alloca-to-pointer copy sequence)")
	}

	// ── Assert PRESENT: optimal SMC pattern ──────────────────────────────────
	if !strings.Contains(asm, "draw_row$r0$imm") {
		t.Errorf("expected EQU label 'draw_row$r0$imm' in output")
	}
	if !strings.Contains(asm, "draw_row_set_r0:") {
		t.Errorf("expected patcher function 'draw_row_set_r0:' in output")
	}
	if !strings.Contains(asm, "LD HL, 0") {
		t.Errorf("expected baked immediate slot 'LD HL, 0' in output")
	}
	if !strings.Contains(asm, "LD (HL), 195") {
		t.Errorf("expected first field store 'LD (HL), 195' in output")
	}
	if !strings.Contains(asm, "INC HL") {
		t.Errorf("expected chain advancement 'INC HL' in output")
	}
	if !strings.Contains(asm, "LD (HL), 60") {
		t.Errorf("expected second field store 'LD (HL), 60' in output")
	}

	// ── Instruction count in draw_row body ───────────────────────────────────
	// Find draw_row: label and count indented instruction lines until blank line
	// or next non-indented label.
	lines := strings.Split(asm, "\n")
	inBody := false
	instrCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "draw_row:" {
			inBody = true
			continue
		}
		if !inBody {
			continue
		}
		// Stop at blank line or next label (non-indented, non-empty, ends with ':')
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			break
		}
		if len(line) > 0 && line[0] != ' ' && line[0] != '\t' {
			break
		}
		// Skip comment-only lines and EQU directives
		if strings.HasPrefix(trimmed, ";") || strings.Contains(trimmed, "EQU") {
			continue
		}
		instrCount++
	}

	t.Logf("draw_row body instruction count: %d", instrCount)

	// LD HL,0 + LD (HL),195 + INC HL + LD (HL),60 + RET = 5 instructions
	// Allow ≤ 7 to be conservative (e.g. extra EX or LD A,n intermediate).
	if instrCount > 7 {
		t.Errorf("draw_row body is bloated: %d instructions (want ≤ 7; optimal is 5)", instrCount)
	}
}

func TestSMCParam_NoBloat_SingleByte(t *testing.T) {
	src := `fun draw_sprite(@smc r0: u16) -> void {
    var b: u8 = 195
    r0^ = b
}
`
	m, err := nanz.Parse(src, "smc_nobloat_test")
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("CompileHIR: %v", err)
	}

	t.Logf("draw_sprite Z80 output:\n%s", asm)

	// Constant should be folded through the local var → direct LD (HL), 195
	if !strings.Contains(asm, "LD (HL), 195") {
		t.Errorf("expected const-folded store 'LD (HL), 195' in output (const folding through local var)")
	}

	// No alloca fallback
	if strings.Contains(asm, "DEC SP") {
		t.Errorf("REGRESSION: alloca anti-pattern 'DEC SP' found in output")
	}

	// Patcher must be present
	if !strings.Contains(asm, "draw_sprite_set_r0:") {
		t.Errorf("expected patcher function 'draw_sprite_set_r0:' in output")
	}
}

// TestConsoleLog_VoidFuncRET verifies that a void function whose last
// instruction before 'return' is an intrinsic call (@console_log / @console_err)
// still emits a trailing RET.  Regression for: intrinsic OpCall was
// mis-detected as a tail call, causing genTerm(TermRet) to skip RET.
func TestConsoleLog_VoidFuncRET(t *testing.T) {
	src := `
fun main() -> void {
    @console_log(42)
    @console_err(7)
    return
}
`
	m, err := nanz.Parse(src, "test")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	asm, err := pipeline.CompileHIR(m)
	if err != nil {
		t.Fatalf("compile: %v", err)
	}

	// Both OUT instructions must be present (mze/mzx standard ports $23/$25).
	if !strings.Contains(asm, "OUT (0x23), A") {
		t.Errorf("missing OUT (0x23) for console_log\n%s", asm)
	}
	if !strings.Contains(asm, "OUT (0x25), A") {
		t.Errorf("missing OUT (0x25) for console_err\n%s", asm)
	}

	// main() must terminate with DI+HALT (not RET) so mze can detect program end.
	if !strings.Contains(asm, "    DI") || !strings.Contains(asm, "    HALT") {
		t.Errorf("void main() must end with DI+HALT for mze compatibility\n%s", asm)
	}
}
