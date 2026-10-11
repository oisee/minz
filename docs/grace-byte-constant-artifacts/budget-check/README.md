# Final byte-constant accumulator budget audit

The final integrated compiler after the two guarded accumulator changes compiled the unchanged budget fixture for all four constant-if/invariant-base choices, with the actual first Grace pass followed by CompileHIRSteps' second Grace pass. No old compiler and no VM/Z80 workload rerun. audit.go independently compares saved source, current ASM, and freshly assembled current binary bytes with previous ffdf877d stage docs/grace-word-shift8-artifacts/budget-check. readSaved supports lossless saved gzip source/ASM.

Compilation and audit both exited 0. All four exact source/ASM/BIN comparisons PASS; binary sizes344/344/232/344bytes. Source SHA2563e52eaec62f9e9f9cb5034340f230935495f656ff91672ede5e71848615f83c7. Current elapsed per mask0.635239/120.699337/4.597225/125.881650seconds (251.813451seconds total, excluding audit/build overhead). These exact four unchanged binaries preserve the existing722.080second machine budget gate; no new long machine gate is claimed.

Only OptSizefalse was executed: budget-specific dedup follows CompileHIRSteps' OptSize read into GraceOptions. Explicit empty pointer policy and the rule map admitting only constant-if/invariant-base disable pointer/forward-copy/region rules; neither admitted rule nor backend uses OptSize. This evidence contains four compilations, not eight measured variants. Any changed compared binary would have required a fresh actual machine gate; none changed.

From /tmp/minz-grace-pgo-resume/minzc:

    GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go run /tmp/minz-byte-constant-budget-evidence/compile.go /tmp/minz-byte-constant-budget-evidence/current
    GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go run /tmp/minz-byte-constant-budget-evidence/audit.go /tmp/minz-byte-constant-budget-evidence

Sources, helpers, commands, complete logs, ASM and BIN are final and stable for parent packaging. No parent production edits by this reviewer.
