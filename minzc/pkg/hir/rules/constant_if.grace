;; Structural HIR block query. The adapter proves that the condition is a
;; literal boolean and that selecting the branch preserves lexical bindings.
;; Calls and stores in the selected arm remain in their original order.
(grace hir-constant-if 10
  (match (block ?b (term-kind "hir.block")))
  (where (hir-has-constant-if ?b))
  (action (custom "hirSelectBranch" ?b)))
