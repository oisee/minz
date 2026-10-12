# Live outer value aliased with a mutable inner counter

Original `examples/nanz/tetris_cpm.nanz` `clear_lines` on a board with four full bottom rows and one hole in every other row returns 4 in the independent model and raw MIR VM, but 1 in ordinary optimized MIR VM. Exact source and workload were frozen before execution. `original-before.txt` and semantic-input.json retain the counterexample. The first narrower row case also exposed skipped ordered reads; the four-row case proves changed semantic output independently of trace optimizations.

`PreallocCoalesce` assumed every edge argument dies at that edge. In this source the outer y remains needed after a nested loop initialized with sy=y. The forward-edge union aliased both SSA registers. After sy reaches zero, the outer loop sees zero and exits too soon. Skipping back-edge unions alone cannot prevent this.

The production change constructs the existing liveness/interference graph before remapping and refuses any union whose original member groups interfere. It checks every member against every member, so prior transitive unions cannot hide conflicts behind a representative. Existing class/ABI and back-edge guards remain. No source or language policy changes.

The standalone countdown regression preserves outer values 1,4,19,255 after inner sy reaches zero; unchanged code fails n=1 with result0. A chain regression additionally checks that every returned union partition remains interference-free and preserves arithmetic results. Original full Tetris is the native regression, including four and twenty cleared rows, full board/order/return PC-SP checks. Targeted existing coalesce/contracts PASS0.029s; full MIR2 result is retained in mir2.txt.

This correction may increase copies/code where the prior compiler illegally collapsed simultaneously-live values; that is not a regression against a valid baseline. Full-module before/after BIN sizes are recorded by the independent family packet. No winning speed claim is made against the wrong-code binary.
