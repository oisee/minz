# Next Session Briefing: VIR Production Readiness

## Context

VIR solver achieves -71% vs SDCC on the validated 520-function corpus but **fails on 16/30 real-world examples**. LIR remains the stable production backend. The goal: make VIR handle the remaining 16 failures so it can become the default.

## Priority 1: Fix VIR Assembly Errors (11 examples)

Most failures are invalid `LD` instructions — VIR generates `LD BC, @mir2.str.5` (symbol addresses) and other 16-bit patterns the assembler rejects.

**Root cause categories:**
1. **Symbol address loads** (`LD BC, @symbol`) — VIR doesn't handle 16-bit const loads with symbol references
2. **16-bit operations** — some patterns not in VIR's Z80 descriptor
3. **String literals** — `@mir2.str.N` references not lowered to actual labels

**Failing examples:** tetris_tui, tui_demo, tui_screen, tui_cpm, tui_zx, tui_commander, tui_commander_l3, meta_screen, nc, sap_mara_demo, sqlite_demo

**Where to look:**
- `minzc/pkg/vir/bridge.go` — `translateInst` for OpConst with symbol
- `minzc/pkg/vir/z80.go` — patterns for 16-bit LD with immediate/symbol
- `minzc/pkg/vir/pipeline.go` — emission of symbol references

**Test:** `./minzc examples/nanz/tui_demo.nanz --vir -o /dev/null 2>&1`

## Priority 2: Fix Wrong Results (3 examples)

These compile and assemble but produce incorrect values.

- `11_match_expression.nanz` — `color_code(2)` returns 0 instead of 15
- `12_state_machine.nanz` — `next_state(0, 1)` returns 3 instead of 1
- `assert_test.nanz` — `max_byte(99, 1)` returns 1 instead of 99

**Likely cause:** Per-block solver generates incorrect register assignments for multi-block functions with complex control flow. The CFG solver may not fire (too many variables) and the per-block fallback has known issues.

**Debug approach:** Compile with `--vir` and examine the generated asm for the failing function. Compare with `--lir` output.

## Priority 3: Philipp Reply Update

`research/abi-paper/philipp-reply-2026-03-23.md` has stale numbers (-60%, swap instead of select_b, abs_diff=13). Update to -71%, abs_diff=4, gcd=9, soft edge constraints.

## Priority 4: ABAP Screens (from parallel session)

The other session started exploring ABAP screen declarations (SwiftUI-style PAI/PBO). Context in `~/dev/vibing-steampunk/`. This is a creative/design task, not VIR.

## Priority 5: C89 Internals Booklet

Write a small guide on how C89 source is internally transformed: parsing (modernc.org/cc) → HIR → out-param promotion → struct-return promotion → MIR2 → VIR/LIR.

## Key Files

| File | Purpose |
|------|---------|
| `minzc/pkg/vir/bridge.go` | MIR2 → VIR translation (fix symbol/16-bit) |
| `minzc/pkg/vir/z80.go` | Z80 patterns (add missing 16-bit patterns) |
| `minzc/pkg/vir/pipeline.go` | Emission, peephole, grace (fix symbol refs) |
| `minzc/pkg/vir/solver.go` | Z3 encoding (may need 16-bit constraint fixes) |
| `minzc/pkg/vir/cfgsolver.go` | CFG solver (soft edges already done) |

## Quick Verification

```bash
cd minzc

# Validated corpus (should be 520/520)
go test ./pkg/vir/ -run "NanzCorpus" -v     # 216/216
go test ./pkg/vir/ -run "C89" -v             # 304/304

# Real-world examples (currently 14/30 pass with VIR)
for f in ../examples/nanz/*.nanz; do
  result=$(./minzc "$f" --vir -o /dev/null 2>&1)
  if echo "$result" | grep -qi "error"; then
    echo "FAIL: $(basename $f)"
  else
    echo "PASS: $(basename $f)"
  fi
done

# Benchmark
go test ./pkg/vir/ -run "TestVIR_vs_SDCC" -v  # should show -71%
```

## Session Goal

Get VIR from 14/30 → 25/30+ on real examples. Then reconsider `--vir` as default.
