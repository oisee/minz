package isle

import (
	"os"
	"testing"
)

// Preserve the retired backend's declarations as a shared-engine fixture.
// Parsing/matching these terms is not proof of effect-safe memory fusion.
func TestLegacyCombineDeclarations(t *testing.T) {
	data, err := os.ReadFile("testdata/legacy-combine.isle")
	if err != nil {
		t.Fatal(err)
	}
	rules, err := Parse(string(data))
	if err != nil {
		t.Fatal(err)
	}
	if len(rules.Rules) != 32 {
		t.Fatalf("rule count %d", len(rules.Rules))
	}
	identity, err := Parse("(rule (add ?x (const 0)) ?x)")
	if err != nil {
		t.Fatal(err)
	}
	term := Term{Kind: TermCall, Name: "add", Args: []Term{{Kind: TermVar, Name: "v"}, {Kind: TermCall, Name: "const", Args: []Term{{Kind: TermInt, IntVal: 0}}}}}
	if got := identity.RewriteAll(term); got.String() != "?v" {
		t.Fatalf("identity %s", got.String())
	}
}
