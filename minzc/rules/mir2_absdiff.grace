;; Abs Diff Fusion — Grace rules for MIR2
;;
;; Recognizes the abs_diff pattern and rewrites for optimal Z80 codegen:
;;   cmp.ugt(a,b) + sub(b,a) → sub(a,b) + CmpSubCarry
;;
;; This enables flag fusion: SUB sets carry when a < b, so no
;; separate CP instruction needed.
;;
;; Note: The pattern matching is instruction-level within a block,
;; handled by the Go adapter via custom predicates.
;;
;; See also: pkg/mir2/absdiff.go (original Go implementation)

(grace abs-diff-fusion 12
  (match
    (block ?b))
  (where
    (inst-count ?b >= 2)
    (custom "hasAbsDiffPattern" ?b))
  (action
    (custom "fuseAbsDiff" ?b)))
