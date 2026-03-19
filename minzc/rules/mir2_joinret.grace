;; Split Join Ret — Grace rules for MIR2
;;
;; Splits trivial join-blocks that only contain a Ret into
;; separate Ret blocks at each predecessor.
;;
;; Pattern:
;;   @join(1 param, 0 insts, ret %param)
;;   + predecessors jump to @join with 1 arg each
;;
;; See also: pkg/mir2/joinret.go (original Go implementation)

(grace split-join-ret 8
  (match
    (block ?join (term-kind "ret")))
  (where
    (param-count ?join == 1)
    (inst-count ?join == 0)
    (custom "isRetOfParam" ?join))
  (action
    (custom "splitJoinRet" ?join)))
