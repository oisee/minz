package parser

import (
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/ast"
)

func TestPreprocessAsmFunctions(t *testing.T) {
	tests := []struct {
		name   string
		input  string
		expect string // substring that should appear in output
	}{
		{
			name:   "simple asm fun",
			input:  `pub asm fun foo() -> u8 { LD A, 1 ; load 1\n RET }`,
			expect: `asm {`,
		},
		{
			name:   "asm fun with semicolon comments",
			input:  "pub asm fun get_sysvars() -> *u8 {\n    RST 0x08\n    PUSH IY\n    POP HL\n    RET\n}",
			expect: "asm {",
		},
		{
			name:   "regular fun unchanged",
			input:  `fun main() { let x = 1; }`,
			expect: `fun main() { let x = 1; }`,
		},
		{
			name:   "asm block in regular fun unchanged",
			input:  `fun foo() { asm { LD A, 1 } }`,
			expect: `fun foo() { asm { LD A, 1 } }`,
		},
		{
			name:   "multiple asm funs",
			input:  "asm fun a() -> u8 { RET }\nasm fun b() -> u8 { RET }",
			expect: "asm {",
		},
		{
			name:   "no false positive on assembly word",
			input:  `fun assembly() { let x = 1; }`,
			expect: `fun assembly() { let x = 1; }`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := preprocessAsmFunctions(tt.input)
			if !strings.Contains(result, tt.expect) {
				t.Errorf("preprocessAsmFunctions(%q)\n  got:  %q\n  want substring: %q", tt.input, result, tt.expect)
			}
		})
	}
}

func TestPreprocessPreservesRegularCode(t *testing.T) {
	input := `
import stdlib.agon.mos;

const FOO: u8 = 42;

fun main() {
    let x = 1;
    asm {
        LD A, 1
        RET
    }
}
`
	result := preprocessAsmFunctions(input)
	if result != input {
		t.Errorf("preprocessAsmFunctions should not modify code without asm fun")
	}
}

func TestAsmFunParsing(t *testing.T) {
	// Test that asm fun with assembly comments actually parses
	code := `
pub asm fun get_sysvars() -> *u8 {
    RST 0x08
    PUSH IY
    POP HL
    RET
}
`
	p := New()
	decls, err := p.ParseString(code, "test_asm_fun")
	if err != nil {
		t.Fatalf("Failed to parse asm fun: %v", err)
	}

	if len(decls) != 1 {
		t.Fatalf("Expected 1 declaration, got %d", len(decls))
	}

	fn, ok := decls[0].(*ast.FunctionDecl)
	if !ok {
		t.Fatalf("Expected FunctionDecl, got %T", decls[0])
	}

	if fn.Name != "get_sysvars" {
		t.Errorf("Expected function name 'get_sysvars', got '%s'", fn.Name)
	}

	if fn.FunctionKind != ast.FunctionKindAsm {
		t.Errorf("Expected FunctionKindAsm, got %d", fn.FunctionKind)
	}

	if !fn.IsPublic {
		t.Error("Expected function to be public")
	}

	// Body should have at least one AsmStmt
	if fn.Body == nil {
		t.Fatal("Expected function body, got nil")
	}
	if len(fn.Body.Statements) == 0 {
		t.Fatal("Expected at least one statement in body")
	}

	asmStmt, ok := fn.Body.Statements[0].(*ast.AsmStmt)
	if !ok {
		t.Fatalf("Expected AsmStmt, got %T", fn.Body.Statements[0])
	}

	// The asm code should contain the assembly instructions
	if !strings.Contains(asmStmt.Code, "RST") {
		t.Errorf("Expected asm code to contain 'RST', got: %s", asmStmt.Code)
	}
}

func TestAsmFunWithSemicolonComments(t *testing.T) {
	code := `
pub asm fun fopen(filename: *u8, mode: u8) -> u8 {
    ; filename in HL, mode in A (after HL)
    ; Need: HL = filename, C = mode, A = function number
    LD C, A
    LD A, 0x0A
    RST 0x00
    RET
}
`
	p := New()
	decls, err := p.ParseString(code, "test_asm_semicolons")
	if err != nil {
		t.Fatalf("Failed to parse asm fun with semicolon comments: %v", err)
	}

	if len(decls) != 1 {
		t.Fatalf("Expected 1 declaration, got %d", len(decls))
	}

	fn, ok := decls[0].(*ast.FunctionDecl)
	if !ok {
		t.Fatalf("Expected FunctionDecl, got %T", decls[0])
	}

	if fn.FunctionKind != ast.FunctionKindAsm {
		t.Errorf("Expected FunctionKindAsm, got %d", fn.FunctionKind)
	}

	if fn.Body == nil || len(fn.Body.Statements) == 0 {
		t.Fatal("Expected function body with statements")
	}

	asmStmt, ok := fn.Body.Statements[0].(*ast.AsmStmt)
	if !ok {
		t.Fatalf("Expected AsmStmt, got %T", fn.Body.Statements[0])
	}

	// Verify the asm code contains the assembly + comments
	if !strings.Contains(asmStmt.Code, "LD C, A") {
		t.Errorf("Expected asm code to contain 'LD C, A', got: %s", asmStmt.Code)
	}
}

func TestMultipleAsmFuns(t *testing.T) {
	code := `
pub asm fun foo() -> u8 {
    LD A, 1
    RET
}

pub asm fun bar() -> u8 {
    LD A, 2
    RET
}

pub fun baz() -> u8 {
    return 3;
}
`
	p := New()
	decls, err := p.ParseString(code, "test_multi_asm")
	if err != nil {
		t.Fatalf("Failed to parse multiple asm funs: %v", err)
	}

	if len(decls) != 3 {
		t.Fatalf("Expected 3 declarations, got %d", len(decls))
	}

	// First two should be asm functions
	for i := 0; i < 2; i++ {
		fn, ok := decls[i].(*ast.FunctionDecl)
		if !ok {
			t.Fatalf("Decl %d: expected FunctionDecl, got %T", i, decls[i])
		}
		if fn.FunctionKind != ast.FunctionKindAsm {
			t.Errorf("Decl %d: expected FunctionKindAsm", i)
		}
	}

	// Third should be regular function
	fn, ok := decls[2].(*ast.FunctionDecl)
	if !ok {
		t.Fatalf("Decl 2: expected FunctionDecl, got %T", decls[2])
	}
	if fn.FunctionKind != ast.FunctionKindRegular {
		t.Errorf("Decl 2: expected FunctionKindRegular, got %d", fn.FunctionKind)
	}
}
