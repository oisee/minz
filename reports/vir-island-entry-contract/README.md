# Explicit input contracts between two solved VIR schedules

islandLiveIns discovers use-before-definition inputs (including asm ins/outs). pinIslandEntry prepends scheduling KeepAlive uses pinned to the actual incoming locations. Missing, invalid and nonallocatable locations are rejected. This establishes locations at schedule entry, permitting later movement rather than asserting a global register assignment.

Native256-byte test verifies D input→A result. A second native256-byte test solves two schedules independently: x+1 then +2, passes first ExitLocations through pinIslandEntry, assembles concatenated PIR and checks result (x+3) modulo256 and exact returnPC/SP. Live-out KeepAlive preserves intermediate; final scheduling use pins A. Complete original assert/island family PASS6.892s.

The initial second schedule with DstHint entered broken per-instruction SMT (assert inside cost); log retained. Current final output placement is an explicit terminal source hint. This helper is NOT yet wired into automatic codegenFuncIslands: boundary live-out selection, transformed identities, and schedule metadata/alignment remain necessary. No arbitrary CFG or complete island correctness claim.
