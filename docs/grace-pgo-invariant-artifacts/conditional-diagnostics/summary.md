Fixed scratch-byte exhaustion at two proven MIR2 Z80 accumulator-save sites: CMP preservation and byte ALU preservation. Their old D fallback overwrote a live DE word, explaining both 514 vs 2 and 4721 vs 4465 exactly.

Changed only z80codegen_cmp.go, z80codegen_alu.go, and new conditional_invariant_regression_test.go. Retained a830946c. No HIR/profile-site edits, commits, subagents, full suite, or budget runs.

Both recovered original fixtures pass all four hoist masks, n=0/1/5/255, flags=0/1, three arithmetic pairs, and MIR Grace off/on. Real four-candidate API also passes the unchanged complete workload. Focused pipeline and MIR2 regressions pass. Raw/optimized MIR identical before/after in all 16 compiled variants.

Full report: /tmp/minz-conditional-site-fix-details.md
Preserved machine/MIR/source/log evidence: /tmp/minz-conditional-site-fix
