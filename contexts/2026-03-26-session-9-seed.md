# Next Session Seed (after Session 9, final)

**Date:** 2026-03-26 evening
**Release:** v0.23.0 Birthday Marathon Release

---

## What Was Done (Session 9)

### @error Layer 2 — SHIPPED
- `?` in function names = fallible (`fun safe_div?(a: u8, b: u8)`)
- Parser enforces `@check`/`@propagate` after every `?`-call
- Missing it → compile error
- 35/35 Nanz examples still compile

### C Standards Sprint — SHIPPED
- **5 libc headers:** stdbool.h, assert.h, ctype.h (17 inline), stdalign.h, stdnoreturn.h
- **C11:** anonymous structs/unions (field promotion), `_Alignof` → 1, `typeof`
- **C23:** `bool`/`true`/`false` predefined, `__STDC_VERSION__ = 201710L`
- **Array designated init:** `uint8_t arr[5] = {[2] = 42, [4] = 99}`
- **514/514 corpus asserts** (350 c89 + 164 c99+)
- `docs/C_Standards_Roadmap.md` — full C99→C23 feature matrix

### MZA INCBIN — SHIPPED
- `INCBIN "file.bin" [, offset [, length]]`
- Binary embedding for sprites, fonts, GPU tables

### RLCA Sled — SHIPPED
- Multi-entry barrel shifter: 9 bytes, 8 entry points
- `CALL __rotate_4` = nibble swap
- Assembly peephole: 3+ RLCAs → `CALL __rotate_N`
- Sled auto-emitted when referenced

### VIR Zero Failures (from jjjlhyva session)
- 12/14 E2E PASS, 0 FAIL. 45 commits. One-line clamp fix.
- 83.6M exhaustive regalloc table (≤6v complete)

### z80-optimizer v1.0.0 (from um2dy4ex session)
- 372 optimal arithmetic sequences
- 164 constant multiplies, 118/120 constant divisions
- Guided brute-force: abstract chains → focused GPU search
- Prefix-shared mul library: 1.1KB for ALL 254 multiplies

### antique-toy (book team)
- New session eo29c66e joined. Sent `_in/minz_v023_highlights.md` with all findings.

---

## Immediate: Check Results

```bash
ddll explore   # get session IDs

# Division sweep — was 118/120, div11 + div43 still running?
ddll send <z80-optimizer>:main "div11/div43 done? 120/120?"

# VIR P6 (InlineTrivial) — was identified, fix ready?
ddll send <vir>:main "P6 InlineTrivial fix landed?"

# antique-toy — any questions about the highlights file?
ddll send <antique-toy>:main "Questions about minz_v023_highlights.md?"
```

## Priority 0: `--asserts` Flag + MIR2→Z80 Bugs

**`--asserts mir|z80|all` flag for mz CLI.**
Currently all test asserts use `via mir2` because z80 path times out on complex functions.
Need: `--asserts mir` (fast, default), `--asserts z80` (full verify), `--asserts all`.
Default `// assert` without `via` = both mir2+z80 (already wired in pipeline, `Via==""` = both).

**MIR2→Z80 lowering bugs** discovered by VIR team:
- `abs_val(5, 0)` → returns 0 on Z80, correct on mir2 VM
- `gcd(12, 8)` → returns 0 on Z80, correct on mir2 VM
- Both PBQP and VIR backends produce wrong Z80 code → MIR2 codegen issue
- Likely: conditional codegen or while-loop register clobber (ADR-0006/0007)

## Priority 1: C23 `#embed` Directive — DONE ✅

The killer feature for Z80. Binary include at compile time.
```c
const unsigned char font[] = { #embed "font.bin" };
const unsigned char sin_table[] = { #embed "sin256.bin" };
```
Needs: preprocessor-level handling (before cc parser sees it).
Approach: intercept `#embed` in source, replace with `DB` byte sequence.
z80-optimizer has .bin files ready: mulopt8, divopt8, regalloc tables.

## Priority 2: C23 `nullptr` — DONE ✅

Predefined as `((void*)0)` in z80Predefined.

## Priority 3: Peephole — swap_nibbles Pattern

Recognize `(x << 4) | (x >> 4)` at IR level → emit 4× RLCA or CALL __rotate_4.
Currently: emits ~12 instructions (SLA×4 + SRL×4 + OR).
Optimal: 4 bytes inline or 3 bytes + shared sled.

## Priority 4: Known Test Failures (2 TODOs)

- `swap_nibbles(0x12)` — u8 truncation in shifts (MIR2 VM uses u16 width)
- `day_type` switch — multi-case fallthrough desugaring
Both are codegen issues, not frontend.

## Priority 5: Paper A Final Draft

All data ready. GPT-5.4 reviewed. Draft at `research/paper-a-draft.md`.

---

## Backlog

### Language
- [ ] PL/M MOD operator in HIR lowerer
- [ ] Frill match expression desugaring
- [ ] Nanz import paths for .lanz/.lizp modules

### Backend (VIR)
- [ ] P1: cross-block clobber constraints
- [ ] P6: InlineTrivial asm-body label guard
- [ ] C89 u16 promotion fix → Paper A signature count 315→~250

### C Frontend
- [x] `#embed` (C23) — DONE
- [x] `nullptr` (C23) — DONE
- [ ] `--asserts mir|z80|all` CLI flag
- [ ] `constexpr` (C23)
- [ ] Enum underlying type `enum E : uint8_t` (C23)
- [ ] `<stdbit.h>` — bit manipulation functions (C23)

### Research
- [ ] Paper A final draft + GPT review
- [ ] Paper B prototype (DP partition)
- [ ] Division sweep completion (div11, div43)
- [ ] ABI paper response to Philipp Krause

### Optimization
- [ ] swap_nibbles IR pattern → RLCA×4 / CALL __rotate_4
- [ ] TSMC barrel shifter (runtime-patched rotation dispatch)
- [ ] INCBIN integration with GPU .bin tables

### Demo / Book
- [ ] ZX Spectrum real SQLite (main() loop fix)
- [ ] Pascal CASE + Sieve on CP/M (needs VIR P6)
- [ ] antique-toy sync: RLCA sled + INCBIN + DD prefix chapter content

---

## Session IDs (check with `ddll explore`)
- minz: ju6yy047
- minz-vir: jjjlhyva
- z80-optimizer: um2dy4ex
- antique-toy: eo29c66e
- GPT-5.4: `ddll ask gpt54 -s <session>`

## Key Files

| File | Purpose |
|------|---------|
| `docs/C_Standards_Roadmap.md` | C99→C23 feature matrix |
| `docs/Error_Propagation_Design.md` | @error full design |
| `examples/nanz/15_error_enforcement.nanz` | ? enforcement demo |
| `examples/nanz/16_rotate_sled.nanz` | RLCA sled demo |
| `examples/c/` | 12 C99+ test programs, 164 asserts |
| `minzc/pkg/c89/libc/` | 12 libc headers |
| `contexts/2026-03-26-session-wisdom.md` | Sessions 3-8 wisdom |
| `reports/2026-03-26-Birthday-Sprint-Zero-Failures.md` | VIR zero failures |
