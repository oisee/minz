package frill

import "testing"

func TestLex(t *testing.T) {
	src := `let double (x : u8) : u8 = x + x`
	tokens, err := Lex(src, "test.frill")
	if err != nil {
		t.Fatal(err)
	}
	// let, double, (, x, :, u8, ), :, u8, =, x, +, x, EOF
	if len(tokens) < 10 {
		t.Errorf("expected >= 10 tokens, got %d", len(tokens))
	}
	if tokens[0].Kind != TokLet {
		t.Errorf("first token should be 'let', got %q", tokens[0].Text)
	}
	t.Logf("%d tokens", len(tokens))
}

func TestParse(t *testing.T) {
	src := `let double (x : u8) : u8 = x + x`
	tokens, _ := Lex(src, "test.frill")
	prog, err := Parse(tokens, "test.frill")
	if err != nil {
		t.Fatal(err)
	}
	if len(prog.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(prog.Decls))
	}
	decl := prog.Decls[0].(*LetDecl)
	if decl.Name != "double" {
		t.Errorf("name = %q, want 'double'", decl.Name)
	}
	if len(decl.Params) != 1 {
		t.Errorf("params = %d, want 1", len(decl.Params))
	}
}

func TestCompile(t *testing.T) {
	src := `
let double (x : u8) : u8 = x + x
let main () : u8 = double(21)
`
	mod, err := Compile(src, "test.frill")
	if err != nil {
		t.Fatal(err)
	}
	if len(mod.Funcs) != 2 {
		t.Fatalf("expected 2 funcs, got %d", len(mod.Funcs))
	}
	t.Logf("Functions: %s, %s", mod.Funcs[0].Name, mod.Funcs[1].Name)
}

func TestPipe(t *testing.T) {
	src := `
let double (x : u8) : u8 = x + x
let inc (x : u8) : u8 = x + 1
let main () : u8 = 5 |> double |> inc
`
	mod, err := Compile(src, "pipe.frill")
	if err != nil {
		t.Fatal(err)
	}
	if len(mod.Funcs) != 3 {
		t.Fatalf("expected 3 funcs, got %d", len(mod.Funcs))
	}
	// main should call inc(double(5))
	t.Logf("Compiled %d functions from Frill with pipe operator", len(mod.Funcs))
}
