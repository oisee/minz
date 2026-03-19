;; CondRetSink — Grace rules for MIR2
;;
;; Transforms a BrIf whose else-branch is a trivial return block
;; into a TermCondRet, enabling conditional-return instructions.
;;
;; Pattern:
;;   @cur: br_if %cond, @then, @else
;;   @else: [pure insts...]; ret   (1 pred, no params)
;;
;; → Hoist @else insts into @cur, replace br_if with cond_ret.
;;
;; See also: pkg/mir2/condret.go (original Go implementation)

(grace cond-ret-sink 15
  (match
    (block ?cur (term-kind "br_if"))
    (block ?else (term-kind "ret"))
    (edge ?cur ?else "else"))
  (where
    (no-params ?else)
    (all-pure ?else)
    (pred-count ?else == 1))
  (action
    (hoist-insts ?else ?cur)
    (custom "condRetReplaceTerm" ?cur ?else)))
