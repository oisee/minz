;; Hoist exact typed, non-trapping addition of invariant scalar parameters.
;; Predicate proves freshness, unchanged inputs and absence of escaping bindings.
(grace hir-invariant-base 5
  (match (block ?b (term-kind "hir.block")))
  (where (hir-has-invariant-base ?b))
  (action (custom "hirHoistBase" ?b)))
