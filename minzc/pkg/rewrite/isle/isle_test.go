package isle

import "testing"

func TestParse_BasicRules(t *testing.T) {
	src := `
;; Types
(type Reg)
(type Inst)
(type u8)
(type u16)

;; Declarations
(decl lower (MIROp) Inst)
(decl put_in_a (Reg) Reg)
(decl gpr8 (Reg) Reg)

;; Rules
(rule (lower (add u8 ?x ?y))
      (z80_add_a (put_in_a ?x) (gpr8 ?y)))

(rule 10 (lower (add u8 ?x (const 1)))
         (z80_inc (gpr8 ?x)))

(rule (lower (sub u8 ?x ?y))
      (z80_sub_a (put_in_a ?x) (gpr8 ?y)))

(rule (lower (const u8 ?v))
      (z80_ld_r_n ?v))

;; Extern constructors
(extern put_in_a)
(extern gpr8)
`
	rs, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}

	if len(rs.Types) != 4 {
		t.Errorf("types: got %d, want 4", len(rs.Types))
	}
	if len(rs.Decls) != 3 {
		t.Errorf("decls: got %d, want 3", len(rs.Decls))
	}
	if len(rs.Rules) != 4 {
		t.Errorf("rules: got %d, want 4", len(rs.Rules))
	}
	if len(rs.Externs) != 2 {
		t.Errorf("externs: got %d, want 2", len(rs.Externs))
	}

	// Check priority ordering: inc rule (priority 10) should be first
	if rs.Rules[0].Priority != 10 {
		t.Errorf("first rule priority: got %d, want 10", rs.Rules[0].Priority)
	}
}

func TestMatch_Variables(t *testing.T) {
	pat := Term{Kind: TermCall, Name: "add", Args: []Term{
		{Kind: TermVar, Name: "x"},
		{Kind: TermVar, Name: "y"},
	}}

	input := Term{Kind: TermCall, Name: "add", Args: []Term{
		{Kind: TermCall, Name: "reg", Args: []Term{{Kind: TermInt, IntVal: 1}}},
		{Kind: TermCall, Name: "reg", Args: []Term{{Kind: TermInt, IntVal: 2}}},
	}}

	bindings := Match(pat, input)
	if bindings == nil {
		t.Fatal("expected match")
	}
	if bindings["x"].String() != "(reg 1)" {
		t.Errorf("?x = %s, want (reg 1)", bindings["x"])
	}
	if bindings["y"].String() != "(reg 2)" {
		t.Errorf("?y = %s, want (reg 2)", bindings["y"])
	}
}

func TestMatch_Nested(t *testing.T) {
	pat := Term{Kind: TermCall, Name: "lower", Args: []Term{
		{Kind: TermCall, Name: "add", Args: []Term{
			{Kind: TermAtom, Name: "u8"},
			{Kind: TermVar, Name: "x"},
			{Kind: TermCall, Name: "const", Args: []Term{{Kind: TermInt, IntVal: 1}}},
		}},
	}}

	input := Term{Kind: TermCall, Name: "lower", Args: []Term{
		{Kind: TermCall, Name: "add", Args: []Term{
			{Kind: TermAtom, Name: "u8"},
			{Kind: TermCall, Name: "reg", Args: []Term{{Kind: TermInt, IntVal: 3}}},
			{Kind: TermCall, Name: "const", Args: []Term{{Kind: TermInt, IntVal: 1}}},
		}},
	}}

	bindings := Match(pat, input)
	if bindings == nil {
		t.Fatal("expected match for add u8 ?x (const 1)")
	}

	inputNo := Term{Kind: TermCall, Name: "lower", Args: []Term{
		{Kind: TermCall, Name: "add", Args: []Term{
			{Kind: TermAtom, Name: "u8"},
			{Kind: TermCall, Name: "reg", Args: []Term{{Kind: TermInt, IntVal: 3}}},
			{Kind: TermCall, Name: "const", Args: []Term{{Kind: TermInt, IntVal: 2}}},
		}},
	}}

	if Match(pat, inputNo) != nil {
		t.Error("should NOT match (const 2)")
	}
}

func TestRewrite_Priority(t *testing.T) {
	src := `
;; General add
(rule (lower (add u8 ?x ?y))
      (z80_add_a ?x ?y))

;; INC peephole (higher priority)
(rule 10 (lower (add u8 ?x (const 1)))
         (z80_inc ?x))
`
	rs, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}

	input := Term{Kind: TermCall, Name: "lower", Args: []Term{
		{Kind: TermCall, Name: "add", Args: []Term{
			{Kind: TermAtom, Name: "u8"},
			{Kind: TermCall, Name: "reg", Args: []Term{{Kind: TermInt, IntVal: 5}}},
			{Kind: TermCall, Name: "const", Args: []Term{{Kind: TermInt, IntVal: 1}}},
		}},
	}}

	result, ok := rs.Rewrite(input)
	if !ok {
		t.Fatal("expected rewrite")
	}

	if result.Name != "z80_inc" {
		t.Errorf("expected z80_inc, got %s", result.Name)
	}
}

func TestRewriteAll_Recursive(t *testing.T) {
	src := `
;; Simplification rules
(rule (add ?x (const 0)) ?x)
(rule (mul ?x (const 1)) ?x)
(rule (neg (neg ?x))     ?x)
`
	rs, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}

	input := Term{Kind: TermCall, Name: "neg", Args: []Term{
		{Kind: TermCall, Name: "neg", Args: []Term{
			{Kind: TermCall, Name: "add", Args: []Term{
				{Kind: TermCall, Name: "reg", Args: []Term{{Kind: TermInt, IntVal: 1}}},
				{Kind: TermCall, Name: "const", Args: []Term{{Kind: TermInt, IntVal: 0}}},
			}},
		}},
	}}

	result := rs.RewriteAll(input)
	if result.String() != "(reg 1)" {
		t.Errorf("expected (reg 1), got %s", result)
	}
}

func TestGuard_IntComparison(t *testing.T) {
	src := `
;; Only fold add to addi when const fits in u8
(rule (add ?x (const ?n)) (if (< ?n 256)) (addi ?x ?n))
;; Fallback: general add
(rule (add ?x ?y) (add_r ?x ?y))
`
	rs, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}

	// const 42 < 256 → should match guarded rule
	input := Term{Kind: TermCall, Name: "add", Args: []Term{
		{Kind: TermCall, Name: "reg", Args: []Term{{Kind: TermInt, IntVal: 1}}},
		{Kind: TermCall, Name: "const", Args: []Term{{Kind: TermInt, IntVal: 42}}},
	}}

	result, ok := rs.Rewrite(input)
	if !ok {
		t.Fatal("expected rewrite")
	}
	if result.Name != "addi" {
		t.Errorf("expected addi, got %s", result.Name)
	}

	// const 300 >= 256 → guard fails, fallback to general add
	input2 := Term{Kind: TermCall, Name: "add", Args: []Term{
		{Kind: TermCall, Name: "reg", Args: []Term{{Kind: TermInt, IntVal: 1}}},
		{Kind: TermCall, Name: "const", Args: []Term{{Kind: TermInt, IntVal: 300}}},
	}}

	result2, ok := rs.Rewrite(input2)
	if !ok {
		t.Fatal("expected fallback rewrite")
	}
	if result2.Name != "add_r" {
		t.Errorf("expected add_r fallback, got %s", result2.Name)
	}
}

func TestExtern_Constructor(t *testing.T) {
	src := `(rule (lower (add ?x ?y)) (my_add (wrap ?x) (wrap ?y)))`
	rs, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}

	externs := NewExternRegistry()
	externs.Register("wrap", func(args []Term, b Bindings) (Term, error) {
		// wrap(x) → (wrapped x)
		return Term{Kind: TermCall, Name: "wrapped", Args: args}, nil
	})

	input := Term{Kind: TermCall, Name: "lower", Args: []Term{
		{Kind: TermCall, Name: "add", Args: []Term{
			{Kind: TermInt, IntVal: 1},
			{Kind: TermInt, IntVal: 2},
		}},
	}}

	result, ok := rs.RewriteWith(input, externs)
	if !ok {
		t.Fatal("expected rewrite")
	}
	if result.String() != "(my_add (wrapped 1) (wrapped 2))" {
		t.Errorf("got %s", result)
	}
}

// helper: parse a single term from source using a wrapper rule
func parseTermFromSrc(t *testing.T, src string) Term {
	t.Helper()
	wrapped := "(rule " + src + " _)"
	rs, err := Parse(wrapped)
	if err != nil {
		t.Fatalf("parse %q: %v", src, err)
	}
	if len(rs.Rules) == 0 {
		t.Fatalf("no rule parsed from %q", src)
	}
	return rs.Rules[0].LHS
}
