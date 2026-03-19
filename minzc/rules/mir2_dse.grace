;; Dead Store Elimination — Grace rules for MIR2
;;
;; Removes pure instructions whose result has no uses.
;; The actual DSE logic is instruction-level (not block-pattern),
;; so this rule file serves as the declarative specification.
;; The Go implementation handles the fixpoint iteration.
;;
;; See also: pkg/mir2/dse.go (original Go implementation)

;; Find blocks that contain only pure instructions
;; (used as a helper predicate by the DSE pass)
(grace dse-pure-block 10
  (match
    (block ?b))
  (where
    (all-pure ?b))
  (action))
