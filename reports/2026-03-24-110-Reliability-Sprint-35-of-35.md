# Report #110: Reliability Sprint — 26/30 → 35/35

**Date:** 2026-03-24
**Session:** Compilation provenance tracing + 7 bugfixes
**Commits:** 6 (13dc1cd1, f1b35ccf, 65b7ecb4, 8c4376ab, a44c2890, 4f4e7dfb)

## Summary

Reliability-first sprint: added full compilation traceability, then used the data
to identify and fix 7 codegen bugs. All 35 nanz examples now compile and assemble.

## Compilation Provenance Tracing

Every function in the ASM output now carries a full audit trail:

```asm
; ── compilation summary ──────────────────────────────────────
; functions: 85  LIR=62  LIR+PBQP-fallback=23
; splits: 10 (HIR-SPLIT high-pressure)
; fallbacks: 23 [fs$fat12$parse_bpb, draw_panel, ...]
; optimization passes fired: 164
; label audit: OK
; ─────────────────────────────────────────────────────────────

; fun max_byte(a: u8 = A, b: u8 = B) -> u8 = A
; [trace] backend=LIR passes=[dse=1,condret-sink=1,cond-rets=1]
```

### Per-function `[trace]` annotation tracks:
- **Backend**: LIR / VIR / PBQP / LIR+PBQP-fallback / VIR+PBQP-fallback
- **Fallback reason**: exact isel/WFC error message
- **HIR-SPLIT origin**: parent function name + register pressure
- **Optimization passes fired**: const-prop, const-fold, ident-simp, call-elim,
  dse, dead-block-arg, branch-equiv, split-join-ret, condret-sink, fuse-abs-diff
- **cond-ret count**: number of conditional return terminators

### Module summary header:
- Function count by backend
- Split count
- Fallback list with function names
- Total optimization passes fired
- Label audit result (OK or list of undefined labels)

### Label audit:
- Post-assembly scan for referenced-but-undefined labels
- Catches `_spill_*`, `_tsmc_*`, `_mir2_str_*`, `_vir_mem*`, CALL/JP targets
- Warnings on stderr + annotated in module summary

## Bugfixes (7)

### 1. condret emission (27/30)
**Root cause:** WFC `ToInsts()` dropped meta cells (condret sentinels had `Pat == nil`).
Conditional returns were never emitted in the flat LIR path.

**Fix:** Emit condret asm directly from `condRetMap` after instruction emission,
bypassing `ToInsts()`. Careful not to include meta cells in the instruction list
(they affect live range computation for call spill logic).

**Impact:** `assert_test.nanz` — `max_byte(10, 20)` returned 10 instead of 20.

### 2. Orphaned TSMC store redirect (label audit clean)
**Root cause:** `scanTSMCSpillPairs` identified r336 as TSMC-eligible, but the codegen
for one use site didn't call `emitTSMCReload`. The TSMC store patched an undefined
`_tsmc_` label while the reload read from a separate `_spill_` label.

**Fix:** Post-emission scan (`fixOrphanedTSMCStores`) finds `_tsmc_` labels referenced
but never defined, redirects store patches to the corresponding `_spill_` label:
`LD (_tsmc_label+1), A` → `LD (_spill_label), A`.

### 3. Local array promotion to globals (28/30)
**Root cause:** `var msg: [u8; 40]` in `do_delete()` was lowered as `Const(0)` — the
VarDeclStmt handler only checked for StructTy, not ArrayTy. The `&msg` expression
created an `AddrOf("msg")` with the raw variable name as a global symbol.

**Fix:** When `ArrayLen > 0`, create a module-level global (`funcName$varName`) with
zero-initialized storage. Track promoted arrays in `localArrays` set so `&name`
returns the address register directly.

**Impact:** `nc.nanz` — undefined symbol `msg`.

### 4. PBQP fallback for multi-block control flow (30/30)
**Root cause:** The flat LIR path flattens all blocks into one instruction stream,
losing `br_if` branches. Instructions from different blocks execute unconditionally.
The multi-block WFC path has cross-block liveness bugs (destructive patterns like
`XOR A` chosen when A is live in later blocks).

**Fix:** `hasNonTrivialBranch()` detects functions with `br_if` or `cond_ret` with
non-trivial successor blocks, returns error → PBQP fallback. Simple `cond_ret` with
bare-ret successors still uses the flat path.

**Impact:** `11_match_expression.nanz`, `12_state_machine.nanz`.

### 5. Extern function stubs (30/30)
**Root cause:** When hybrid LIR+PBQP splicing runs, `@extern` function stubs (empty
`label: / RET`) from LIR output are lost because `splitAsmByFunction` can't match
them (different header format).

**Fix:** `emitExternStubs()` scans for referenced-but-undefined extern labels and
appends stubs after splicing.

### 6. PUSH IXH → register transfer (35/35)
**Root cause:** WFC regalloc generated `PUSH IXH / POP DE` — but `PUSH IXH` doesn't
exist on Z80 (only full `PUSH IX`).

**Fix:** `fixInvalidHalfRegPushPop()` text-level fixup replaces all `PUSH IXH/IXL/IYH/IYL`
+ `POP rr` sequences with direct transfers: `LD D, 0 / LD E, IXH`.

**Impact:** `screen_alv.nanz`, `screen_report.nanz`.

### 7. BDOS tail-call optimization guard
**Root cause:** `CALL 5 / RET → JP 5` broke BDOS syscalls. JP jumps to address 5
and never returns — BDOS expects a return address on the stack.

**Fix:** Skip tail-call optimization when target is a numeric address (BDOS, RST).
Applied to both LIR and PBQP backends.

## VIR Groundwork

- **CLI `--vir` flag**: was broken (`useVIR && !useLIR` = always false). Fixed.
- **Symbol sanitization**: `@mir2.str.0` → `_mir2_str_0` in VIR emission
- **Memory spill backing**: `mem0` → `_vir_mem0` with DW 0 storage
- **String pool**: `EmitStringPool` added to VIR success path
- **TermCondReturn**: new LIR term kind for multi-block condret

## Score

| Metric | Before | After |
|--------|--------|-------|
| Nanz examples | 26/30 | **35/35** |
| Go test packages | 4/4 | **5/5** (added hir) |
| Commits | — | 6 |
| LOC changed | — | ~600 |

## Remaining Work

- **Cross-block WFC liveness**: LIR multi-block functions fall to PBQP because
  per-block WFC picks destructive patterns. Root cause documented in memory.
- **VIR production**: intrinsic emission, multi-condret, default backend switch.
- **5 "runs but no output" ABAP programs**: need CP/M BDOS string print fix.
