Fixed within `minzc/pkg/mir2` only.

- Added explicit `TransferLoopOrigin` for clones and body moves.
- Preserved SiteID, Role, and Rule through split returns, Grace merges, sinking, compare fusion, unrolling, and tail-recursion moves.
- Conflicting attributions keep separate blocks; fusion never picks an arbitrary origin.
- Block removal cleans its metadata. Verification rejects missing labels and incomplete origins. LUT replacement clears obsolete lineage.

[Regression tests](/tmp/minz-grace-pgo-resume/minzc/pkg/mir2/loop_origin_test.go) reproduce failures against original code. The full MIR2 suite passes using your specified Go binary and cache.

Remaining boundaries: the map still represents one origin per block; generic `HoistInsts` and direct label mutations require explicit lineage handling. Bounded-unroll legality and SSA/value remapping were not validated by this attribution fix.

No commit. Unrelated working-tree changes were left untouched.