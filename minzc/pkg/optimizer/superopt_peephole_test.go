package optimizer

import (
	"os"
	"strings"
	"testing"
)

func TestNormalizeForSuperopt(t *testing.T) {
	tests := []struct {
		name string
		line string
		want string
	}{
		{"8-bit immediate", "    LD A, 255    ; comment", "LD A, 0FFh"},
		{"CP decimal", "    CP 68", "CP 44h"},
		{"no immediate", "    XOR A", "XOR A"},
		{"DJNZ counter", "    LD B, 5       ; DJNZ counter", "LD B, 05h"},
		{"dollar hex port", "    OUT ($23), A", "OUT (23h), A"},
		{"16-bit zero", "    LD HL, 0", "LD HL, 0000h"},
		{"16-bit small", "    LD HL, 1", "LD HL, 0001h"},
		{"16-bit value", "    LD BC, 1024", "LD BC, 0400h"},
		{"label skip", "label:", ""},
		{"comment skip", "; comment", ""},
		{"directive DB", "    DB 65, 66", ""},
		{"directive ORG", "    ORG 32768", ""},
		{"SMC marker", "    LD (param_x_imm0), A", ""},
		{"SMC keep", "    LD A, 0    ; @keep", ""},
		{"empty line", "", ""},
		{"whitespace only", "    ", ""},
		{"ADD no imm", "    ADD A, A", "ADD A, A"},
		{"SLA no imm", "    SLA A", "SLA A"},
		{"OR A no imm", "    OR A", "OR A"},
		{"LD A zero", "    LD A, 0", "LD A, 00h"},
		{"LD A ten", "    LD A, 10", "LD A, 0Ah"},
		{"LD A 67", "    LD A, 67", "LD A, 43h"},
		{"dollar hex AF", "    OUT ($FE), A", "OUT (0FEh), A"},
		{"DEFB directive", "    DEFB 0", ""},
		{"DEFW directive", "    DEFW 1234", ""},
		{"PATCH_TABLE", "    PATCH_TABLE", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeForSuperopt(tt.line)
			if got != tt.want {
				t.Errorf("normalizeForSuperopt(%q) = %q, want %q", tt.line, got, tt.want)
			}
		})
	}
}

func TestReplacementToMinZ(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"XOR A", "XOR A"},
		{"OR A", "OR A"},
		{"LD A, 0FFh", "LD A, 255"},
		{"LD A, 00h", "LD A, 0"},
		{"SUB A", "SUB A"},
		{"AND A", "AND A"},
		{"LD HL, 0001h", "LD HL, 1"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := replacementToMinZ(tt.input)
			if got != tt.want {
				t.Errorf("replacementToMinZ(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestSuperoptRuleLookup(t *testing.T) {
	rules := map[string]SuperoptRule{
		"LD A, 00h : SRL A": {ReplacementAsm: "XOR A", BytesSaved: 3, CyclesSaved: 11},
		"SLA A : RR A":      {ReplacementAsm: "OR A", BytesSaved: 3, CyclesSaved: 12},
		"AND 00h : NEG":     {ReplacementAsm: "SUB A", BytesSaved: 3, CyclesSaved: 11},
	}

	pass := NewSuperoptPeepholePassFromMap(rules)
	if pass.RuleCount() != 3 {
		t.Errorf("expected 3 rules, got %d", pass.RuleCount())
	}

	// Verify lookup
	if r, ok := pass.rules["LD A, 00h : SRL A"]; !ok || r.ReplacementAsm != "XOR A" {
		t.Error("expected rule 'LD A, 00h : SRL A' → 'XOR A'")
	}
	if r, ok := pass.rules["SLA A : RR A"]; !ok || r.ReplacementAsm != "OR A" {
		t.Error("expected rule 'SLA A : RR A' → 'OR A'")
	}
}

func TestSuperoptOnAssembly(t *testing.T) {
	rules := map[string]SuperoptRule{
		"LD A, 00h : SRL A": {ReplacementAsm: "XOR A", BytesSaved: 3, CyclesSaved: 11},
		"SLA A : RR A":      {ReplacementAsm: "OR A", BytesSaved: 3, CyclesSaved: 12},
	}
	pass := NewSuperoptPeepholePassFromMap(rules)

	input := strings.Join([]string{
		"    ; Function start",
		"    LD A, 0",
		"    SRL A",
		"    LD B, A",
		"    SLA A",
		"    RR A",
		"    RET",
	}, "\n")

	result := pass.OptimizeAssembly(input)

	// First pair should be replaced
	if !strings.Contains(result, "XOR A") {
		t.Error("expected 'LD A, 0 : SRL A' to be replaced with 'XOR A'")
	}

	// Second pair should be replaced
	if !strings.Contains(result, "OR A") {
		t.Error("expected 'SLA A : RR A' to be replaced with 'OR A'")
	}

	// Count
	if pass.Count() != 2 {
		t.Errorf("expected 2 rules applied, got %d", pass.Count())
	}

	if pass.BytesSaved() != 6 {
		t.Errorf("expected 6 bytes saved, got %d", pass.BytesSaved())
	}

	// Summary line
	if !strings.Contains(result, "; Superoptimizer: 2 rules applied, 6 bytes saved") {
		t.Error("expected summary comment")
	}
}

func TestSuperoptSkipsSafety(t *testing.T) {
	rules := map[string]SuperoptRule{
		"LD A, 00h : SRL A": {ReplacementAsm: "XOR A", BytesSaved: 3, CyclesSaved: 11},
	}
	pass := NewSuperoptPeepholePassFromMap(rules)

	// Label between instructions should prevent matching
	input := strings.Join([]string{
		"    LD A, 0",
		"loop:",
		"    SRL A",
	}, "\n")
	result := pass.OptimizeAssembly(input)
	if strings.Contains(result, "XOR A") {
		t.Error("should not optimize across labels")
	}

	// SMC marker should prevent matching
	pass2 := NewSuperoptPeepholePassFromMap(rules)
	input2 := strings.Join([]string{
		"    LD A, 0    ; @keep",
		"    SRL A",
	}, "\n")
	result2 := pass2.OptimizeAssembly(input2)
	if strings.Contains(result2, "XOR A") {
		t.Error("should not optimize SMC-marked instructions")
	}

	// Directive should prevent matching
	pass3 := NewSuperoptPeepholePassFromMap(rules)
	input3 := strings.Join([]string{
		"    LD A, 0",
		"    DB 0",
		"    SRL A",
	}, "\n")
	result3 := pass3.OptimizeAssembly(input3)
	if strings.Contains(result3, "XOR A") {
		t.Error("should not optimize across directives")
	}
}

func TestSuperoptLoadGzip(t *testing.T) {
	gzPath := os.ExpandEnv("$HOME/dev/z80-optimizer/rules.json.gz")
	if _, err := os.Stat(gzPath); os.IsNotExist(err) {
		t.Skip("rules.json.gz not found at", gzPath)
	}

	pass, err := NewSuperoptPeepholePass(gzPath)
	if err != nil {
		t.Fatalf("failed to load gzip rules: %v", err)
	}

	if pass.RuleCount() < 100000 {
		t.Errorf("expected 100K+ rules, got %d", pass.RuleCount())
	}

	// Verify a known rule exists
	if _, ok := pass.rules["SLA A : RR A"]; !ok {
		t.Error("expected rule 'SLA A : RR A' to exist")
	}
}

func TestSuperoptLoadJSON(t *testing.T) {
	jsonPath := os.ExpandEnv("$HOME/dev/z80-optimizer/rules.json")
	if _, err := os.Stat(jsonPath); os.IsNotExist(err) {
		t.Skip("rules.json not found at", jsonPath)
	}

	pass, err := NewSuperoptPeepholePass(jsonPath)
	if err != nil {
		t.Fatalf("failed to load JSON rules: %v", err)
	}

	if pass.RuleCount() < 100000 {
		t.Errorf("expected 100K+ rules, got %d", pass.RuleCount())
	}
}

func TestSuperoptPreservesIndent(t *testing.T) {
	rules := map[string]SuperoptRule{
		"SLA A : RR A": {ReplacementAsm: "OR A", BytesSaved: 3, CyclesSaved: 12},
	}
	pass := NewSuperoptPeepholePassFromMap(rules)

	input := "    SLA A\n    RR A\n"
	result := pass.OptimizeAssembly(input)

	lines := strings.Split(result, "\n")
	if !strings.HasPrefix(lines[0], "    OR A") {
		t.Errorf("expected preserved indent, got %q", lines[0])
	}
	if strings.TrimSpace(lines[1]) != "" {
		t.Errorf("expected blanked second line, got %q", lines[1])
	}
}
