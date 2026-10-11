# Budget equivalence after lexical HIR lowering repair

Compile the unchanged byte-first budget fixture with the real two-Grace-pass sequence for the four constant-if/invariant-base masks, once each. Compare its source, exact ASM and assembled binary to the tokenizer stage's saved `../../grace-pgo-tokenizer-artifacts/budget-check/current/` evidence from 568269a8. The previous stage already established equivalence to the earlier 722.080-second machine gate. This check executes no VM/target workload and does not rerun the old compiler.

OptSize is deduplicated by the established code path: it gates only guarded pointer/forward-copy HIR rules, already excluded by this fixture's explicit empty pointer policy and common-rule-only policy. No additional eight compilation claim is made. See `../../grace-pgo-mixed-intern-artifacts/budget-size-dedup.md` for exact code references. Structured lexical repairs must retain these actual binaries before the earlier workload gate can be reused.

From minzc, run `go run ../docs/grace-hir-scope-artifacts/budget-check/compile.go ../docs/grace-hir-scope-artifacts/budget-check/current`, then `go run ../docs/grace-hir-scope-artifacts/budget-check/audit.go ../docs/grace-hir-scope-artifacts/budget-check`. The auditor accepts losslessly gzipped historical source/ASM. `compile.txt` records timing; `audit.txt` records results. Parent owns the containing manifest and remaining gates.

Final result: PASS all four source/ASM/assembled-binary comparisons. The compile process completed normally (exit zero), and binary sizes remain 344, 344, 232, 344 bytes. The structured lexical repair changes none of this fixture's measured machine candidates, supporting reuse of the unchanged earlier 722.080-second workload gate.
