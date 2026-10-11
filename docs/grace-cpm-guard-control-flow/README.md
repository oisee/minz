# Guard execution along control flow

A lexical predecessor `CALL __cpm_stack_check` does not prove that every incoming branch executes the guard. The checker now tracks an execution credit in each control-flow state. Only returning from the trusted checker creates credit; the next instruction consumes it, and branch/call entry starts without credit. Every guarded PUSH/CALL must possess credit on its actual incoming path.

Negative fixtures jump directly to PUSH and conditional CALL, despite a lexical guard immediately above the target. Both unconditional/conditional JP/JR bypasses reject. A correctly guarded conditional CALL remains accepted. Existing original parser/tokenizer, spill-reentrancy and overflow proofs remain PASS with identical saved COM bytes and native metrics.

`focused.txt` records all production runtime/recursive tests. `control-flow.txt` records the new checker admission cases. This strengthens admission; generated programs keep their existing guard placement and machine code.
