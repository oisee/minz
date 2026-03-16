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
	t.Logf("rules sorted by priority: %d, %d, %d, %d",
		rs.Rules[0].Priority, rs.Rules[1].Priority,
		rs.Rules[2].Priority, rs.Rules[3].Priority)
}

func TestMatch_Variables(t *testing.T) {
	// Pattern: (add ?x ?y)
	pat := Term{Kind: TermCall, Name: "add", Args: []Term{
		{Kind: TermVar, Name: "x"},
		{Kind: TermVar, Name: "y"},
	}}

	// Input: (add (reg 1) (reg 2))
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
	// Pattern: (lower (add u8 ?x (const 1)))
	pat := Term{Kind: TermCall, Name: "lower", Args: []Term{
		{Kind: TermCall, Name: "add", Args: []Term{
			{Kind: TermAtom, Name: "u8"},
			{Kind: TermVar, Name: "x"},
			{Kind: TermCall, Name: "const", Args: []Term{{Kind: TermInt, IntVal: 1}}},
		}},
	}}

	// Input that matches: (lower (add u8 (reg 3) (const 1)))
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
	t.Logf("?x = %s", bindings["x"])

	// Input that doesn't match: (lower (add u8 (reg 3) (const 2)))
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

	// Input: (lower (add u8 (reg 5) (const 1)))
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
	t.Logf("rewrite result: %s", result)

	// Should match INC rule (priority 10) not general ADD (priority 0)
	if result.Name != "z80_inc" {
		t.Errorf("expected z80_inc, got %s (priority not working?)", result.Name)
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

	// Input: (neg (neg (add (reg 1) (const 0))))
	// Should simplify to: (reg 1)
	input := Term{Kind: TermCall, Name: "neg", Args: []Term{
		{Kind: TermCall, Name: "neg", Args: []Term{
			{Kind: TermCall, Name: "add", Args: []Term{
				{Kind: TermCall, Name: "reg", Args: []Term{{Kind: TermInt, IntVal: 1}}},
				{Kind: TermCall, Name: "const", Args: []Term{{Kind: TermInt, IntVal: 0}}},
			}},
		}},
	}}

	result := rs.RewriteAll(input)
	t.Logf("input:  %s", input)
	t.Logf("result: %s", result)

	// add(reg(1), const(0)) → reg(1), then neg(neg(reg(1))) → reg(1)
	if result.String() != "(reg 1)" {
		t.Errorf("expected (reg 1), got %s", result)
	}
}

func TestZ80_ISelRules(t *testing.T) {
	src := `
;; Z80 instruction selection rules
(rule (lower (const u8 ?v))           (ld_r_n ?v))
(rule (lower (const u16 ?v))          (ld_rr_nn ?v))
(rule (lower (add u8 ?x ?y))         (add_a (put_a ?x) (gpr8 ?y)))
(rule 10 (lower (add u8 ?x (const 1))) (inc (gpr8 ?x)))
(rule (lower (sub u8 ?x ?y))         (sub_a (put_a ?x) (gpr8 ?y)))
(rule (lower (add u16 ?x ?y))        (add_hl (put_hl ?x) (pair ?y)))
(rule (lower (sub u16 ?x ?y))        (sbc_hl (put_hl ?x) (pair ?y)))
(rule (lower (load u8 ?ptr))         (ld_a_hl (put_hl ?ptr)))
(rule (lower (neg u8 ?x))            (z80_neg (put_a ?x)))
`
	rs, err := Parse(src)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		input    string // S-expr
		expected string // expected top-level constructor name
	}{
		{"(lower (const u8 42))", "ld_r_n"},
		{"(lower (add u8 (reg 1) (const 1)))", "inc"},       // priority!
		{"(lower (add u8 (reg 1) (reg 2)))", "add_a"},       // general
		{"(lower (sub u8 (reg 1) (reg 2)))", "sub_a"},
		{"(lower (add u16 (reg 3) (reg 4)))", "add_hl"},
		{"(lower (neg u8 (reg 5)))", "z80_neg"},
	}

	for _, tt := range tests {
		input := parseTermFromSrc(t, tt.input)
		result, ok := rs.Rewrite(input)
		if !ok {
			t.Errorf("%s: no match", tt.input)
			continue
		}
		if result.Name != tt.expected {
			t.Errorf("%s: got %s, want %s", tt.input, result.Name, tt.expected)
		} else {
			t.Logf("%s → %s ✓", tt.input, result)
		}
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
