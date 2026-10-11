# Four current compile-only masks

Compile unchanged budget fixture once for each constant-if/invariant-base mask with both actualGrace passes. Compare currentsource, exactASM and assembledbinary to `../../grace-hir-scope-artifacts/budget-check/`. No old compiler or target workload rerun. See containingREADME for commands andOptSize deduplication limit.

Terminal result: all four source/exactASM/assembledbinary comparisons PASS against the scope-stage candidates. Sizes remain344/344/232/344 bytes. Current compilations completed in0.368s/122.603s/5.254s/124.840s; no old compiler or workload rerun.
