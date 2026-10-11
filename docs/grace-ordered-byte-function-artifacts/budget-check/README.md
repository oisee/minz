# Final ordered-byte-function integration budget audit

The final integrated compiler (a0e8 baseline plus the frozen ordered-byte contract/API/CLI changes) compiled the unchanged budget fixture for all four constant-if/invariant-base masks. Each ran the actual first Grace pass then CompileHIRSteps' second Grace pass. OrderedBytePolicy is absent and the explicit opt-in transformation is inactive in this fixture. No old compiler or VM/Z80 workload rerun.

Both compile and independent fresh-assembler audit exited0. Source, all four exactASM texts and actualassembledBIN bytes match previous a0e8 docs/grace-byte-constant-artifacts/budget-check. Binary sizes344/344/232/344B, sourceSHA3e52eaec62f9e9f9cb5034340f230935495f656ff91672ede5e71848615f83c7. Currentper-maskelapsed0.402095/123.779231/5.886877/120.680218seconds (250.748421seconds total excluding build/audit overhead). The prior722.080second machine budget gate remains applicable by exactbinaryidentity; no freshlongmachinegate is claimed.

Only OptSizefalse was compiled. Existing budget-specific dedup is justified by CompileHIRSteps' OptSize read into GraceOptions and explicit empty pointer policy/rule map admitting only constant-if/invariant-base. Pointer, forward-copy, region and ordered-byte changes are disabled; neither admittedrule norbackend reads OptSize. This is four executed compilations, not eight measuredvariants. A changedbinary would require a freshactualmachinegate; nonechanged.

From /tmp/minz-grace-pgo-resume/minzc:

    GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go run /tmp/minz-ordered-byte-budget-evidence/compile.go /tmp/minz-ordered-byte-budget-evidence/current
    GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go run /tmp/minz-ordered-byte-budget-evidence/audit.go /tmp/minz-ordered-byte-budget-evidence

Helpers use readSaved for lossless gzip source/ASM. Source, helpers, complete logs and currentASM/BIN are final/stable for parent packaging. No parentproductionedits by reviewer.
