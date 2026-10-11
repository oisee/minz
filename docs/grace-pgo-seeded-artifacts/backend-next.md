# Seeded sum next performance investigation

Verified negative measured result: original148452T62B; guardedpointer241596T164B. Original wins both objectives. Do not weaken winner or workload to get a positive claim.

Fast MIR carries zero r11:[acc] across head compare.ne, forcing A preservation on head/load/decrement; general countdown emits sub+backjump, unlike direct range TermDJNZ. Word addition has both pointer and accumulator allocated pointerclass, causing multiple push/pop and exchange. Investigate eliminating non-loop-carried zero const liveness and lowering zero-checked countdown to TermDJNZ with correct mutated scalar edge args, preserving pointer/seed sum and guard/fallback. Generic deadblock/DJNZ edge regressions already exist.

First generate fresh before/final source+ASM/MIR allocation to isolate costs. Full18caller cases exacttraffic/persistentmemory and15360VMboundarycases plus48Z80conformance remain acceptance gate; original u8 sum cost model exhaustive1022domain must rerun if backend changes. Any improvement must retain measured exactbinary and no broken source or per-site attribution. ScreenSMCcallback disabled only in CPUconformancehelper because hostRAM data overwrite wronglyinvokedRSTkeyboard hooks; platformscreen behavior itself not changed.
