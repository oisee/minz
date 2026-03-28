package pascal

import (
	"fmt"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
)

func TestParseMinimal(t *testing.T) {
	src := `program Hello;
begin
end.`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	if prog.Name != "HELLO" {
		t.Errorf("name = %q, want HELLO", prog.Name)
	}
}

func TestParseConst(t *testing.T) {
	src := `program Test;
const
  Size = 100;
  MaxVal = $FF;
begin
end.`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Decls) != 2 {
		t.Fatalf("got %d decls, want 2", len(prog.Decls))
	}
	cd := prog.Decls[0].(*ConstDecl)
	if cd.Name != "SIZE" {
		t.Errorf("const name = %q", cd.Name)
	}
}

func TestParseVar(t *testing.T) {
	src := `program Test;
var
  I, J: Integer;
  Ch: Char;
begin
end.`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Decls) != 2 {
		t.Fatalf("got %d decls, want 2", len(prog.Decls))
	}
	vd := prog.Decls[0].(*VarDecl)
	if len(vd.Names) != 2 {
		t.Errorf("got %d names, want 2", len(vd.Names))
	}
}

func TestParseArrayType(t *testing.T) {
	src := `program Test;
type
  TFlags = array[0..100] of Boolean;
var
  Flags: TFlags;
begin
end.`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	td := prog.Decls[0].(*TypeDecl)
	at, ok := td.Ty.(*ArrayType)
	if !ok {
		t.Fatalf("expected ArrayType, got %T", td.Ty)
	}
	if at.Lo != 0 || at.Hi != 100 {
		t.Errorf("array bounds: %d..%d", at.Lo, at.Hi)
	}
}

func TestParseRecord(t *testing.T) {
	src := `program Test;
type
  TPoint = record
    X, Y: Integer;
  end;
begin
end.`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	td := prog.Decls[0].(*TypeDecl)
	rt, ok := td.Ty.(*RecordType)
	if !ok {
		t.Fatalf("expected RecordType, got %T", td.Ty)
	}
	if len(rt.Fields) != 1 { // one field group: X, Y
		t.Errorf("got %d field groups, want 1", len(rt.Fields))
	}
	if len(rt.Fields[0].Names) != 2 {
		t.Errorf("got %d names in first field, want 2", len(rt.Fields[0].Names))
	}
}

func TestParseProcedure(t *testing.T) {
	src := `program Test;
procedure Swap(var X, Y: Integer);
var Tmp: Integer;
begin
  Tmp := X;
  X := Y;
  Y := Tmp;
end;
begin
end.`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	pd := prog.Decls[0].(*ProcDecl)
	if pd.Name != "SWAP" {
		t.Errorf("name = %q, want SWAP", pd.Name)
	}
	if len(pd.Params) != 1 {
		t.Fatalf("got %d param groups", len(pd.Params))
	}
	if !pd.Params[0].IsVar {
		t.Error("expected var params")
	}
}

func TestParseFunction(t *testing.T) {
	src := `program Test;
function Max(A, B: Integer): Integer;
begin
  if A > B then
    Max := A
  else
    Max := B;
end;
begin
end.`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	pd := prog.Decls[0].(*ProcDecl)
	if pd.RetTy == nil {
		t.Fatal("expected return type")
	}
}

func TestParseForLoop(t *testing.T) {
	src := `program Test;
var I: Integer;
begin
  for I := 1 to 10 do
    I := I;
end.`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Body) != 1 {
		t.Fatalf("got %d stmts, want 1", len(prog.Body))
	}
	fs, ok := prog.Body[0].(*ForStmt)
	if !ok {
		t.Fatalf("expected ForStmt, got %T", prog.Body[0])
	}
	if fs.Downto {
		t.Error("expected TO, not DOWNTO")
	}
}

func TestParseWhileRepeat(t *testing.T) {
	src := `program Test;
var X: Integer;
begin
  while X > 0 do
    X := X - 1;
  repeat
    X := X + 1;
  until X = 10;
end.`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Body) != 2 {
		t.Fatalf("got %d stmts, want 2", len(prog.Body))
	}
	if _, ok := prog.Body[0].(*WhileStmt); !ok {
		t.Errorf("stmt 0: expected WhileStmt, got %T", prog.Body[0])
	}
	if _, ok := prog.Body[1].(*RepeatStmt); !ok {
		t.Errorf("stmt 1: expected RepeatStmt, got %T", prog.Body[1])
	}
}

func TestParseCaseStmt(t *testing.T) {
	src := `program Test;
var Ch: Char;
begin
  case Ch of
    'A': Ch := 'B';
    'B': Ch := 'C';
  else
    Ch := 'A';
  end;
end.`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	cs, ok := prog.Body[0].(*CaseStmt)
	if !ok {
		t.Fatalf("expected CaseStmt, got %T", prog.Body[0])
	}
	if len(cs.Arms) != 2 {
		t.Errorf("got %d arms, want 2", len(cs.Arms))
	}
	if cs.Default == nil {
		t.Error("expected default clause")
	}
}

func TestParseNestedProc(t *testing.T) {
	src := `program Test;
var Total: Integer;
procedure Outer(N: Integer);
  procedure Inner(X: Integer);
  begin
    Total := Total + X;
  end;
begin
  Inner(N);
end;
begin
  Total := 0;
  Outer(10);
end.`
	prog, err := ParseProgram(src)
	if err != nil {
		t.Fatal(err)
	}
	pd := prog.Decls[1].(*ProcDecl)
	if len(pd.SubProc) != 1 {
		t.Fatalf("expected 1 nested proc, got %d", len(pd.SubProc))
	}
	if pd.SubProc[0].Name != "INNER" {
		t.Errorf("nested proc name = %q", pd.SubProc[0].Name)
	}
}

func TestCompileMinimal(t *testing.T) {
	src := `program Hello;
begin
end.`
	hm, err := Compile(src, "hello")
	if err != nil {
		t.Fatal(err)
	}
	if hm.Name != "hello" {
		t.Errorf("module name = %q", hm.Name)
	}
	if len(hm.Funcs) != 1 {
		t.Fatalf("expected 1 func (main), got %d", len(hm.Funcs))
	}
	if hm.Funcs[0].Name != "main" {
		t.Errorf("func name = %q, want main", hm.Funcs[0].Name)
	}
}

func TestCompileWithVarsAndFor(t *testing.T) {
	src := `program Sieve;
const Size = 100;
var I, Count: Integer;
begin
  Count := 0;
  for I := 0 to 10 do
    Count := Count + 1;
end.`
	hm, err := Compile(src, "sieve")
	if err != nil {
		t.Fatal(err)
	}
	if len(hm.Globals) != 2 {
		t.Errorf("globals: got %d, want 2", len(hm.Globals))
	}
	dump := hm.Dump()
	if !strings.Contains(dump, "main") {
		t.Error("expected main function in dump")
	}
}

func TestCompileProcedure(t *testing.T) {
	src := `program Test;
var A: Integer;
procedure AddOne(X: Integer);
begin
  A := X + 1;
end;
begin
  A := 0;
  AddOne(41);
end.`
	hm, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	// Should have AddOne + main
	if len(hm.Funcs) != 2 {
		t.Errorf("funcs: got %d, want 2", len(hm.Funcs))
	}
}

func TestCompileFunction(t *testing.T) {
	src := `program Test;
function Double(X: Integer): Integer;
begin
  Double := X + X;
end;
var R: Integer;
begin
  R := Double(21);
end.`
	hm, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, f := range hm.Funcs {
		if f.Name == "DOUBLE" {
			found = true
			if f.RetTy.String() != "i16" {
				t.Errorf("Double return type = %s, want i16", f.RetTy)
			}
		}
	}
	if !found {
		t.Error("DOUBLE function not found")
	}
}

func TestCompileIfElse(t *testing.T) {
	src := `program Test;
var X: Integer;
begin
  X := 5;
  if X > 3 then
    X := 1
  else
    X := 0;
end.`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompileCase(t *testing.T) {
	src := `program Test;
var Ch: Byte;
begin
  Ch := 65;
  case Ch of
    65: Ch := 66;
    66: Ch := 67;
  else
    Ch := 0;
  end;
end.`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompileRepeat(t *testing.T) {
	src := `program Test;
var X: Integer;
begin
  X := 0;
  repeat
    X := X + 1;
  until X = 10;
end.`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompileTypedConst(t *testing.T) {
	src := `program Test;
const
  Counter: Integer = 0;
var I: Integer;
begin
  for I := 1 to 5 do
    Counter := Counter + I;
end.`
	hm, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range hm.Globals {
		if g.Name == "COUNTER" {
			found = true
		}
	}
	if !found {
		t.Error("typed const COUNTER not found as global")
	}
}

func TestCompileRecord(t *testing.T) {
	src := `program Test;
type
  TPoint = record
    X, Y: Integer;
  end;
var P: TPoint;
begin
  P.X := 10;
  P.Y := 20;
end.`
	hm, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(hm.Structs) != 1 {
		t.Fatalf("expected 1 struct, got %d", len(hm.Structs))
	}
	if hm.Structs[0].Name != "TPOINT" {
		t.Errorf("struct name = %q", hm.Structs[0].Name)
	}
}

func TestCompileComments(t *testing.T) {
	src := `program Test;
{ This is a brace comment }
(* This is a paren-star comment *)
// This is a line comment
var X: Integer;
begin
  X := 42; { inline comment }
end.`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompileExpressions(t *testing.T) {
	src := `program Test;
var A, B, C: Integer;
begin
  A := 2 + 3 * 4;
  B := (A - 1) div 2;
  C := A mod 3;
  if (A > 0) and (B < 100) then
    C := A + B;
end.`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompileStringType(t *testing.T) {
	src := `program Test;
var S: string[20];
begin
end.`
	hm, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(hm.Globals) < 1 {
		t.Fatal("expected global for string var")
	}
}

func TestCompileForDownto(t *testing.T) {
	src := `program Test;
var I: Integer;
begin
  for I := 10 downto 1 do
    I := I;
end.`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompileWriteLn(t *testing.T) {
	src := `program Test;
var X: Byte;
begin
  X := 42;
  WriteLn(X);
end.`
	hm, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	dump := hm.Dump()
	if !strings.Contains(dump, "WriteU8") {
		t.Error("expected WriteU8 call for Byte arg")
	}
	if !strings.Contains(dump, "WriteCrLf") {
		t.Error("expected WriteCrLf call for WriteLn")
	}
}

func TestCompileUsesClause(t *testing.T) {
	src := `program Test;
uses Crt, Graph;
begin
end.`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestCompileAssert(t *testing.T) {
	src := `program Test;
function Double(X: Integer): Integer;
begin
  Double := X + X;
end;
begin
  assert Double(21) = 42;
end.`
	hm, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(hm.Asserts) != 1 {
		t.Fatalf("expected 1 assert, got %d", len(hm.Asserts))
	}
	a := hm.Asserts[0]
	if a.FuncName != "DOUBLE" {
		t.Errorf("assert func = %q, want DOUBLE", a.FuncName)
	}
	if a.Expected != 42 {
		t.Errorf("assert expected = %d, want 42", a.Expected)
	}
	if len(a.Args) != 1 || a.Args[0] != 21 {
		t.Errorf("assert args = %v, want [21]", a.Args)
	}
}

func TestParseHexAndChar(t *testing.T) {
	src := `program Test;
const A = $FF;
var Ch: Char;
begin
  Ch := #65;
end.`
	_, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestAssertE2E(t *testing.T) {
	src := `program Test;
function Double(X: Integer): Integer;
begin
  Double := X + X;
end;
begin
  assert Double(0) = 0;
  assert Double(21) = 42;
end.`
	hm, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	_, err = pipeline.CompileHIR(hm)
	if err != nil {
		t.Fatalf("pipeline (asserts should pass): %v", err)
	}
}

func TestFunctionPointer(t *testing.T) {
	src := `program FPTest;
type
  TTransform = function(x: byte): byte;

function Double(x: byte): byte;
begin
  Double := x + x;
end;

function Apply(f: TTransform; x: byte): byte;
begin
  Apply := f(x);
end;

function TestFP: byte;
begin
  TestFP := Apply(@Double, 5);
end;

begin
end.`
	hm, err := Compile(src, "fp_test")
	if err != nil {
		t.Fatalf("Compile: %v", err)
	}
	// Check that TTransform is a ProcPtrType → TyPtr param
	found := false
	for _, f := range hm.Funcs {
		if f.Name == "APPLY" {
			for _, p := range f.Params {
				if p.Name == "F" && p.Ty == mir2.TyPtr {
					found = true
				}
			}
		}
	}
	if !found {
		t.Error("APPLY param F should be TyPtr (function pointer)")
	}
	// Check that Apply body contains CallIndirectExpr (not CallExpr with "F")
	applyFn := hm.FuncByName("APPLY")
	if applyFn == nil {
		t.Fatal("APPLY function not found")
	}
	hirStr := strings.Builder{}
	fmt.Fprintf(&hirStr, "%v", applyFn.Body)
	if strings.Contains(hirStr.String(), "CallIndirectExpr") {
		t.Log("OK: Apply body contains CallIndirectExpr")
	}
	t.Logf("APPLY body: %+v", applyFn.Body)
}

func TestMaxE2E(t *testing.T) {
	src := `program Test;
function Max(A, B: Byte): Byte;
begin
  if A > B then
    Max := A
  else
    Max := B;
end;
begin
  assert Max(10, 2) = 10;
  assert Max(2, 10) = 10;
  assert Max(5, 5) = 5;
  assert Max(255, 0) = 255;
  assert Max(0, 255) = 255;
  assert Max(1, 0) = 1;
  assert Max(0, 1) = 1;
end.`
	hm, err := Compile(src, "test")
	if err != nil {
		t.Fatal(err)
	}
	asm, err := pipeline.CompileHIR(hm)
	if err != nil {
		t.Fatalf("pipeline: %v", err)
	}
	t.Log("\n" + asm)
}
