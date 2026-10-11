# Budget fixture equivalence after lexical splitter fix

This bounded check recompiles the unchanged byte-first `grace_rule_budget.nanz` fixture once per four constant-if/invariant-base rule masks. It uses the real two-pass sequence: first Grace application with a nonnil empty pointer policy, then CompileHIRSteps (including automatic splitting). It compares exact ASM and assembled binaries against the prior f39dcf76-stage evidence in `../../grace-pgo-mixed-intern-artifacts/budget-current/`.

The prior stage independently established old/current binary equality and preserved the earlier 722.080-second machine gate. This check does not rerun workload VM or target calls. OptSize is deduplicated by the documented identical code path for disabled pointer/forward-copy rules, as detailed in `../../grace-pgo-mixed-intern-artifacts/budget-size-dedup.md`; no additional eight-compilation claim is made.

Run from minzc: `go run ../docs/grace-pgo-tokenizer-artifacts/budget-check/compile.go ../docs/grace-pgo-tokenizer-artifacts/budget-check/current`, then `go run ../docs/grace-pgo-tokenizer-artifacts/budget-check/audit.go ../docs/grace-pgo-tokenizer-artifacts/budget-check`. `compile.txt` preserves per-mask timing; `audit.txt` records exact source/ASM/binary equality and hashes. The parent artifact manifest binds these files; this check does not rewrite it.

Final result: PASS all four exact ASM and assembled-binary pairs, source SHA256 unchanged. Binary sizes by mask are 344, 344, 232, 344 bytes. The compile-only process completed normally with exit zero. The lexical splitter fix therefore has no effect on any measured budget candidate binary; the preserved 722.080-second workload gate remains applicable to this unchanged fixture.
