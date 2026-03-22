// saturate.go — Equality saturation for LIR e-graph.
//
// Adds the missing saturation loop to egraph.go:
//   1. Match rewrite rules against e-graph
//   2. Apply rewrites (add new variants to e-classes)
//   3. Rebuild (canonicalize after merges)
//   4. Repeat until fixpoint or iteration limit
//
// This is the core of the egg algorithm (~200 LOC), specialized for Z80.
// Combined with Z3 extraction, it finds provably optimal instruction sequences.
package lir

// SatRule is a rewrite rule for equality saturation.
// LHS pattern matches against MIROp sequences, RHS is the replacement.
type SatRule struct {
	Name  string
	Match func(ops []MIROp, i int) (matched int, bindings map[string]int) // returns number of ops matched
	Apply func(ops []MIROp, i int, bindings map[string]int) []MIROp       // returns replacement ops
	Gain  int                                                              // T-states saved (positive = better)
}

// Saturate applies rewrite rules to the e-graph until fixpoint.
// Each rule may add new variants to existing e-classes.
// Returns number of new variants added.
func (eg *EGraph) Saturate(rules []SatRule, maxIter int) int {
	totalAdded := 0
	for iter := 0; iter < maxIter; iter++ {
		added := 0
		for ci, cls := range eg.Classes {
			root := eg.find(ci)
			if root != ci {
				continue // skip non-root classes
			}
			// Try each rule against each variant in this class.
			for _, rule := range rules {
				for vi, variant := range cls.Variants {
					_ = vi
					matched, bindings := rule.Match(variant.Ops, 0)
					if matched <= 0 {
						continue
					}
					// Apply rule to produce new variant.
					newOps := rule.Apply(variant.Ops, 0, bindings)
					if newOps == nil {
						continue
					}
					// Compute new cost.
					newCost := variant.Cost - rule.Gain
					if newCost < 0 {
						newCost = 0
					}
					// Check if this variant already exists.
					if eg.hasVariant(root, newOps) {
						continue
					}
					// Add new variant to same class.
					cls.Variants = append(cls.Variants, EVariant{
						Ops:  newOps,
						Cost: newCost,
						Tag:  rule.Name,
					})
					added++
				}
			}
		}
		totalAdded += added
		if added == 0 {
			break // fixpoint
		}
	}
	return totalAdded
}

// hasVariant checks if an e-class already has a variant with identical ops.
func (eg *EGraph) hasVariant(classID int, ops []MIROp) bool {
	cls := eg.Classes[eg.find(classID)]
	for _, v := range cls.Variants {
		if opsEqual(v.Ops, ops) {
			return true
		}
	}
	return false
}

func opsEqual(a, b []MIROp) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].Op != b[i].Op || a[i].Dst != b[i].Dst ||
			a[i].Src != b[i].Src || a[i].Imm != b[i].Imm {
			return false
		}
	}
	return true
}

// ── Z80 Saturation Rules ─────────────────────────────────────────────────

// Z80SatRules contains equality saturation rules for Z80.
// These discover equivalent instruction sequences that may have lower cost
// depending on register assignment context.
var Z80SatRules = []SatRule{
	// LD A, 0 → XOR A (7T → 4T, saves 3T)
	{
		Name: "ld_a_zero_to_xor",
		Match: func(ops []MIROp, i int) (int, map[string]int) {
			if len(ops) > i && ops[i].Op == OpConst && ops[i].Imm == 0 && ops[i].Width == 8 {
				return 1, nil
			}
			return 0, nil
		},
		Apply: func(ops []MIROp, i int, _ map[string]int) []MIROp {
			return []MIROp{{Op: OpXor, Dst: ops[i].Dst, Src: [2]int{ops[i].Dst, ops[i].Dst}, Width: 8}}
		},
		Gain: 3,
	},

	// ADD x, 0 → (identity, remove) saves 4T
	{
		Name: "add_zero_elim",
		Match: func(ops []MIROp, i int) (int, map[string]int) {
			if len(ops) > i && ops[i].Op == OpAdd && ops[i].Imm == 0 && ops[i].Src[1] == -1 {
				return 1, nil
			}
			return 0, nil
		},
		Apply: func(ops []MIROp, i int, _ map[string]int) []MIROp {
			return []MIROp{{Op: OpMove, Dst: ops[i].Dst, Src: [2]int{ops[i].Src[0], -1}, Width: ops[i].Width}}
		},
		Gain: 4,
	},

	// Consecutive identical const loads → reuse (CSE at MIROp level)
	// const X, sym=S; ... ; const Y, sym=S → const X, sym=S; move Y, X
	// This catches the duplicate LD IY, p_name pattern.
	{
		Name: "const_cse",
		Match: func(ops []MIROp, i int) (int, map[string]int) {
			if i == 0 || len(ops) <= i {
				return 0, nil
			}
			if ops[i].Op != OpConst {
				return 0, nil
			}
			// Look backward for identical const.
			for j := i - 1; j >= 0 && j >= i-10; j-- {
				if ops[j].Op == OpConst && ops[j].Imm == ops[i].Imm &&
					ops[j].Sym == ops[i].Sym && ops[j].Width == ops[i].Width {
					return 1, map[string]int{"prev_dst": ops[j].Dst}
				}
			}
			return 0, nil
		},
		Apply: func(ops []MIROp, i int, bindings map[string]int) []MIROp {
			prevDst := bindings["prev_dst"]
			return []MIROp{{Op: OpMove, Dst: ops[i].Dst, Src: [2]int{prevDst, -1},
				Width: ops[i].Width, Sym: ops[i].Sym}}
		},
		Gain: 3, // LD rr,nn = 10T, LD rr,rr' ≈ 7T (or free if coalesced)
	},
}

// OpXor is already defined in machines.go as 8.
