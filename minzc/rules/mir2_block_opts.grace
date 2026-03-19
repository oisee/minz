;; Block-level optimization patterns for MIR2
;;
;; These Grace rules express CFG-level patterns that go beyond
;; the 5 core passes (DSE, CondRetSink, SplitJoinRet, DeadBlockArg, AbsDiff).
;;
;; Source: lir/rules.go KnownPatterns(), lir/cfgrules.go DefaultBlockRules()

;; ── Empty block elimination ─────────────────────────────────────────────
;; Block with jump, 0 insts, 0 params → redirect predecessors, delete.
;; Cypher: MATCH (e:Block {term:'jump', insts:0, params:0})
;;         SET predecessors.target = e.target; DELETE e

(grace empty-block-elim 40
  (match
    (block ?e (term-kind "jump")))
  (where
    (inst-count ?e == 0)
    (param-count ?e == 0))
  (action
    (custom "redirectAndDelete" ?e)))

;; ── Block merge ─────────────────────────────────────────────────────────
;; pred(jump) → succ, where succ has exactly 1 predecessor and 0 params
;; → merge succ's insts+term into pred.
;; Cypher: MATCH (p)-[:SUCC]->(s) WHERE p.term='jump' AND pred_count(s)=1

(grace block-merge 35
  (match
    (block ?p (term-kind "jump"))
    (block ?s)
    (edge ?p ?s "succ"))
  (where
    (pred-count ?s == 1)
    (param-count ?s == 0))
  (action
    (custom "mergeBlocks" ?p ?s)))

;; ── Dead ret-ret ────────────────────────────────────────────────────────
;; Block ending with ret, followed by unreachable ret block.
;; The second ret block has 0 predecessors → safe to delete.

(grace dead-ret-block 25
  (match
    (block ?dead (term-kind "ret")))
  (where
    (pred-count ?dead == 0)
    (is-not-entry ?dead))
  (action
    (delete-block ?dead)))

;; ── Trivial branch elimination ──────────────────────────────────────────
;; br_if where both targets are the same block → convert to jump
;; Cypher: MATCH (b:Block {term:'br_if'}) WHERE b.then = b.else

(grace trivial-branch 30
  (match
    (block ?b (term-kind "br_if")))
  (where
    (same-targets ?b))
  (action
    (custom "branchToJump" ?b)))
