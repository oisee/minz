# Final current sum-array cost evidence

One released invocation of frozen executable `b479c62fd53a16dc46f735090625f3d559998fcfcc15481a72470630c9d209c0` completed: `TestProfileGuidedSumVMZ80 -count=1`, PASS 0.80s, exit 0. Base e51846f1, test-only c46a0a4f; timings were asserted unchanged. Prior 147/202-byte evidence remains untouched.

| Length | Result | Original T / B | Pointer and selected T / B |
|---:|---:|---:|---:|
| 0 | 0 | 69 / 195 | 212 / 250 |
| 1 | 1 | 210 / 195 | 247 / 250 |
| 5 | 15 | 774 / 195 | 395 / 250 |
| 16 | 136 | 2325 / 195 | 802 / 250 |
| 64 | 32 | 9093 / 195 | 2578 / 250 |
| 255 | 128 | 36024 / 195 | 9645 / 250 |

The six-call workload totals are 48495 original and 13879 pointer/selected T-states. Actual VM and Z80 results match independent n(n+1)/2 modulo256. Current evidence identity is `sum-array-z80-all-six-lengths-decimal-print-v3`; the compiler context binds the retained executable. Whole-module bytes increased by 48 solely because main's print.u8 now emits proper decimal output; both sum_array ASM suffixes match the corresponding prior dumps exactly. Timing harness calls sum_array, not main.

The same focused invocation also executed eight boundary checks (n0..3 under speed and size). Speed retains original at 0/1 and pointer at 2/3; Size retains original throughout. Short two-call speed and size decisions retain original; full-workload Size retains original. These boundary executions pass assertions in the retained test but do not have separate raw measurement artifacts. Only the eighteen six-length rows are claimed as individually saved measurements.

All program/bootstrapped ASM and BIN, source HIR, raw/optimized MIR, workload/profile/decision snapshots, terminal log and exact executable are retained. The manifest lists every stored file; all are force-added to Git. No other test or CPU run was performed in this refresh. Historical 1022-row model audits are not relabeled as current execution evidence.
