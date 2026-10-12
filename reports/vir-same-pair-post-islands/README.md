# Same-pair readmission after island fixes rejected

On4eb2c23a (includes continuation and automatic single-block island handoff), zero-cost same_pair still reduces double_dec to3 bytes with full65536 native word-domain PASS but fails IndexedArrayBaseRegression0 instead16. Adding CFG width domains based on op.Width incorrectly constrains flag results (comparison operand width differs from result) and fails max/min/counted-loop. Restricting that probe to16-bit definition widths leaves only IndexedArrayBaseRegression, now7 instead16. No optimization admitted.

All three modified source files restored. Unchanged indexed scenario then PASS5.473s. Need independently isolate board_set and inter-call live-result preservation before readmission; improvement on the arithmetic fixture is not enough. Full current production gate remains active, no restart.
