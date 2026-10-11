# Independent direct global pair load audit

The new backend's two saved thirty-call payloads pass the independent unchanged
SelfLanz source model: results, all ten globals, full declared resource memory,
and ordered host accesses. The same values/events also match the previous
measured binaries. Source/workload, semantic HIR, origins and function-site maps
remain identical; compiler executable identity changes to `ea9b0791...`, as
required. Old profiles have not been relabeled current.

| Candidate | Prior T / bytes | New T / bytes | Saved T / bytes |
|---|---|---|---|
| Original HIR | 1221494 / 2091 | 1135370 / 2000 | 86124 / 91 |
| Both arena+append | 1147025 / 2197 | 1079636 / 2099 | 67389 / 98 |

New actual steps are 109829 and 104388 respectively. These are whole-caller
measurements, not the earlier 46608-T arithmetic estimate for only two cur
loads. Other eligible loads and candidate allocation interactions affect costs;
no static site estimate is promoted to a whole-program timing proof.

Every saved BIN/ASM/MIR/allocation payload is bound to its embedded evidence.
Compact sidecar/call-frame/allocation hashes match. New actual ED4B/ED5B word
loads have exact rooted global symbol, offset0/width2, matching little-endian
absolute opcode address and read direction; no numeric/resource-pointer
permission widening appears. The focused machine tests and independent native
review cover low-before-high consumption, AF/HL preservation and destination
cache invalidation. This saved-artifact audit adds no new CPU replay and does
not independently re-prove the emulator implementation.

Source review fixed the new DE/BC optimization's lifetime guard to canonical
function-wide address use, including successor/asm/indirect-call uses, and
known global storage width. Legacy HL emission is preserved. Machine fixtures
check direct opcode and complete data traffic, asm pointer retention and
actual low-byte use after the load. No remaining concrete bounded production
legality hole was found in this review.

Reproduce the saved-payload audit:

```sh
PYTHONDONTWRITEBYTECODE=1 python3 docs/grace-global-word-load-review/audit_actual.py "$NEW/actual" "$PRIOR/final" final-audit.json
```

Use saved global-word-load-evidence and memory-host-pc-cost-evidence packs;
lossless gzip payloads are accepted. The owner performs any integration and
broader gates separately. This branch contains review artifacts only.
