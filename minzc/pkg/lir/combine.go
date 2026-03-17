// combine.go — ISLE-based instruction combining pass.
//
// Runs BEFORE isel to recognize multi-instruction patterns (like shift+or
// for 16-bit LE loads) and fuse them into single combined operations.
//
// Pipeline:
//
//	MIROps → build DAG → ISLE rewrite → extract combined MIROps
//
// The combining rules are written in ISLE S-expression syntax.
// This is a "pre-isel" optimization — it reduces the MIROp count
// before the pattern-table isel runs, giving the backend fewer,
// more target-friendly operations.
package lir

import (
	"fmt"

	"github.com/minz/minzc/pkg/lir/isle"
)

// CombineRules are ISLE rules for instruction combining.
// Each rule matches a tree of MIR operations and rewrites to a combined op.
var CombineRules = `
;; ── Load combining ──────────────────────────────────────────────────────
;;
;; Pattern: or(shl(load8(add(base, 1)), 8), load8(base))
;; Result:  load16_le(base)
;;
;; This is the FatFS ld_word pattern: two 8-bit loads fused into one 16-bit LE load.

(rule 20 (or (shl (load8 (add ?base (const 1))) (const 8))
             (load8 ?base))
         (load16_le ?base))

;; Commuted variant: load8(base) is first arg to or
(rule 20 (or (load8 ?base)
             (shl (load8 (add ?base (const 1))) (const 8)))
         (load16_le ?base))

;; ── Identity / strength reduction ───────────────────────────────────────

(rule (add ?x (const 0))  ?x)
(rule (or  ?x (const 0))  ?x)
(rule (shl ?x (const 0))  ?x)
(rule (and ?x (const 0))  (const 0))
`

// CombineResult holds the output of instruction combining.
type CombineResult struct {
	Ops     []MIROp // combined MIR ops (fewer than input)
	Changed bool    // true if any combining happened
}

// Combine runs ISLE combining rules on a MIROp sequence.
// Returns the (possibly shortened) sequence.
func Combine(ops []MIROp) (*CombineResult, error) {
	if len(ops) == 0 {
		return &CombineResult{Ops: ops}, nil
	}

	rules, err := isle.Parse(CombineRules)
	if err != nil {
		return nil, fmt.Errorf("combine: parse rules: %w", err)
	}

	// Build a DAG: vreg → defining op index
	defMap := make(map[int]int) // vreg → index in ops
	for i, op := range ops {
		if op.Dst >= 0 {
			defMap[op.Dst] = i
		}
	}

	// Convert each op to an ISLE term tree (expanding sources to their defs).
	// Then try to rewrite the tree. If rewriting succeeds, emit the combined op
	// and mark consumed ops as dead.
	dead := make([]bool, len(ops))
	changed := false

	for i := len(ops) - 1; i >= 0; i-- {
		if dead[i] {
			continue
		}

		tree := opToTerm(ops, i, defMap, 6) // deep enough for or→shl→load8→add→const
		rewritten := rules.RewriteAll(tree)

		if rewritten.String() != tree.String() {
			// Collect consumed indices BEFORE replacing the op
			origConsumed := collectConsumed(ops, i, defMap, 6)

			// Combining succeeded — extract the new op
			newOp, extraConsumed, err := termToOp(rewritten, ops, i, defMap)
			if err == nil {
				ops[i] = newOp
				// Mark intermediate ops as dead, but NOT the op whose result
				// is directly referenced by the combined op (e.g. base addr).
				allConsumed := append(origConsumed, extraConsumed...)
				keepAlive := make(map[int]bool)
				// The new op's sources must stay alive
				for s := 0; s < 2; s++ {
					if newOp.Src[s] >= 0 {
						if idx, ok := defMap[newOp.Src[s]]; ok {
							keepAlive[idx] = true
						}
					}
				}
				for _, idx := range allConsumed {
					if idx != i && !keepAlive[idx] {
						dead[idx] = true
					}
				}
				changed = true
			}
		}
	}

	// Collect non-dead ops
	var combined []MIROp
	for i, op := range ops {
		if !dead[i] {
			combined = append(combined, op)
		}
	}

	return &CombineResult{Ops: combined, Changed: changed}, nil
}

// opToTerm converts a MIROp at index i into an ISLE term tree,
// recursively inlining source definitions up to maxDepth.
func opToTerm(ops []MIROp, i int, defMap map[int]int, maxDepth int) isle.Term {
	op := ops[i]
	name := opName(op.Op)

	// Base case: const
	if op.Op == OpConst {
		return isle.Term{Kind: isle.TermCall, Name: "const", Args: []isle.Term{
			{Kind: isle.TermInt, IntVal: op.Imm},
		}}
	}

	// Load8: (load8 addr)
	if op.Op == OpLoad && op.Width <= 8 {
		src := isle.Term{Kind: isle.TermCall, Name: "reg", Args: []isle.Term{
			{Kind: isle.TermInt, IntVal: int64(op.Src[0])},
		}}
		if maxDepth > 0 && op.Src[0] >= 0 {
			if defIdx, ok := defMap[op.Src[0]]; ok {
				src = opToTerm(ops, defIdx, defMap, maxDepth-1)
			}
		}
		return isle.Term{Kind: isle.TermCall, Name: "load8", Args: []isle.Term{src}}
	}

	// General binary op: (name src0 src1)
	var args []isle.Term
	for s := 0; s < 2; s++ {
		if op.Src[s] < 0 {
			continue
		}
		if maxDepth > 0 {
			if defIdx, ok := defMap[op.Src[s]]; ok {
				args = append(args, opToTerm(ops, defIdx, defMap, maxDepth-1))
				continue
			}
		}
		args = append(args, isle.Term{Kind: isle.TermCall, Name: "reg", Args: []isle.Term{
			{Kind: isle.TermInt, IntVal: int64(op.Src[s])},
		}})
	}

	return isle.Term{Kind: isle.TermCall, Name: name, Args: args}
}

// termToOp converts a rewritten ISLE term back to a MIROp.
// Returns the new op and indices of ops consumed by the combining.
func termToOp(t isle.Term, ops []MIROp, origIdx int, defMap map[int]int) (MIROp, []int, error) {
	orig := ops[origIdx]

	switch t.Name {
	case "load16_le":
		// (load16_le ?base) — base is the address source
		baseVreg, consumed := extractBaseVreg(t.Args[0], ops, defMap)
		return MIROp{
			Op:    OpLoad16LE,
			Dst:   orig.Dst,
			Src:   [2]int{baseVreg, -1},
			Width: 16,
		}, consumed, nil

	case "const":
		if len(t.Args) > 0 && t.Args[0].Kind == isle.TermInt {
			return MIROp{
				Op:  OpConst,
				Dst: orig.Dst,
				Src: [2]int{-1, -1},
				Imm: t.Args[0].IntVal,
			}, nil, nil
		}
	}

	return orig, nil, fmt.Errorf("cannot convert term %s to MIROp", t)
}

// extractBaseVreg extracts the virtual register number from a term that
// represents a memory address.
func extractBaseVreg(t isle.Term, ops []MIROp, defMap map[int]int) (int, []int) {
	switch t.Kind {
	case isle.TermCall:
		if t.Name == "reg" && len(t.Args) > 0 && t.Args[0].Kind == isle.TermInt {
			return int(t.Args[0].IntVal), nil
		}
		// If it's a complex expression (e.g. add), find which vreg defined it
		// by looking up in defMap — the base address vreg
		if t.Name == "add" || t.Name == "const" {
			// Walk ops to find matching def
			for vreg, idx := range defMap {
				tree := opToTerm(ops, idx, defMap, 1)
				if tree.String() == t.String() {
					return vreg, []int{idx}
				}
			}
		}
	}
	return -1, nil
}

// collectConsumed finds all op indices that feed into the op at origIdx,
// recursively up to maxDepth.
func collectConsumed(ops []MIROp, origIdx int, defMap map[int]int, maxDepth int) []int {
	if maxDepth <= 0 {
		return nil
	}
	var result []int
	op := ops[origIdx]
	for s := 0; s < 2; s++ {
		if op.Src[s] >= 0 {
			if defIdx, ok := defMap[op.Src[s]]; ok {
				result = append(result, defIdx)
				result = append(result, collectConsumed(ops, defIdx, defMap, maxDepth-1)...)
			}
		}
	}
	return result
}

// opName returns the ISLE term name for a MIR opcode.
func opName(op int) string {
	switch op {
	case OpConst:
		return "const"
	case OpMove:
		return "move"
	case OpAdd:
		return "add"
	case OpSub:
		return "sub"
	case OpMul:
		return "mul"
	case OpAnd:
		return "and"
	case OpOr:
		return "or"
	case OpXor:
		return "xor"
	case OpShl:
		return "shl"
	case OpShr:
		return "shr"
	case OpLoad:
		return "load8"
	case OpLoad16LE:
		return "load16_le"
	case OpCmp:
		return "cmp"
	default:
		return fmt.Sprintf("op%d", op)
	}
}
