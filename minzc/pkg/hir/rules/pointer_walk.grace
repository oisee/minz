;; A range-derived runtime guard selects a countdown/pointer loop. The slow
;; arm keeps the original unmasked indexed-address behavior, including traps.
(grace hir-guarded-pointer-walk 4
  (match (block ?b (term-kind "hir.block")))
  (where (hir-has-guarded-pointer-walk ?b))
  (action (custom "hirGuardPointerWalk" ?b)))
