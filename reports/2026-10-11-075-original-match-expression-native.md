# Original enum match expressions on VM and native Z80

Original examples/nanz/11_match_expression.nanz source/workload SHA was frozen in61c648f3 before native measurement at compilerc1401a5b. Book v7 §16.1 specifies auto-incremented byte enum values: Color and Shape declared variants are0,1,2. §16.3 specifies match-expression arm values and wildcard behavior. Independent host tables reproduce the named source arms: color_code5/10/15, shape_sides0/4/3, is_warm1/0/0.

All three original functions and all declared variants pass raw VM, final VM and actual assembled Z80 under default, GraceSize and GraceSpeed: nine calls per mode,27 native total, plus the matching VM calls. Whole source and all nine source assertions remain intact. No generated extra function/export is added. Invalid enum tags are outside this declared-domain proof and are not assigned expectations from incidental lowering fallback.

Invocation uses actual production parameter/return allocation with hostile initial registers, a real CALL/RET driver preserving flags, expected return PC/SP, stack guards and all64KiB memory outside actual stack and exact emitted spill slots. These pure raw/final functions must produce zero memory observations. Native test PASS0.037s; no miscompile was found. This is complete behavior over the finite declared enum domain of one source module, not all corpus behavior or optimization gain.

Full source, frozen expected calls, raw/final MIR, actualASM/BIN, allocation, native per-entry T-state totals/minSP and SHA/log remain in reports/match-expression-native. Canonical dated share /mnt/safe/minz/grace/2026-10-11/match-expression-native.
