# Conditional return must not execute the untaken edge's copies

Original IRC `buf_starts_with` after Grace Size has a conditional return of 1 when `prefix[i]` is zero, otherwise passes that byte to the loop body. Allocation puts the return constant in A and the byte in D. The general `TermCondRet` emitter incorrectly copied Then's D→A argument on the return path, after initializing A=1. A matching `PING` consequently returned false; the outer classifier returned Unknown (6), whereas raw/final VM and the independent character model returned Ping (0).

`branchLabel` already places Then argument copies in its branch trampoline. The fix removes their second emission on the untaken return path. Return-value copies, condition dispatch, cleanup, and actual Then copies remain. No language or optimization-policy change.

The independent physical regression exercises all 256 byte inputs, six comparison predicates, two physical block layouts, and both A/E return-value homes: 6,144 assembled native calls plus VM checks. Then values and return constants intentionally share A on the edge. Both taken and untaken paths verify the original return PC/SP and all 64 KiB except the actual private stack scratch. Before: equality/not-equality/simple unsigned cases overwrite the return. After: PASS. Compound <=/> paths and nontrivial return copies also remain covered.

Existing targeted CondRet tests PASS (0.610 s). Full MIR2 PASS (80.208 s). Before original full-module MIR/ASM and reduced diagnostic are retained. The accompanying full IRC computational packet supplies the real-source regression; network/TUI/main execution is outside its boundary.
