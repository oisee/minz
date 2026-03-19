// Package grace implements a graph pattern matching and rewriting engine
// for compiler IRs, providing Cypher-equivalent query power over CFG/DFG.
//
// Grace rules use S-expression syntax with three sections:
//
//	(grace rule-name priority
//	  (match ...)    — graph pattern (blocks, edges, node constraints)
//	  (where ...)    — predicates (is-pure, use-count, pred-count, etc.)
//	  (action ...))  — rewrite actions (delete, replace, hoist, etc.)
//
// The engine operates on abstract IRGraph/IRGraphMut interfaces, so it works
// at any IR level (HIR, MIR2, LIR) via adapters.
package grace

import "github.com/minz/minzc/pkg/rewrite"

// GraceRule is one complete graph rewrite rule.
type GraceRule struct {
	Name     string
	Priority int
	Match    []MatchClause
	Where    []Predicate
	Actions  []Action
}

// MatchClause describes one element of the graph pattern.
type MatchClause struct {
	Kind MatchKind
	Var  string // variable name for binding
	Var2 string // second variable (for edges)

	// For block matches:
	TermKind string // required terminator kind ("" = any)

	// For edge matches:
	EdgeKind string // "then", "else", "succ", "back"

	// For node/instruction matches:
	NodeOp string // opcode filter ("" = any)
}

// MatchKind classifies a match clause.
type MatchKind int

const (
	MatchBlock MatchKind = iota // match a block
	MatchEdge                   // match a CFG edge between two blocks
	MatchNode                   // match an instruction within a block
)

// Predicate is a WHERE clause constraint.
type Predicate struct {
	Kind PredicateKind
	Var  string // block variable
	Var2 string // second variable (optional)

	// For field comparisons:
	Field string
	Op    string
	Value int

	// For custom predicates:
	CustomName string
	CustomArgs []string
}

// PredicateKind classifies a predicate.
type PredicateKind int

const (
	PredNone       PredicateKind = iota
	PredPredCount                // pred-count var op value
	PredFieldCmp                 // var.field op value (inst_count, param_count)
	PredNoParms                  // no-params var
	PredAllPure                  // all-pure var
	PredUseCount                 // use-count var op value
	PredDominates                // dominates var var2
	PredTermKind                 // term-kind var "kind"
	PredCustom                   // custom predicate name
)

// Action is a rewrite step to execute when a rule matches.
type Action struct {
	Kind ActionKind
	Var  string // target block/node variable
	Var2 string // second variable (for hoist, merge)

	// For replace-term:
	NewTermKind string
	NewTargets  []string

	// For custom actions:
	CustomName string
}

// ActionKind classifies a rewrite action.
type ActionKind int

const (
	ActNone        ActionKind = iota
	ActDelete                 // delete instruction or block
	ActDeleteBlock            // remove block from graph
	ActReplace                // replace instruction with new one
	ActHoistInsts             // move all instructions from Var to Var2
	ActReplaceTerm            // replace terminator
	ActRemoveParam            // remove dead block parameter
	ActCustom                 // call registered Go function
)

// RuleResult holds statistics from applying grace rules.
type RuleResult struct {
	Applied int
	ByRule  map[string]int
	Rounds  int
}

// ApplyRules applies a set of grace rules to an IRGraphMut until fixpoint or maxRounds.
// Rules are tried in priority order (highest first). On first match+apply, restart.
func ApplyRules(graph rewrite.IRGraphMut, rules []GraceRule, preds *PredicateRegistry, maxRounds int) *RuleResult {
	result := &RuleResult{ByRule: make(map[string]int)}

	// Sort by priority descending
	sorted := make([]GraceRule, len(rules))
	copy(sorted, rules)
	sortRules(sorted)

	for round := 0; round < maxRounds; round++ {
		result.Rounds = round + 1
		fired := false

		for i := range sorted {
			rule := &sorted[i]
			bindings := findMatch(graph, rule, preds)
			if bindings == nil {
				continue
			}
			if applyActions(graph, rule.Actions, bindings, preds) {
				result.Applied++
				result.ByRule[rule.Name]++
				fired = true
				break
			}
		}

		if !fired {
			break
		}
	}

	return result
}

func sortRules(rules []GraceRule) {
	for i := 1; i < len(rules); i++ {
		for j := i; j > 0 && rules[j].Priority > rules[j-1].Priority; j-- {
			rules[j], rules[j-1] = rules[j-1], rules[j]
		}
	}
}
