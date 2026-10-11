# Final integrated unsigned shift-by-eight budget check

Current compiler after the general byte-extraction fix compiled the unchanged budget source for all four constant-if/invariant-base masks, each through the real first Grace run and CompileHIRSteps second Grace run. No old compiler and no workload VM/Z80 rerun. `audit.go` compares source, exact ASM and freshly assembled binary bytes with parent stage88549 `docs/grace-pgo-copy-region-artifacts/budget-check`; saved source/ASM gzip is handled losslessly.

Both commands exited0. All four comparisons PASS; sizes344/344/232/344bytes. SourceSHA2563e52eaec62f9e9f9cb5034340f230935495f656ff91672ede5e71848615f83c7. Current per-maskelapsed0.556s/121.363s/5.624s/118.585s (246.128s total). Exact unchanged four binaries preserve the previous722.080second machine budget gate. No fresh long machine gate is claimed.

Only OptSizefalse compiled: the existing budget-specific deduplication follows the actual CompileHIRSteps OptSize read into HIRGrace; explicit empty pointerpolicy and rule map containing only constant-if/invariant-base exclude pointer/forward-copy/region rewrites. Neither admitted rule nor backend reads OptSize. This is four executed compilations, not eight measured size variants. The generic backend change would have required a fresh machine gate if any compared binary changed; none did.

Commands from parent minzc:

    GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go run /tmp/minz-word-shift8-budget-evidence/compile.go /tmp/minz-word-shift8-budget-evidence/current
    GOCACHE=/tmp/minz-grace-gocache /home/alice/go/pkg/mod/golang.org/toolchain@v0.0.1-go1.26.8.linux-amd64/bin/go run /tmp/minz-word-shift8-budget-evidence/audit.go /tmp/minz-word-shift8-budget-evidence

Sources, commands/helpers, complete logs and actualASM/BIN are stable for parent artifact packaging. No parent production file edits by reviewer.
