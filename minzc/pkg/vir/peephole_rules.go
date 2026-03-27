// peephole_rules.go — GPU-proven peephole optimization rules.
//
// Loads rules from z80-optimizer's peephole_top500.json (or custom path).
// Each rule: source instruction sequence → optimal replacement.
// All rules proven correct by GPU exhaustive search on all 256 input values.
//
// Table-driven, easily expandable: add entries to JSON, reload.
// Format: {"source": "SLA A : RR A", "replacement": "OR A", "cycles_saved": 12}
package vir

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
)

type PeepholeRule struct {
	Source      string `json:"source"`
	Replacement string `json:"replacement"`
	CyclesSaved int    `json:"cycles_saved"`
	BytesSaved  int    `json:"bytes_saved"`
	Category    string `json:"category"`
}

type PeepholeRuleTable struct {
	// Map from normalized source → rule (for O(1) lookup)
	rules2 map[string]*PeepholeRule // 2-instruction patterns
	rules3 map[string]*PeepholeRule // 3-instruction patterns
	total  int
}

var (
	globalPeephole     *PeepholeRuleTable
	globalPeepholeOnce sync.Once
)

func GetPeepholeRules() *PeepholeRuleTable {
	globalPeepholeOnce.Do(func() {
		globalPeephole = loadPeepholeRules()
	})
	return globalPeephole
}

func loadPeepholeRules() *PeepholeRuleTable {
	t := &PeepholeRuleTable{
		rules2: make(map[string]*PeepholeRule),
		rules3: make(map[string]*PeepholeRule),
	}

	paths := []string{
		os.ExpandEnv("$HOME/dev/z80-optimizer/data/peephole_top500.json"),
		os.ExpandEnv("$HOME/dev/z80-optimizer/data/peephole_4T_plus.json"),
	}
	if p := os.Getenv("PEEPHOLE_RULES_PATH"); p != "" {
		paths = []string{p}
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var rules []PeepholeRule
		if err := json.Unmarshal(data, &rules); err != nil {
			fmt.Fprintf(os.Stderr, "[peephole] parse error: %v\n", err)
			continue
		}
		for i := range rules {
			r := &rules[i]
			key := normalizeSource(r.Source)
			parts := strings.Split(r.Source, " : ")
			if len(parts) == 2 {
				t.rules2[key] = r
			} else if len(parts) == 3 {
				t.rules3[key] = r
			}
		}
		t.total = len(rules)
		fmt.Fprintf(os.Stderr, "[peephole] loaded %d rules from %s\n", len(rules), path)
		return t
	}

	return t
}

// normalizeSource normalizes instruction sequence for matching.
// "SLA A : RR A" → "SLA A\nRR A" (newline-separated, trimmed)
func normalizeSource(src string) string {
	parts := strings.Split(src, " : ")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, "\n")
}

// Lookup2 checks if two consecutive instructions match a 2-rule pattern.
func (t *PeepholeRuleTable) Lookup2(inst1, inst2 string) *PeepholeRule {
	if t == nil || len(t.rules2) == 0 {
		return nil
	}
	key := strings.TrimSpace(inst1) + "\n" + strings.TrimSpace(inst2)
	return t.rules2[key]
}

// Lookup3 checks if three consecutive instructions match a 3-rule pattern.
func (t *PeepholeRuleTable) Lookup3(inst1, inst2, inst3 string) *PeepholeRule {
	if t == nil || len(t.rules3) == 0 {
		return nil
	}
	key := strings.TrimSpace(inst1) + "\n" + strings.TrimSpace(inst2) + "\n" + strings.TrimSpace(inst3)
	return t.rules3[key]
}

func (t *PeepholeRuleTable) Size() int {
	if t == nil {
		return 0
	}
	return t.total
}
