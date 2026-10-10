Implemented `VM.ObserveDataAccess func(VMDataAccess)` for executed `OpLoad`/`OpStore` bytes, recording addresses and values in little-endian order. Nil preserves the existing byte loops.

Focused tests passed for loop counts, overlapping u16 accesses, wrapping, exclusions, bounds failures, and baseline equivalence. Only the two scoped files changed; no commits.

Coverage and test details: [report](/tmp/minz-pgo-access-agent.md).