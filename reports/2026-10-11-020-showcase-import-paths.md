# Showcase: import resolution enabled

The Nanz showcase frontend now uses ParseWithOpts with the source directory
and repository stdlib root, matching module-aware compilation. No skip or
known-failure entries were added. Full compile/assemble showcase after:
14 failures instead of 26, 12 previously failing examples now pass.
Eighteen of nineteen missing-module errors are gone; six expose HIR/ASM
errors, one still cannot resolve stdlib.agon.mos. Raw before/after logs and
per-name comparison are retained in docs/grace-showcase-import-paths.

This improves actual source→ASM→BIN coverage, not a claim that every
newly compiled program was independently executed on Z80. Existing two
result mismatches remain. Next work: repair remaining import spelling/path
and investigate the newly reachable backend failures with native oracles.
