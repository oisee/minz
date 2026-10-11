# Final ordered-byte read-write integration budget audit

The final integrated compiler (63b17425 baseline plus the frozen read-write contract/API/CLI changes) compiled the unchanged budget fixture for all four constant-if/invariant-base masks. Each ran the actual first Grace pass then CompileHIRSteps' second Grace pass. OrderedBytePolicy and OrderedByteRWPolicy are absent and the explicit opt-in transformation is inactive in this fixture. No old compiler or VM/Z80 workload rerun.

Both compile and independent fresh-assembler audit exited0. Source, all four exactASM texts and actualassembledBIN bytes match previous 63b17425 docs/grace-ordered-byte-function-artifacts/budget-check. Binary sizes344/344/232/344B, sourceSHA3e52eaec62f9e9f9cb5034340f230935495f656ff91672ede5e71848615f83c7. Currentper-maskelapsed0.502362/122.887055/4.730602/122.365542seconds (250.485561seconds total excluding build/audit overhead). The prior722.080second machine budget gate remains applicable by exactbinaryidentity; no freshlongmachinegate is claimed.

Only OptSizefalse was compiled. Existing budget-specific dedup is justified by CompileHIRSteps' OptSize read into GraceOptions and explicit empty pointer policy/rule map admitting only constant-if/invariant-base. Pointer, forward-copy, region and ordered-byte changes are disabled; neither admittedrule norbackend reads OptSize. This is four executed compilations, not eight measuredvariants. A changedbinary would require a freshactualmachinegate; nonechanged.

From /tmp/minz-grace-pgo-resume/minzc:

    GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go run /tmp/minz-rw-budget-evidence/compile.go /tmp/minz-rw-budget-evidence/current
    GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go run /tmp/minz-rw-budget-evidence/audit.go /tmp/minz-rw-budget-evidence

Helpers use readSaved for lossless gzip source/ASM. Source, helpers, complete logs and currentASM/BIN are final/stable for parent packaging. No parentproductionedits by reviewer.
