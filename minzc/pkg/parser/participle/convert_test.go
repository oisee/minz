package participle

import (
	"testing"

	"github.com/minz/minzc/pkg/ast"
)

func TestConvertBasicFile(t *testing.T) {
	source := `
import std.io;

const MAX = 100;

struct Point {
    x: u8,
    y: u8,
}

fun add(a: u8, b: u8) -> u8 {
    return a + b;
}

fun main() {
    let x = 5;
    let y = add(x, 10);
}
`
	parser := New()
	pFile, err := parser.ParseString("test.minz", source)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}

	converter := NewConverter("test.minz")
	astFile, err := converter.Convert(pFile)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	// Check imports
	if len(astFile.Imports) != 1 {
		t.Errorf("expected 1 import, got %d", len(astFile.Imports))
	} else if astFile.Imports[0].Path != "std.io" {
		t.Errorf("expected import 'std.io', got '%s'", astFile.Imports[0].Path)
	}

	// Check declarations count
	if len(astFile.Declarations) != 4 {
		t.Errorf("expected 4 declarations, got %d", len(astFile.Declarations))
	}

	// Check const
	if len(astFile.Declarations) > 0 {
		if cd, ok := astFile.Declarations[0].(*ast.ConstDecl); ok {
			if cd.Name != "MAX" {
				t.Errorf("expected const 'MAX', got '%s'", cd.Name)
			}
		} else {
			t.Errorf("expected ConstDecl, got %T", astFile.Declarations[0])
		}
	}

	// Check struct
	if len(astFile.Declarations) > 1 {
		if sd, ok := astFile.Declarations[1].(*ast.StructDecl); ok {
			if sd.Name != "Point" {
				t.Errorf("expected struct 'Point', got '%s'", sd.Name)
			}
			if len(sd.Fields) != 2 {
				t.Errorf("expected 2 fields, got %d", len(sd.Fields))
			}
		} else {
			t.Errorf("expected StructDecl, got %T", astFile.Declarations[1])
		}
	}

	// Check functions
	if len(astFile.Declarations) > 2 {
		if fd, ok := astFile.Declarations[2].(*ast.FunctionDecl); ok {
			if fd.Name != "add" {
				t.Errorf("expected function 'add', got '%s'", fd.Name)
			}
			if len(fd.Params) != 2 {
				t.Errorf("expected 2 params, got %d", len(fd.Params))
			}
		} else {
			t.Errorf("expected FunctionDecl, got %T", astFile.Declarations[2])
		}
	}
}

func TestConvertExpressions(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		checkFn  func(*ast.File) bool
	}{
		{
			name:  "binary_expr",
			input: "fun f() { let x = 1 + 2 * 3; }",
			checkFn: func(f *ast.File) bool {
				fd := f.Declarations[0].(*ast.FunctionDecl)
				vd := fd.Body.Statements[0].(*ast.VarDecl)
				_, ok := vd.Value.(*ast.BinaryExpr)
				return ok
			},
		},
		{
			name:  "function_call",
			input: "fun f() { let x = foo(1, 2); }",
			checkFn: func(f *ast.File) bool {
				fd := f.Declarations[0].(*ast.FunctionDecl)
				vd := fd.Body.Statements[0].(*ast.VarDecl)
				call, ok := vd.Value.(*ast.CallExpr)
				return ok && len(call.Arguments) == 2
			},
		},
		{
			name:  "field_access",
			input: "fun f() { let x = obj.field; }",
			checkFn: func(f *ast.File) bool {
				fd := f.Declarations[0].(*ast.FunctionDecl)
				vd := fd.Body.Statements[0].(*ast.VarDecl)
				fe, ok := vd.Value.(*ast.FieldExpr)
				return ok && fe.Field == "field"
			},
		},
		{
			name:  "array_index",
			input: "fun f() { let x = arr[0]; }",
			checkFn: func(f *ast.File) bool {
				fd := f.Declarations[0].(*ast.FunctionDecl)
				vd := fd.Body.Statements[0].(*ast.VarDecl)
				_, ok := vd.Value.(*ast.IndexExpr)
				return ok
			},
		},
		{
			name:  "struct_literal",
			input: "fun f() { let p = Point { x: 1, y: 2 }; }",
			checkFn: func(f *ast.File) bool {
				fd := f.Declarations[0].(*ast.FunctionDecl)
				vd := fd.Body.Statements[0].(*ast.VarDecl)
				sl, ok := vd.Value.(*ast.StructLiteral)
				return ok && sl.TypeName == "Point" && len(sl.Fields) == 2
			},
		},
	}

	parser := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pFile, err := parser.ParseString("test.minz", tt.input)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			converter := NewConverter("test.minz")
			astFile, err := converter.Convert(pFile)
			if err != nil {
				t.Fatalf("convert failed: %v", err)
			}

			if !tt.checkFn(astFile) {
				t.Error("check function failed")
			}
		})
	}
}

func TestConvertStatements(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		checkFn  func(*ast.File) bool
	}{
		{
			name:  "if_stmt",
			input: "fun f() { if x { } }",
			checkFn: func(f *ast.File) bool {
				fd := f.Declarations[0].(*ast.FunctionDecl)
				_, ok := fd.Body.Statements[0].(*ast.IfStmt)
				return ok
			},
		},
		{
			name:  "while_stmt",
			input: "fun f() { while x { } }",
			checkFn: func(f *ast.File) bool {
				fd := f.Declarations[0].(*ast.FunctionDecl)
				_, ok := fd.Body.Statements[0].(*ast.WhileStmt)
				return ok
			},
		},
		{
			name:  "for_stmt",
			input: "fun f() { for i in 0..10 { } }",
			checkFn: func(f *ast.File) bool {
				fd := f.Declarations[0].(*ast.FunctionDecl)
				fs, ok := fd.Body.Statements[0].(*ast.ForStmt)
				return ok && fs.Iterator == "i"
			},
		},
		{
			name:  "return_stmt",
			input: "fun f() -> u8 { return 42; }",
			checkFn: func(f *ast.File) bool {
				fd := f.Declarations[0].(*ast.FunctionDecl)
				rs, ok := fd.Body.Statements[0].(*ast.ReturnStmt)
				return ok && rs.Value != nil
			},
		},
		{
			name:  "assign_stmt",
			input: "fun f() { x = 5; }",
			checkFn: func(f *ast.File) bool {
				fd := f.Declarations[0].(*ast.FunctionDecl)
				_, ok := fd.Body.Statements[0].(*ast.AssignStmt)
				return ok
			},
		},
	}

	parser := New()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pFile, err := parser.ParseString("test.minz", tt.input)
			if err != nil {
				t.Fatalf("parse failed: %v", err)
			}

			converter := NewConverter("test.minz")
			astFile, err := converter.Convert(pFile)
			if err != nil {
				t.Fatalf("convert failed: %v", err)
			}

			if !tt.checkFn(astFile) {
				t.Error("check function failed")
			}
		})
	}
}
