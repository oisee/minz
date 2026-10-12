# Automatic single-block island split uses explicit contracts

solveIslandPair discovers second-schedule live-ins, appends explicit KeepAlive live-out uses to a copy of the first schedule, solves it, and pins second-schedule inputs to actual ExitLocations. This handles values live through the split clobber rather than relying on formerly allocated dead values. Input operation slices are unchanged.

codegenFuncIslands now uses this contract for a split within one MIR block and emits the concatenated solutions followed by the original terminator exactly once. Native regression forces the automatic split at a DE/D/E clobber, then verifies (x+3) modulo256 for all256 byte inputs with exact returnPC/SP. Original assert/island family PASS6.861s.

Multi-block island path still needs contract integration with CFG labels/edges and transformed vreg identity. It retains its old behavior; this change does not certify it. Defined first-schedule widths seed KeepAlive; unresolved cross-block widths and solver hinted paths remain work. Production HIR→MIR2→Z80 does not require VIR. Full immutable gates predate this research change and continue separately.
