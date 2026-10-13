;; Explicit opt-in exact byte copy; preserves sequential overlapping read/store.
(grace hir-guarded-forward-copy 4
 (match (block ?b (term-kind "hir.block")))
 (where (hir-has-forward-copy ?b))
 (action (custom "hirForwardCopy" ?b)))
