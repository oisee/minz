;; Dead Block Argument Elimination — Grace rules for MIR2
;;
;; Removes block parameters that are never used anywhere.
;; When a parameter is removed, corresponding arguments are
;; dropped from every incoming edge.
;;
;; Note: The actual implementation requires use-count analysis
;; across the whole function, which is handled by the Go adapter.
;;
;; See also: pkg/mir2/deadblockarg.go (original Go implementation)

(grace dead-block-arg 5
  (match
    (block ?b))
  (where
    (param-count ?b > 0)
    (custom "hasDeadParams" ?b))
  (action
    (custom "elimDeadParams" ?b)))
