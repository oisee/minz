;; Explicit source-site opt-in; ordered peeled comparison with guarded tail.
(grace hir-guarded-region-compare 5
 (match (block ?b (term-kind "hir.block")))
 (where (hir-has-region-compare ?b))
 (action (custom "hirRegionCompare" ?b)))
