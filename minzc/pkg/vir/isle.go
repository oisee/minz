// isle.go — ISLE-based instruction combining on VIR level.
//
// Runs BEFORE Z3 solver to reduce the VIROp count via term rewriting.
// Each rule matches a tree of VIR operations and rewrites to a combined op.
//
// Rules are written in ISLE S-expression syntax (same as pkg/lir/combine.go).
// This is a "pre-solver" optimization — fewer VIROps = simpler Z3 problem.
//
// Pipeline:
//
//	VIROps → build DAG → ISLE rewrite → extract combined VIROps → Z3 solver
package vir

// ISLECombineRules are the rewriting rules for VIR instruction combining.
// These are equivalent to the LIR rules — the opcodes map 1:1.
var ISLECombineRules = `
;; ── Load combining ──────────────────────────────────────────────────────
;; FatFS ld_word: two 8-bit loads → one 16-bit LE load
(rule 20 (or (shl (load8 (add ?base (const 1))) (const 8))
             (load8 ?base))
         (load16_le ?base))
(rule 20 (or (load8 ?base)
             (shl (load8 (add ?base (const 1))) (const 8)))
         (load16_le ?base))

;; ── Multiply strength reduction ──────────────────────────────────────────
;; Z80 has no MUL instruction. Reduce to shifts+adds.
(rule 15 (mul ?x (const 2))   (add ?x ?x))
(rule 15 (mul (const 2) ?x)   (add ?x ?x))
(rule 15 (mul ?x (const 4))   (shl ?x (const 2)))
(rule 15 (mul (const 4) ?x)   (shl ?x (const 2)))
(rule 15 (mul ?x (const 8))   (shl ?x (const 3)))
(rule 15 (mul (const 8) ?x)   (shl ?x (const 3)))

;; Non-power-of-2: decompose to shifts+adds
(rule 14 (mul ?x (const 3))   (add ?x (add ?x ?x)))
(rule 14 (mul (const 3) ?x)   (add ?x (add ?x ?x)))
(rule 14 (mul ?x (const 5))   (add ?x (shl ?x (const 2))))
(rule 14 (mul (const 5) ?x)   (add ?x (shl ?x (const 2))))

;; Trivial
(rule 10 (mul ?x (const 1))   ?x)
(rule 10 (mul (const 1) ?x)   ?x)
(rule 10 (mul ?x (const 0))   (const 0))
(rule 10 (mul (const 0) ?x)   (const 0))

;; ── Identity / strength reduction ───────────────────────────────────────
(rule (add ?x (const 0))  ?x)
(rule (add (const 0) ?x)  ?x)
(rule (or  ?x (const 0))  ?x)
(rule (or  (const 0) ?x)  ?x)
(rule (shl ?x (const 0))  ?x)
(rule (and ?x (const 0))  (const 0))
(rule (and (const 0) ?x)  (const 0))
(rule (sub ?x (const 0))  ?x)
(rule (xor ?x (const 0))  ?x)
(rule (xor (const 0) ?x)  ?x)

;; ── Constant folding ────────────────────────────────────────────────────
;; Already handled in foldConstIntoALU — not duplicated here to avoid
;; breaking load16_le patterns (ISLE rewrites bottom-up).
`

// ISLECombine runs ISLE combining rules on VIROps.
// Currently a no-op placeholder — the constant folding is already in
// bridge.go (foldConstIntoALU). Full ISLE engine integration TODO
// (requires porting pkg/lir/isle/ to work with VIROp types).
//
// The rules above are kept for documentation and future use.
func ISLECombine(ops []VIROp) []VIROp {
	// Phase 1: already done in bridge — const+ALU → immediate ALU
	// Phase 2: identity/zero elimination (simple, no DAG needed)
	ops = eliminateIdentityOps(ops)
	// Phase 3: load16_le fusion (FatFS ld_word pattern)
	ops = fuseLoad16LE(ops)
	// Note: dead code elimination deferred — return moves appended AFTER
	// ISLECombine, so consts used only by return moves would be wrongly removed.
	// The solver's coalescing handles unused vregs.
	return ops
}

// eliminateDeadOps is a conservative dead code pass: only removes
// OpConst ops whose results are never used. Safe because consts have
// no side effects and removing other ops risks breaking cross-block refs.
func eliminateDeadOps(ops []VIROp) []VIROp {
	used := make(map[int]bool)
	for _, op := range ops {
		for _, s := range op.Src {
			if s > 0 { used[s] = true }
		}
	}
	var result []VIROp
	for _, op := range ops {
		if op.Op == OpConst && op.Dst > 0 && !used[op.Dst] && op.Sym == "" {
			continue // dead const
		}
		result = append(result, op)
	}
	return result
}

// fuseLoad16LE detects the FatFS ld_word pattern and fuses it.
// Two-pass: first find all fusions and mark skip indices, then filter.
func fuseLoad16LE(ops []VIROp) []VIROp {
	defAt := make(map[int]int)
	for i, op := range ops {
		if op.Dst > 0 { defAt[op.Dst] = i }
	}
	consts := make(map[int]int64)
	for _, op := range ops {
		if op.Op == OpConst && op.Dst > 0 && op.Sym == "" { consts[op.Dst] = op.Imm }
	}

	// Pass 1: find all fusion points + collect all skip indices
	type fusion struct {
		orIdx    int // index of the OR op to replace
		baseVreg int
		dstVreg  int
	}
	var fusions []fusion
	skip := make(map[int]bool)

	for i, op := range ops {
		if op.Op != OpOr || op.Src[0] <= 0 || op.Src[1] <= 0 {
			continue
		}

		// Try to match or(shl(load(add(base,1)), 8), load(base_or_add0))
		base, matched, skips := matchLoad16LE(ops, i, defAt, consts)
		if matched {
			fusions = append(fusions, fusion{i, base, op.Dst})
			for _, s := range skips {
				skip[s] = true
			}
		}
	}

	if len(fusions) == 0 {
		return ops
	}

	// Pass 2: emit ops, replacing OR with Load16LE, skipping consumed ops
	fusionAt := make(map[int]fusion)
	for _, f := range fusions {
		fusionAt[f.orIdx] = f
	}

	var result []VIROp
	for i, op := range ops {
		if skip[i] {
			continue
		}
		if f, isFusion := fusionAt[i]; isFusion {
			result = append(result, VIROp{
				Op: OpLoad16LE, Dst: f.dstVreg,
				Src: [2]int{f.baseVreg, -1}, Width: 16,
			})
			continue
		}
		result = append(result, op)
	}
	return result
}

// matchLoad16LE checks if the OR at ops[orIdx] is a load16_le pattern.
// Returns (baseVreg, matched, skipIndices).
func matchLoad16LE(ops []VIROp, orIdx int, defAt map[int]int, consts map[int]int64) (int, bool, []int) {
	op := ops[orIdx]
	var skips []int

	// Try both orders of OR (commutative)
	for _, order := range [][2]int{{0, 1}, {1, 0}} {
		shlSrc, loadLowSrc := op.Src[order[0]], op.Src[order[1]]

		shlIdx, shlOk := defAt[shlSrc]
		loadLowIdx, loadLowOk := defAt[loadLowSrc]
		if !shlOk || !loadLowOk { continue }
		if ops[shlIdx].Op != OpShl || ops[loadLowIdx].Op != OpLoad { continue }

		shlOp := ops[shlIdx]
		loadLowOp := ops[loadLowIdx]

		// Shift by 8?
		if shlOp.Src[1] <= 0 { continue }
		shiftAmt, ok := consts[shlOp.Src[1]]
		if !ok || shiftAmt != 8 { continue }

		// SHL source = load(high_addr)
		loadHighIdx, ok := defAt[shlOp.Src[0]]
		if !ok || ops[loadHighIdx].Op != OpLoad { continue }
		loadHighOp := ops[loadHighIdx]

		// High addr = base + 1 (via Add, AddImm, or Add with const)
		baseVreg := 0
		addHighIdx := -1
		if ai, ok := defAt[loadHighOp.Src[0]]; ok {
			addOp := ops[ai]
			if addOp.Op == OpAddImm && addOp.Imm == 1 {
				baseVreg = addOp.Src[0]
				addHighIdx = ai
			} else if addOp.Op == OpAdd {
				if addOp.Src[1] > 0 {
					if v, ok := consts[addOp.Src[1]]; ok && v == 1 {
						baseVreg = addOp.Src[0]
						addHighIdx = ai
					}
				}
				if baseVreg == 0 && addOp.Src[0] > 0 {
					if v, ok := consts[addOp.Src[0]]; ok && v == 1 {
						baseVreg = addOp.Src[1]
						addHighIdx = ai
					}
				}
			}
		}
		if baseVreg == 0 { continue }

		// Low addr = base or base+0
		lowBase := loadLowOp.Src[0]
		addLowIdx := -1
		validLow := lowBase == baseVreg
		if !validLow {
			if ai, ok := defAt[lowBase]; ok {
				a := ops[ai]
				if (a.Op == OpAddImm && a.Imm == 0 && a.Src[0] == baseVreg) ||
					(a.Op == OpAdd && (a.Src[0] == baseVreg || a.Src[1] == baseVreg)) ||
					(a.Op == OpMove && a.Src[0] == baseVreg) {
					validLow = true
					addLowIdx = ai
				}
			}
		}
		if !validLow { continue }

		// MATCH! Collect all indices to skip
		skips = append(skips, orIdx, shlIdx, loadLowIdx, loadHighIdx)
		if addHighIdx >= 0 { skips = append(skips, addHighIdx) }
		if addLowIdx >= 0 { skips = append(skips, addLowIdx) }
		// Const vregs used in the pattern
		if ci, ok := defAt[shlOp.Src[1]]; ok && ops[ci].Op == OpConst { skips = append(skips, ci) }
		// Const 1 for add
		if addHighIdx >= 0 && ops[addHighIdx].Op == OpAdd {
			for _, s := range ops[addHighIdx].Src {
				if s > 0 {
					if _, isConst := consts[s]; isConst {
						if ci, ok := defAt[s]; ok { skips = append(skips, ci) }
					}
				}
			}
		}
		// Const 0 for low add
		if addLowIdx >= 0 {
			for _, s := range ops[addLowIdx].Src {
				if s > 0 && s != baseVreg {
					if _, isConst := consts[s]; isConst {
						if ci, ok := defAt[s]; ok { skips = append(skips, ci) }
					}
				}
			}
		}

		return baseVreg, true, skips
	}

	return 0, false, nil
}

// eliminateIdentityOps removes trivially redundant operations:
// add(x, 0) → x, mul(x, 1) → x, and(x, 0) → 0, etc.
func eliminateIdentityOps(ops []VIROp) []VIROp {
	// Build const map
	consts := make(map[int]int64)
	for _, op := range ops {
		if op.Op == OpConst && op.Dst > 0 && op.Sym == "" {
			consts[op.Dst] = op.Imm
		}
	}

	var result []VIROp
	skip := make(map[int]bool)

	// Mark const 0 vregs for potential sub(0,x)→neg(x) rewrite
	zeroConsts := make(map[int]bool)
	for vreg, imm := range consts {
		if imm == 0 { zeroConsts[vreg] = true }
	}

	// Pre-scan: mark dead consts for sub(0,x)→neg(x) rewrites
	for _, op := range ops {
		if op.Op == OpSub && op.Src[0] > 0 && zeroConsts[op.Src[0]] {
			for j, cop := range ops {
				if cop.Op == OpConst && cop.Dst == op.Src[0] { skip[j] = true }
			}
		}
	}

	for i, op := range ops {
		if skip[i] {
			continue
		}

		switch op.Op {
		case OpAdd, OpOr, OpXor:
			// x + 0 → x, x | 0 → x, x ^ 0 → x
			if op.Src[1] > 0 {
				if v, ok := consts[op.Src[1]]; ok && v == 0 {
					// Replace: dst = src0 (move)
					result = append(result, VIROp{
						Op: OpMove, Dst: op.Dst, Src: [2]int{op.Src[0], -1},
						Width: op.Width,
					})
					continue
				}
			}
			if op.Src[0] > 0 && (op.Op == OpAdd || op.Op == OpOr || op.Op == OpXor) {
				if v, ok := consts[op.Src[0]]; ok && v == 0 {
					result = append(result, VIROp{
						Op: OpMove, Dst: op.Dst, Src: [2]int{op.Src[1], -1},
						Width: op.Width,
					})
					continue
				}
			}

		case OpSub:
			// 0 - x → neg(x)
			if op.Src[0] > 0 && zeroConsts[op.Src[0]] {
				result = append(result, VIROp{
					Op: OpNeg, Dst: op.Dst, Src: [2]int{op.Src[1], -1},
					Width: op.Width,
				})
				continue
			}
			// x - 0 → x
			if op.Src[1] > 0 {
				if v, ok := consts[op.Src[1]]; ok && v == 0 {
					result = append(result, VIROp{
						Op: OpMove, Dst: op.Dst, Src: [2]int{op.Src[0], -1},
						Width: op.Width,
					})
					continue
				}
			}

		case OpAnd:
			// x & 0 → 0
			if op.Src[1] > 0 {
				if v, ok := consts[op.Src[1]]; ok && v == 0 {
					result = append(result, VIROp{
						Op: OpConst, Dst: op.Dst, Imm: 0, Width: op.Width,
					})
					continue
				}
			}

		case OpMul:
			// x * 0 → 0, x * 1 → x, x * 2 → x + x
			if op.Src[1] > 0 {
				if v, ok := consts[op.Src[1]]; ok {
					switch v {
					case 0:
						result = append(result, VIROp{
							Op: OpConst, Dst: op.Dst, Imm: 0, Width: op.Width,
						})
						continue
					case 1:
						result = append(result, VIROp{
							Op: OpMove, Dst: op.Dst, Src: [2]int{op.Src[0], -1},
							Width: op.Width,
						})
						continue
					case 2:
						result = append(result, VIROp{
							Op: OpAdd, Dst: op.Dst, Src: [2]int{op.Src[0], op.Src[0]},
							Width: op.Width,
						})
						continue
					}
				}
			}

		case OpShl:
			// x << 0 → x
			if op.Src[1] > 0 {
				if v, ok := consts[op.Src[1]]; ok && v == 0 {
					result = append(result, VIROp{
						Op: OpMove, Dst: op.Dst, Src: [2]int{op.Src[0], -1},
						Width: op.Width,
					})
					continue
				}
			}
		}

		result = append(result, op)
	}

	// Note: dead const elimination is done in the solver's coalesceVRegs pass,
	// not here — at this point, return moves haven't been appended yet,
	// so consts referenced only by return moves would be wrongly eliminated.
	return result
}
