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

// fuseLoad16LE detects the FatFS ld_word pattern and fuses it:
//
//   const 1; add(base, const_1); load8 → high byte
//   const 8; shl(high, const_8)
//   const 0; add(base, const_0); load8 → low byte (or direct load8(base))
//   or(shifted, low) → result
//
//   → OpLoad16LE(base)  (single VIROp, 4 Z80 instructions)
func fuseLoad16LE(ops []VIROp) []VIROp {
	// Build def map: vreg → defining op index
	defAt := make(map[int]int)
	for i, op := range ops {
		if op.Dst > 0 {
			defAt[op.Dst] = i
		}
	}

	// Build const map
	consts := make(map[int]int64)
	for _, op := range ops {
		if op.Op == OpConst && op.Dst > 0 && op.Sym == "" {
			consts[op.Dst] = op.Imm
		}
	}

	skip := make(map[int]bool)
	var result []VIROp

	for i, op := range ops {
		if skip[i] {
			continue
		}

		// Match: or(shl_result, low_byte) where shl_result = shl(high_byte, 8)
		if op.Op == OpOr && op.Src[0] > 0 && op.Src[1] > 0 {
			shlIdx, shlOk := defAt[op.Src[0]]
			loadLowIdx, loadLowOk := defAt[op.Src[1]]

			// Try both orders (or is commutative)
			if !shlOk || ops[shlIdx].Op != OpShl {
				shlIdx, shlOk = defAt[op.Src[1]]
				loadLowIdx, loadLowOk = defAt[op.Src[0]]
			}

			if shlOk && loadLowOk &&
				ops[shlIdx].Op == OpShl && ops[loadLowIdx].Op == OpLoad {

				shlOp := ops[shlIdx]
				loadLowOp := ops[loadLowIdx]

				// Check shift amount = 8
				if shlOp.Src[1] > 0 {
					if shiftAmt, ok := consts[shlOp.Src[1]]; ok && shiftAmt == 8 {

						// shl source = high byte load
						if loadHighIdx, ok := defAt[shlOp.Src[0]]; ok &&
							ops[loadHighIdx].Op == OpLoad {

							loadHighOp := ops[loadHighIdx]

							// High byte addr = base + 1
							// Low byte addr = base + 0 (or just base)
							// Find the base pointer
							var baseVreg int
							matched := false

							// Check if high addr = add(base, const_1)
							if addHighIdx, ok := defAt[loadHighOp.Src[0]]; ok &&
								ops[addHighIdx].Op == OpAdd {
								addHighOp := ops[addHighIdx]
								// One src should be const 1
								if addHighOp.Src[1] > 0 {
									if v, ok := consts[addHighOp.Src[1]]; ok && v == 1 {
										baseVreg = addHighOp.Src[0]
										matched = true
									}
								}
								if !matched && addHighOp.Src[0] > 0 {
									if v, ok := consts[addHighOp.Src[0]]; ok && v == 1 {
										baseVreg = addHighOp.Src[1]
										matched = true
									}
								}
							}
							// Also try AddImm(base, 1)
							if !matched {
								if addHighIdx, ok := defAt[loadHighOp.Src[0]]; ok &&
									ops[addHighIdx].Op == OpAddImm && ops[addHighIdx].Imm == 1 {
									baseVreg = ops[addHighIdx].Src[0]
									skip[addHighIdx] = true
									matched = true
								}
							}
							// Also try Add where const is already folded away
							if !matched {
								if addHighIdx, ok := defAt[loadHighOp.Src[0]]; ok &&
									ops[addHighIdx].Op == OpAdd {
									addOp := ops[addHighIdx]
									if addOp.Src[1] > 0 {
										if v, ok2 := consts[addOp.Src[1]]; ok2 && v == 1 {
											baseVreg = addOp.Src[0]
											skip[addHighIdx] = true
											matched = true
										}
									}
								}
							}

							if matched && baseVreg > 0 {
								// Verify low byte loads from base (or base+0)
								lowBase := loadLowOp.Src[0]
								validLow := lowBase == baseVreg
								if !validLow {
									// Check if low addr = add(base, 0) or addImm(base, 0)
									if addLowIdx, ok := defAt[lowBase]; ok {
										addLowOp := ops[addLowIdx]
										if addLowOp.Op == OpAdd &&
											(addLowOp.Src[0] == baseVreg || addLowOp.Src[1] == baseVreg) {
											validLow = true
											skip[addLowIdx] = true
										}
										if addLowOp.Op == OpAddImm && addLowOp.Imm == 0 &&
											addLowOp.Src[0] == baseVreg {
											validLow = true
											skip[addLowIdx] = true
										}
										if addLowOp.Op == OpMove && addLowOp.Src[0] == baseVreg {
											validLow = true
											skip[addLowIdx] = true
										}
									}
								}

								if validLow {
									// MATCH! Replace with Load16LE
									// Mark all constituent ops for skip
									skip[shlIdx] = true
									skip[loadLowIdx] = true
									if li, ok := defAt[loadHighOp.Src[0]]; ok { skip[li] = true } // add for high
									skip[defAt[shlOp.Src[0]]] = true // load high
									// Skip const ops (1, 8, 0) if single-use
									for _, cvreg := range []int{shlOp.Src[1]} {
										if ci, ok := defAt[cvreg]; ok && ops[ci].Op == OpConst {
											skip[ci] = true
										}
									}

									result = append(result, VIROp{
										Op:    OpLoad16LE,
										Dst:   op.Dst,
										Src:   [2]int{baseVreg, -1},
										Width: 16,
									})
									continue
								}
							}
						}
					}
				}
			}
		}

		result = append(result, op)
	}

	return result
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
